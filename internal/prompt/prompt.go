// Package prompt loads and validates dndig prompt files: Markdown with a
// frontmatter block that configures one generation.
package prompt

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickume/dndig/internal/frontmatter"
)

// Kind says what the prompt depicts. It changes the wording of the prompt
// preamble and which continuity rules apply; it never changes the API call.
type Kind string

const (
	KindCharacter Kind = "character"
	KindScene     Kind = "scene"
	KindLocation  Kind = "location"
	KindItem      Kind = "item"
	KindOther     Kind = "other"
)

// Reference is an ad-hoc reference image listed under `references:`.
type Reference struct {
	// Path is absolute once the prompt is loaded.
	Path string
	// As is the caption used in the prompt preamble ("Image 3 is <As>");
	// empty means the file's base name.
	As string
	// Title is set when the reference named another prompt's title rather
	// than a file; the workspace resolves it to that prompt's pick.
	Title string
}

// Prompt is one loaded prompt file.
type Prompt struct {
	Path string // absolute path of the .md file
	Dir  string // its directory
	Stem string // file name without extension

	Title       string
	Kind        Kind
	Style       string // style name or path, empty for none
	AspectRatio string
	Resolution  string
	Model       string // empty means the project/default model
	Takes       int
	Temperature *float64
	Seed        *int64
	Cast        []string
	References  []Reference
	Search      bool
	Body        string
}

// AspectRatios lists every ratio the Gemini image models accept.
var AspectRatios = []string{
	"1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9",
	"1:4", "4:1", "1:8", "8:1",
}

// Resolutions lists the image sizes in the wire spelling.
var Resolutions = []string{"512", "1K", "2K", "4K"}

// Defaults applied when a field is absent.
const (
	DefaultAspectRatio = "1:1"
	DefaultResolution  = "1K"
	DefaultTakes       = 1
	MaxTakes           = 8
	// MaxCast is the documented number of people whose likeness the Pro
	// model preserves in one request.
	MaxCast = 5
)

var titleRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Workspace is the directory that holds the prompt's takes and pick:
// `<dir>/<stem>/`.
func (p *Prompt) Workspace() string { return filepath.Join(p.Dir, p.Stem) }

// Load reads and parses a prompt file. Relative reference paths are
// resolved against the file's directory.
func Load(path string) (*Prompt, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	return Parse(abs, string(data))
}

