package prompt

import (
	"path/filepath"
	"strings"
	"testing"
)

const kaelen = `---
title: kaelen
kind: character
style: campaign
aspect_ratio: "2:3"
resolution: 2k
takes: 3
temperature: 0.7
seed: 42
references: [../refs/bow.png, {path: ../refs/cloak.jpg, as: "her green cloak"}]
---
Kaelen is a wood-elf ranger with a scar over the left eye.
`

func TestParseFullPrompt(t *testing.T) {
	p, err := Parse("/camp/characters/kaelen.md", kaelen)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "kaelen" || p.Kind != KindCharacter || p.Style != "campaign" {
		t.Fatalf("p = %+v", p)
	}
	if p.AspectRatio != "2:3" || p.Resolution != "2K" || p.Takes != 3 {
		t.Fatalf("settings = %s %s %d", p.AspectRatio, p.Resolution, p.Takes)
	}
	if p.Temperature == nil || *p.Temperature != 0.7 || p.Seed == nil || *p.Seed != 42 {
		t.Fatalf("temperature/seed = %v %v", p.Temperature, p.Seed)
	}
	if len(p.References) != 2 || p.References[0].Path != filepath.Clean("/camp/refs/bow.png") || p.References[1].As != "her green cloak" {
		t.Fatalf("references = %+v", p.References)
	}
	if p.Workspace() != filepath.Clean("/camp/characters/kaelen") {
		t.Fatalf("workspace = %s", p.Workspace())
	}
	if !strings.HasPrefix(p.Body, "Kaelen is") {
		t.Fatalf("body = %q", p.Body)
	}
}

func TestDefaultsAndAliases(t *testing.T) {
	p, err := Parse("/camp/scenes/ambush.md", "---\ninstructions: ../styles/old.md\nbatch: 2\ncast: [kaelen, borin]\nreferences: [bridge]\n---\nAn ambush.\n")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "ambush" || p.Kind != KindOther || p.AspectRatio != "1:1" || p.Resolution != "1K" {
		t.Fatalf("defaults = %+v", p)
	}
	if p.Style != "../styles/old.md" || p.Takes != 2 {
		t.Fatalf("aliases not honoured: %+v", p)
	}
	if got := p.DependsOn(); strings.Join(got, ",") != "kaelen,borin,bridge" {
		t.Fatalf("depends = %v", got)
	}
	if p.References[0].Title != "bridge" || p.References[0].Path != "" {
		t.Fatalf("a bare name must be a title reference: %+v", p.References[0])
	}
}

func TestValidationErrorsAreCollected(t *testing.T) {
	_, err := Parse("/x/bad.md", "---\naspect_ratio: 7:1\nresolution: 8K\ntakes: 99\nkind: monster\ncolour: red\ncast: [a, b, c, d, e, f]\n---\n\n")
	if err == nil {
		t.Fatal("no error")
	}
	for _, want := range []string{"aspect_ratio", "resolution", "takes", "kind", "unknown field", "at most 5", "body is empty"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}

func TestBodyOnlyIsNotAPrompt(t *testing.T) {
	if _, err := Parse("/x/notes.md", "# Notes\nnothing here\n"); err == nil {
		t.Fatal("a file without frontmatter must be rejected")
	}
}
