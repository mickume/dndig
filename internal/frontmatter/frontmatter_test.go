package frontmatter

import (
	"strings"
	"testing"
)

func TestParseSplitsFrontmatterAndBody(t *testing.T) {
	doc, err := Parse("---\ntitle: kaelen\naspect_ratio: \"2:3\" # portrait\ncast: [borin, \"old nan\"]\n---\n\nA wood-elf ranger.\n")
	if err != nil {
		t.Fatal(err)
	}
	if !doc.HasFrontmatter || doc.Body != "A wood-elf ranger." {
		t.Fatalf("doc = %+v", doc)
	}
	if f, _ := doc.Get("title"); f.Scalar != "kaelen" || f.Quoted {
		t.Fatalf("title = %+v", f)
	}
	if f, _ := doc.Get("aspect_ratio"); f.Scalar != "2:3" || !f.Quoted {
		t.Fatalf("aspect_ratio = %+v (the comment must be stripped, the quotes honoured)", f)
	}
	f, _ := doc.Get("cast")
	if f.Kind != KindList || len(f.List) != 2 || f.List[0].Scalar != "borin" || f.List[1].Scalar != "old nan" {
		t.Fatalf("cast = %+v", f)
	}
}

func TestParseWithoutFrontmatterIsJustABody(t *testing.T) {
	doc, err := Parse("Just a prompt.\n")
	if err != nil || doc.HasFrontmatter || doc.Body != "Just a prompt." {
		t.Fatalf("doc = %+v, err = %v", doc, err)
	}
}

func TestBlockListsAndMapItems(t *testing.T) {
	doc, err := Parse(strings.Join([]string{
		"---",
		"references:",
		"  - ../refs/bridge.jpg",
		"  - {path: ../refs/sword.png, as: \"Kaelen's sword\"}",
		"takes: 3",
		"---",
		"body",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	f, ok := doc.Get("references")
	if !ok || f.Kind != KindList || len(f.List) != 2 {
		t.Fatalf("references = %+v", f)
	}
	if f.List[0].Scalar != "../refs/bridge.jpg" {
		t.Fatalf("item 0 = %+v", f.List[0])
	}
	if m := f.List[1].Map; m["path"] != "../refs/sword.png" || m["as"] != "Kaelen's sword" {
		t.Fatalf("item 1 = %+v", f.List[1])
	}
	if f, _ := doc.Get("takes"); f.Scalar != "3" {
		t.Fatalf("takes = %+v (the field after a block list must still parse)", f)
	}
}

func TestErrorsNameTheLine(t *testing.T) {
	for name, in := range map[string]string{
		"unclosed block": "---\ntitle: x\n",
		"nested map":     "---\nimage:\n  size: 2K\n---\n",
		"duplicate key":  "---\ntitle: a\ntitle: b\n---\n",
		"bad key":        "---\nTitle: a\n---\n",
		"unterminated":   "---\ntitle: \"a\n---\n",
		"bad list":       "---\ncast: [a, b\n---\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(in); err == nil {
				t.Fatalf("no error for %q", in)
			}
		})
	}
	_, err := Parse("---\ntitle: a\nTitle: b\n---\n")
	if err == nil || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("err = %v, want the line number", err)
	}
}

func TestHashInsideAValueIsNotAComment(t *testing.T) {
	doc, err := Parse("---\ntitle: chapter#3\nnote: \"# not a comment\"\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	if f, _ := doc.Get("title"); f.Scalar != "chapter#3" {
		t.Fatalf("title = %q", f.Scalar)
	}
	if f, _ := doc.Get("note"); f.Scalar != "# not a comment" {
		t.Fatalf("note = %q", f.Scalar)
	}
}

func TestEmptyValues(t *testing.T) {
	doc, err := Parse("---\nstyle:\ncast: []\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	if f, _ := doc.Get("style"); f.Kind != KindScalar || f.Scalar != "" {
		t.Fatalf("style = %+v", f)
	}
	if f, _ := doc.Get("cast"); f.Kind != KindList || len(f.List) != 0 {
		t.Fatalf("cast = %+v", f)
	}
}