// Parse parses text as the prompt file at path (used for the directory,
// stem and error messages).
func Parse(path, text string) (*Prompt, error) {
	doc, err := frontmatter.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if !doc.HasFrontmatter {
		return nil, fmt.Errorf("%s: not a prompt file (no frontmatter block)", path)
	}
	p := &Prompt{
		Path:        path,
		Dir:         filepath.Dir(path),
		Stem:        strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Kind:        KindOther,
		AspectRatio: DefaultAspectRatio,
		Resolution:  DefaultResolution,
		Takes:       DefaultTakes,
		Body:        doc.Body,
	}
	p.Title = p.Stem
	var errs []error
	fail := func(f frontmatter.Field, format string, args ...any) {
		errs = append(errs, fmt.Errorf("%s:%d: %s: %s", path, f.Line, f.Key, fmt.Sprintf(format, args...)))
	}
	for _, f := range doc.Fields {
		switch f.Key {
		case "title":
			p.Title = f.Scalar
		case "kind":
			p.Kind = Kind(strings.ToLower(f.Scalar))
			switch p.Kind {
			case KindCharacter, KindScene, KindLocation, KindItem, KindOther:
			default:
				fail(f, "unknown kind %q (character, scene, location, item, other)", f.Scalar)
			}
		case "style", "instructions":
			p.Style = f.Scalar
		case "aspect_ratio":
			p.AspectRatio = f.Scalar
		case "resolution":
			p.Resolution = strings.ToUpper(strings.TrimSpace(f.Scalar))
		case "model":
			p.Model = f.Scalar
		case "takes", "batch":
			n, err := strconv.Atoi(f.Scalar)
			if err != nil || n < 1 || n > MaxTakes {
				fail(f, "must be a whole number from 1 to %d, got %q", MaxTakes, f.Scalar)
			} else {
				p.Takes = n
			}
		case "temperature":
			v, err := strconv.ParseFloat(f.Scalar, 64)
			if err != nil || v < 0 || v > 2 {
				fail(f, "must be a number from 0 to 2, got %q", f.Scalar)
			} else {
				p.Temperature = &v
			}
		case "seed":
			v, err := strconv.ParseInt(f.Scalar, 10, 32)
			if err != nil {
				fail(f, "must be a 32-bit whole number, got %q", f.Scalar)
			} else {
				p.Seed = &v
			}
		case "search":
			switch strings.ToLower(f.Scalar) {
			case "true", "yes", "on":
				p.Search = true
			case "false", "no", "off", "":
			default:
				fail(f, "must be true or false, got %q", f.Scalar)
			}
		case "cast":
			for _, it := range items(f) {
				if it.Map != nil {
					fail(f, "cast entries are names, not maps")
					continue
				}
				if !titleRe.MatchString(it.Scalar) {
					fail(f, "%q is not a valid title (letters, digits, _ and -)", it.Scalar)
					continue
				}
				p.Cast = append(p.Cast, it.Scalar)
			}
		case "references":
			for _, it := range items(f) {
				r := Reference{}
				if it.Map != nil {
					r.Path, r.As = it.Map["path"], it.Map["as"]
					if r.Path == "" {
						fail(f, "a reference map needs a path: {path: x.png, as: \"...\"}")
						continue
					}
				} else {
					r.Path = it.Scalar
				}
				if !strings.ContainsAny(r.Path, `/\.`) && titleRe.MatchString(r.Path) {
					// A bare name is another prompt's title, resolved later.
					r.Title, r.Path = r.Path, ""
				} else if !filepath.IsAbs(r.Path) {
					r.Path = filepath.Join(p.Dir, r.Path)
				}
				p.References = append(p.References, r)
			}
		default:
			fail(f, "unknown field")
		}
	}
	if !titleRe.MatchString(p.Title) {
		errs = append(errs, fmt.Errorf("%s: title %q must be letters, digits, _ and - only", path, p.Title))
	}
	if !contains(AspectRatios, p.AspectRatio) {
		errs = append(errs, fmt.Errorf("%s: aspect_ratio %q is not one of %s", path, p.AspectRatio, strings.Join(AspectRatios, ", ")))
	}
	if !contains(Resolutions, p.Resolution) {
		errs = append(errs, fmt.Errorf("%s: resolution %q is not one of %s", path, p.Resolution, strings.Join(Resolutions, ", ")))
	}
	if len(p.Cast) > MaxCast {
		errs = append(errs, fmt.Errorf("%s: cast lists %d characters; the model preserves at most %d", path, len(p.Cast), MaxCast))
	}
	if seen := dupes(p.Cast); seen != "" {
		errs = append(errs, fmt.Errorf("%s: cast names %q twice", path, seen))
	}
	if strings.TrimSpace(p.Body) == "" {
		errs = append(errs, fmt.Errorf("%s: the prompt body is empty", path))
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return p, nil
}

func items(f frontmatter.Field) []frontmatter.Item {
	if f.Kind == frontmatter.KindList {
		return f.List
	}
	if f.Scalar == "" {
		return nil
	}
	return []frontmatter.Item{{Scalar: f.Scalar}}
}

func contains(set []string, v string) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}

func dupes(names []string) string {
	seen := map[string]bool{}
	for _, n := range names {
		k := strings.ToLower(n)
		if seen[k] {
			return n
		}
		seen[k] = true
	}
	return ""
}

// DependsOn lists the titles this prompt needs picks for: its cast and
// every reference that names a title rather than a file.
func (p *Prompt) DependsOn() []string {
	var out []string
	out = append(out, p.Cast...)
	for _, r := range p.References {
		if r.Title != "" {
			out = append(out, r.Title)
		}
	}
	return out
}
