// Package project finds the campaign root, reads dndig.yaml, indexes the
// prompt files under the root and orders them by dependency.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mickume/dndig/internal/frontmatter"
	"github.com/mickume/dndig/internal/prompt"
	"github.com/mickume/dndig/internal/style"
)

// ConfigFile is the marker and configuration file at the project root.
const ConfigFile = "dndig.yaml"

// StylesDir is where `style: <name>` is looked up, under the root.
const StylesDir = "styles"

// Config is the content of dndig.yaml. Every field is optional.
type Config struct {
	// Model is the image model used when a prompt names none.
	Model string
	// VisionModel is used by `style derive`.
	VisionModel string
	// Style is applied to prompts that name none.
	Style string
	// Workers bounds concurrent requests.
	Workers int
}

// Project is a campaign directory.
type Project struct {
	Root   string
	Config Config
	// HasConfig reports whether dndig.yaml was found; without it the root
	// is the directory the command was pointed at.
	HasConfig bool

	byTitle map[string]*prompt.Prompt
	// Skipped lists Markdown files under the root that are not prompt files
	// (style files, notes) with the reason, for `status`.
	Skipped []Skipped
}

// Skipped is a Markdown file the index did not treat as a prompt.
type Skipped struct {
	Path   string
	Reason string
}

// Find locates the project for a path: the nearest ancestor directory that
// holds dndig.yaml, else the path's own directory. The path may be a prompt
// file or a directory.
func Find(path string) (*Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	dir := abs
	if st, err := os.Stat(abs); err != nil {
		return nil, err
	} else if !st.IsDir() {
		dir = filepath.Dir(abs)
	}
	for d := dir; ; d = filepath.Dir(d) {
		cfgPath := filepath.Join(d, ConfigFile)
		if _, err := os.Stat(cfgPath); err == nil {
			cfg, err := LoadConfig(cfgPath)
			if err != nil {
				return nil, err
			}
			return &Project{Root: d, Config: cfg, HasConfig: true}, nil
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	return &Project{Root: dir}, nil
}

// LoadConfig parses dndig.yaml.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	fields, err := frontmatter.ParseFields(strings.Split(string(data), "\n"), 1)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	var c Config
	for _, f := range fields {
		switch f.Key {
		case "model":
			c.Model = f.Scalar
		case "vision_model":
			c.VisionModel = f.Scalar
		case "style":
			c.Style = f.Scalar
		case "workers":
			n, err := strconv.Atoi(f.Scalar)
			if err != nil || n < 1 || n > 16 {
				return Config{}, fmt.Errorf("%s:%d: workers must be 1..16, got %q", path, f.Line, f.Scalar)
			}
			c.Workers = n
		default:
			return Config{}, fmt.Errorf("%s:%d: unknown field %q", path, f.Line, f.Key)
		}
	}
	return c, nil
}

// Rel renders a path relative to the root for display and sidecars.
func (p *Project) Rel(path string) string {
	if rel, err := filepath.Rel(p.Root, path); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return path
}

// Index scans the root for prompt files. Directories named takes,
// discards, styles and dot-directories are not scanned. A Markdown file
// that does not parse as a prompt is skipped and recorded, not fatal:
// campaign notes live next to prompts.
func (p *Project) Index() error {
	p.byTitle = map[string]*prompt.Prompt{}
	p.Skipped = nil
	var dup []error
	return filepath.WalkDir(p.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != p.Root && (strings.HasPrefix(name, ".") || name == "takes" || name == "discards" || name == StylesDir) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		pr, err := prompt.Load(path)
		if err != nil {
			p.Skipped = append(p.Skipped, Skipped{Path: path, Reason: err.Error()})
			return nil
		}
		key := strings.ToLower(pr.Title)
		if other, ok := p.byTitle[key]; ok {
			dup = append(dup, fmt.Errorf("title %q is used by both %s and %s", pr.Title, p.Rel(other.Path), p.Rel(path)))
			return errors.Join(dup...)
		}
		p.byTitle[key] = pr
		return nil
	})
}

