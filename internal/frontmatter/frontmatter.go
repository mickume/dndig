// Package frontmatter parses the YAML subset dndig uses in prompt files,
// style files and dndig.yaml.
//
// The subset is deliberately small and fully specified here, so a prompt
// file is never at the mercy of a YAML corner case:
//
//   - `key: value` pairs at the top level; keys are lowercase identifiers
//     with `_` or `-`;
//   - scalars: bare words, "double-quoted" and 'single-quoted' strings;
//   - flow lists `[a, "b c", d]` and block lists (`- item` on following
//     lines);
//   - a list item may be a flow map `{path: x.png, as: "the bridge"}` with
//     scalar values only;
//   - `#` starts a comment outside quotes; blank lines are ignored.
//
// Anything else (nested maps, multi-line scalars, anchors) is a parse error
// that names the line, rather than a silently misread value.
package frontmatter

import (
	"errors"
	"fmt"
	"strings"
)

// Document is a parsed file: the ordered frontmatter fields and the body
// that followed the closing `---`.
type Document struct {
	Fields []Field
	Body   string
	// HasFrontmatter reports whether the file opened with a `---` block.
	HasFrontmatter bool
}

// Field is one top-level entry. Exactly one of Scalar, List is meaningful,
// selected by Kind.
type Field struct {
	Key  string
	Kind Kind
	// Scalar is the value for KindScalar. Quoted reports whether it was
	// quoted, which matters for numbers ("2K" vs 2).
	Scalar string
	Quoted bool
	// List holds the items for KindList.
	List []Item
	Line int
}

// Item is a list element: a scalar, or a small flow map of scalars.
type Item struct {
	Scalar string
	Map    map[string]string // nil for a scalar item
}

// Kind classifies a field's value.
type Kind int

const (
	KindScalar Kind = iota
	KindList
)

// ErrNoFrontmatter is returned by Parse for text that does not open with
// a `---` line.
var ErrNoFrontmatter = errors.New("frontmatter: no opening --- block")

// Get returns the field named key, if present.
func (d Document) Get(key string) (Field, bool) {
	for _, f := range d.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// Parse splits text into frontmatter and body. Text without a `---` opener
// is returned whole as the body with HasFrontmatter false and a nil error;
// a block that opens and never closes is an error.
func Parse(text string) (Document, error) {
	text = strings.TrimPrefix(text, "\uFEFF")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		return Document{Body: strings.TrimSpace(text)}, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return Document{}, errors.New("frontmatter: the --- block is never closed")
	}
	fields, err := ParseFields(lines[1:end], 2)
	if err != nil {
		return Document{}, err
	}
	body := strings.Join(lines[end+1:], "\n")
	return Document{Fields: fields, Body: strings.TrimSpace(body), HasFrontmatter: true}, nil
}

// ParseFields parses key/value lines (a frontmatter block or a whole
// dndig.yaml). firstLine is the 1-based line number of lines[0], for
// error messages.
func ParseFields(lines []string, firstLine int) ([]Field, error) {
	var out []Field
	seen := map[string]bool{}
	for i := 0; i < len(lines); i++ {
		ln := firstLine + i
		raw := strings.TrimRight(lines[i], "\r")
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return nil, fmt.Errorf("line %d: unexpected indentation (nested values are not supported)", ln)
		}
		key, rest, ok := strings.Cut(line, ":")
		if !ok || !validKey(key) {
			return nil, fmt.Errorf("line %d: expected `key: value`, got %q", ln, strings.TrimSpace(raw))
		}
		if seen[key] {
			return nil, fmt.Errorf("line %d: duplicate key %q", ln, key)
		}
		seen[key] = true
		rest = strings.TrimSpace(rest)
		f := Field{Key: key, Line: ln}
		switch {
		case rest == "":
			// A block list follows, or the value is empty.
			items, n, err := parseBlockList(lines[i+1:], ln+1)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				f.Kind = KindScalar
			} else {
				f.Kind = KindList
				f.List = items
				i += n
			}
		case strings.HasPrefix(rest, "["):
			items, err := parseFlowList(rest, ln)
			if err != nil {
				return nil, err
			}
			f.Kind = KindList
			f.List = items
		default:
			s, quoted, err := parseScalar(rest, ln)
			if err != nil {
				return nil, err
			}
			f.Scalar, f.Quoted = s, quoted
		}
		out = append(out, f)
	}
	return out, nil
}

func validKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r == '_':
		case r == '-' && i > 0, r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

// stripComment removes a `#` comment that is outside quotes. A `#` that
// is not preceded by whitespace or the start of the value is part of the
// value (YAML's rule), so `title: chapter#3` survives.
func stripComment(line string) string {
	var quote rune
	for i, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == '#':
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		}
	}
	return line
}

func parseScalar(s string, ln int) (string, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false, nil
	}
	switch s[0] {
	case '"', '\'':
		q := s[0]
		if len(s) < 2 || s[len(s)-1] != q {
			return "", false, fmt.Errorf("line %d: unterminated quoted string", ln)
		}
		inner := s[1 : len(s)-1]
		if q == '"' {
			inner = strings.ReplaceAll(inner, `\"`, `"`)
		} else {
			inner = strings.ReplaceAll(inner, `''`, `'`)
		}
		return inner, true, nil
	case '{':
		return "", false, fmt.Errorf("line %d: a map is only allowed as a list item", ln)
	}
	return s, false, nil
}

// parseFlowList parses `[a, b, {k: v}]`. Items are split on commas that
// are outside quotes and braces.
func parseFlowList(s string, ln int) ([]Item, error) {
	s = strings.TrimSpace(s)
	if !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("line %d: unterminated list", ln)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return []Item{}, nil
	}
	parts, err := splitTop(inner, ',', ln)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(parts))
	for _, p := range parts {
		it, err := parseItem(p, ln)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, nil
}

// parseBlockList consumes `- item` lines. It returns the items and the
// number of lines consumed (0 when the next line is not a list item).
func parseBlockList(lines []string, firstLine int) ([]Item, int, error) {
	var items []Item
	n := 0
	for i, raw := range lines {
		line := stripComment(strings.TrimRight(raw, "\r"))
		t := strings.TrimSpace(line)
		if t == "" {
			if items == nil {
				return nil, 0, nil
			}
			// A blank line inside a block list ends it only if what follows
			// is not another item; keep scanning.
			continue
		}
		if !strings.HasPrefix(t, "- ") && t != "-" {
			break
		}
		it, err := parseItem(strings.TrimSpace(strings.TrimPrefix(t, "-")), firstLine+i)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, it)
		n = i + 1
	}
	if items == nil {
		return nil, 0, nil
	}
	return items, n, nil
}

func parseItem(s string, ln int) (Item, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "{") {
		if !strings.HasSuffix(s, "}") {
			return Item{}, fmt.Errorf("line %d: unterminated map item", ln)
		}
		inner := strings.TrimSpace(s[1 : len(s)-1])
		m := map[string]string{}
		if inner == "" {
			return Item{Map: m}, nil
		}
		pairs, err := splitTop(inner, ',', ln)
		if err != nil {
			return Item{}, err
		}
		for _, p := range pairs {
			k, v, ok := strings.Cut(p, ":")
			k = strings.TrimSpace(k)
			if !ok || !validKey(k) {
				return Item{}, fmt.Errorf("line %d: expected `key: value` inside {...}, got %q", ln, strings.TrimSpace(p))
			}
			val, _, err := parseScalar(v, ln)
			if err != nil {
				return Item{}, err
			}
			m[k] = val
		}
		return Item{Map: m}, nil
	}
	val, _, err := parseScalar(s, ln)
	if err != nil {
		return Item{}, err
	}
	if val == "" {
		return Item{}, fmt.Errorf("line %d: empty list item", ln)
	}
	return Item{Scalar: val}, nil
}

// splitTop splits s on sep outside quotes and braces.
func splitTop(s string, sep rune, ln int) ([]string, error) {
	var out []string
	var cur strings.Builder
	var quote rune
	depth := 0
	for _, r := range s {
		switch {
		case quote != 0:
			cur.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
			cur.WriteRune(r)
		case r == '{':
			depth++
			cur.WriteRune(r)
		case r == '}':
			depth--
			cur.WriteRune(r)
		case r == sep && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("line %d: unterminated quoted string", ln)
	}
	if depth != 0 {
		return nil, fmt.Errorf("line %d: unbalanced braces", ln)
	}
	out = append(out, cur.String())
	return out, nil
}
