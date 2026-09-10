// Package style loads and renders style directive files: Markdown with a
// frontmatter block whose body is the text sent to the image model.
package style

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickume/dndig/internal/frontmatter"
)

// Style is one loaded style file.
type Style struct {
	Path        string
	Name        string
	DerivedFrom []string // example images, informational
	Model       string   // the model that derived it, informational
	Created     string
	// References are style reference images sent with every generation that
	// uses this style, labelled as style-only. Absolute paths.
	References []string
	// Directive is the body: the text sent to the image model.
	Directive string
}

// Load reads a style file.
func Load(path string) (*Style, error) {
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

// Parse parses text as the style file at path.
func Parse(path, text string) (*Style, error) {
	doc, err := frontmatter.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s := &Style{Path: path, Directive: doc.Body}
	s.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	dir := filepath.Dir(path)
	for _, f := range doc.Fields {
		switch f.Key {
		case "name":
			if f.Scalar != "" {
				s.Name = f.Scalar
			}
		case "derived_from":
			s.DerivedFrom = append(s.DerivedFrom, list(f)...)
		case "model":
			s.Model = f.Scalar
		case "created":
			s.Created = f.Scalar
		case "references":
			for _, it := range list(f) {
				if !filepath.IsAbs(it) {
					it = filepath.Join(dir, it)
				}
				s.References = append(s.References, it)
			}
		default:
			return nil, fmt.Errorf("%s:%d: unknown field %q", path, f.Line, f.Key)
		}
	}
	if strings.TrimSpace(s.Directive) == "" {
		return nil, fmt.Errorf("%s: the style directive (the body) is empty", path)
	}
	return s, nil
}

func list(f frontmatter.Field) []string {
	var out []string
	if f.Kind == frontmatter.KindList {
		for _, it := range f.List {
			if it.Scalar != "" {
				out = append(out, it.Scalar)
			}
		}
		return out
	}
	if f.Scalar != "" {
		out = append(out, f.Scalar)
	}
	return out
}

// Derived is what the style-derivation agent hands back: the directive
// paragraph and the structured notes behind it.
type Derived struct {
	Summary     string
	Medium      string
	Brushwork   string
	Palette     []string
	Lighting    string
	Composition string
	Texture     string
	Mood        string
	Avoid       []string
	Directive   string
}

// Render writes a style file for a derived style. Examples are recorded
// as paths relative to the style file so the project stays relocatable.
func Render(name string, d Derived, examples []string, model string, now time.Time, dir string) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", quote(name))
	if len(examples) > 0 {
		b.WriteString("derived_from:\n")
		for _, e := range examples {
			if rel, err := filepath.Rel(dir, e); err == nil {
				e = filepath.ToSlash(rel)
			}
			fmt.Fprintf(&b, "  - %s\n", quote(e))
		}
	}
	if model != "" {
		fmt.Fprintf(&b, "model: %s\n", quote(model))
	}
	fmt.Fprintf(&b, "created: %s\n", now.UTC().Format("2006-01-02"))
	b.WriteString("---\n")
	b.WriteString(strings.TrimSpace(d.Directive))
	b.WriteString("\n\n## Style notes\n\n")
	row := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			fmt.Fprintf(&b, "- **%s:** %s\n", k, strings.TrimSpace(v))
		}
	}
	row("Summary", d.Summary)
	row("Medium", d.Medium)
	row("Brushwork", d.Brushwork)
	row("Palette", strings.Join(d.Palette, ", "))
	row("Lighting", d.Lighting)
	row("Composition", d.Composition)
	row("Texture", d.Texture)
	row("Mood", d.Mood)
	row("Avoid", strings.Join(d.Avoid, ", "))
	return b.String()
}

func quote(s string) string {
	if strings.ContainsAny(s, `:#"'[]{},`) || strings.TrimSpace(s) != s || s == "" {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