// Lookup finds an indexed prompt by title, case-insensitively.
func (p *Project) Lookup(title string) (*prompt.Prompt, bool) {
	pr, ok := p.byTitle[strings.ToLower(title)]
	return pr, ok
}

// Prompts returns every indexed prompt, sorted by path.
func (p *Project) Prompts() []*prompt.Prompt {
	out := make([]*prompt.Prompt, 0, len(p.byTitle))
	for _, pr := range p.byTitle {
		out = append(out, pr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// ResolveStyle finds the style a prompt names. A bare name is
// `<root>/styles/<name>.md`; anything with a path separator or extension
// is a path, relative to the prompt's directory first and the root second.
// An empty spec falls back to the project default; no style at all returns
// (nil, nil).
func (p *Project) ResolveStyle(spec string, from *prompt.Prompt) (*style.Style, error) {
	if spec == "" {
		spec = p.Config.Style
	}
	if spec == "" {
		return nil, nil
	}
	var candidates []string
	if strings.ContainsAny(spec, `/\`) || filepath.Ext(spec) != "" {
		if filepath.IsAbs(spec) {
			candidates = []string{spec}
		} else {
			if from != nil {
				candidates = append(candidates, filepath.Join(from.Dir, spec))
			}
			candidates = append(candidates, filepath.Join(p.Root, spec))
		}
	} else {
		candidates = []string{filepath.Join(p.Root, StylesDir, spec+".md")}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return style.Load(c)
		}
	}
	return nil, fmt.Errorf("style %q not found (looked at %s)", spec, strings.Join(candidates, ", "))
}

// Order sorts prompts so that every prompt comes after the prompts it
// depends on (its cast, and references that name a title). Prompts with no
// dependencies keep their given order; a dependency outside the list is
// left to the pick check at generation time.
func (p *Project) Order(prompts []*prompt.Prompt) ([]*prompt.Prompt, error) {
	index := map[string]int{}
	for i, pr := range prompts {
		index[strings.ToLower(pr.Title)] = i
	}
	deps := make([][]int, len(prompts))
	indeg := make([]int, len(prompts))
	for i, pr := range prompts {
		for _, t := range pr.DependsOn() {
			j, ok := index[strings.ToLower(t)]
			if !ok || j == i {
				continue
			}
			deps[j] = append(deps[j], i)
			indeg[i]++
		}
	}
	var ready []int
	for i := range prompts {
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	var out []*prompt.Prompt
	for len(ready) > 0 {
		sort.Ints(ready)
		n := ready[0]
		ready = ready[1:]
		out = append(out, prompts[n])
		for _, m := range deps[n] {
			indeg[m]--
			if indeg[m] == 0 {
				ready = append(ready, m)
			}
		}
	}
	if len(out) != len(prompts) {
		var cyc []string
		for i, pr := range prompts {
			if indeg[i] > 0 {
				cyc = append(cyc, fmt.Sprintf("%s (needs %s)", p.Rel(pr.Path), strings.Join(pr.DependsOn(), ", ")))
			}
		}
		return nil, fmt.Errorf("circular dependency between prompts:\n  %s", strings.Join(cyc, "\n  "))
	}
	return out, nil
}

// Collect turns command-line paths into prompts: a file is loaded, a
// directory contributes every prompt file directly inside it (not
// recursively), in name order.
func Collect(paths []string) ([]*prompt.Prompt, error) {
	var out []*prompt.Prompt
	seen := map[string]bool{}
	add := func(path string) error {
		pr, err := prompt.Load(path)
		if err != nil {
			return err
		}
		if !seen[pr.Path] {
			seen[pr.Path] = true
			out = append(out, pr)
		}
		return nil
	}
	for _, path := range paths {
		st, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !st.IsDir() {
			if err := add(path); err != nil {
				return nil, err
			}
			continue
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}
		var errs []error
		found := false
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
				continue
			}
			full := filepath.Join(path, e.Name())
			data, err := os.ReadFile(full)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(string(data), "---") {
				continue // notes, not a prompt
			}
			if err := add(full); err != nil {
				errs = append(errs, err)
				continue
			}
			found = true
		}
		if len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		if !found {
			return nil, fmt.Errorf("%s: no prompt files found", path)
		}
	}
	return out, nil
}
