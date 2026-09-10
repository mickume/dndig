package style

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseStyleFile(t *testing.T) {
	s, err := Parse("/camp/styles/campaign.md", "---\nname: Campaign\nreferences: [../refs/cover.jpg]\nderived_from:\n  - ../refs/cover.jpg\nmodel: gemini-x\ncreated: 2026-09-10\n---\nPainterly, muted palette.\n")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "Campaign" || s.Directive != "Painterly, muted palette." || s.Model != "gemini-x" {
		t.Fatalf("s = %+v", s)
	}
	if len(s.References) != 1 || s.References[0] != filepath.Clean("/camp/refs/cover.jpg") {
		t.Fatalf("references = %v", s.References)
	}
}

func TestPlainTextStyleFileStillWorks(t *testing.T) {
	// A 1.2.2 style file has no frontmatter at all.
	s, err := Parse("/camp/styles/old.md", "Dramatic cinematic lighting.\n")
	if err != nil || s.Directive != "Dramatic cinematic lighting." || s.Name != "old" {
		t.Fatalf("s = %+v, err = %v", s, err)
	}
}

func TestRenderRoundTrips(t *testing.T) {
	d := Derived{
		Summary: "Storybook fantasy", Medium: "gouache", Brushwork: "loose",
		Palette: []string{"ochre", "teal"}, Lighting: "golden hour", Mood: "warm",
		Avoid:     []string{"photorealism", "text"},
		Directive: "Gouache storybook illustration: loose brushwork, ochre and teal, golden-hour light. Not photorealistic.",
	}
	text := Render("my style: v2", d, []string{"/camp/refs/a.png"}, "gemini-3.1-pro-preview", time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC), "/camp/styles")
	s, err := Parse("/camp/styles/mine.md", text)
	if err != nil {
		t.Fatalf("%v\n%s", err, text)
	}
	if s.Name != "my style: v2" || s.Created != "2026-09-10" || s.DerivedFrom[0] != "../refs/a.png" {
		t.Fatalf("s = %+v\n%s", s, text)
	}
	if !strings.HasPrefix(s.Directive, "Gouache storybook") || !strings.Contains(s.Directive, "**Avoid:** photorealism, text") {
		t.Fatalf("directive = %q", s.Directive)
	}
}
