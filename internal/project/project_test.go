package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickume/dndig/internal/prompt"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "dndig.yaml"), "model: gemini-3.1-flash-image\nstyle: campaign\nworkers: 3\n")
	write(t, filepath.Join(root, "styles", "campaign.md"), "---\nname: campaign\n---\nPainterly.\n")
	write(t, filepath.Join(root, "characters", "kaelen.md"), "---\nkind: character\n---\nKaelen.\n")
	write(t, filepath.Join(root, "characters", "borin.md"), "---\nkind: character\nstyle: ../styles/campaign.md\n---\nBorin.\n")
	write(t, filepath.Join(root, "characters", "notes.md"), "# Session notes\nnot a prompt\n")
	write(t, filepath.Join(root, "scenes", "ambush.md"), "---\nkind: scene\ncast: [Kaelen, borin]\nreferences: [bridge]\n---\nAmbush.\n")
	write(t, filepath.Join(root, "places", "bridge.md"), "---\nkind: location\n---\nA bridge.\n")
	write(t, filepath.Join(root, "characters", "kaelen", "takes", "001.md"), "---\ntitle: kaelen\n---\nmust be skipped\n")
	return root
}

func TestFindWalksUpToTheConfig(t *testing.T) {
	root := fixture(t)
	p, err := Find(filepath.Join(root, "scenes", "ambush.md"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Root != root || !p.HasConfig || p.Config.Model != "gemini-3.1-flash-image" || p.Config.Workers != 3 {
		t.Fatalf("p = %+v", p)
	}
	bare := t.TempDir()
	write(t, filepath.Join(bare, "a.md"), "---\n---\nx\n")
	q, err := Find(filepath.Join(bare, "a.md"))
	if err != nil || q.Root != bare || q.HasConfig {
		t.Fatalf("q = %+v, err = %v", q, err)
	}
}

func TestIndexLookupAndStyle(t *testing.T) {
	root := fixture(t)
	p, _ := Find(root)
	if err := p.Index(); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Lookup("KAELEN"); !ok {
		t.Fatal("lookup must be case-insensitive")
	}
	if len(p.Prompts()) != 4 {
		t.Fatalf("prompts = %d, want 4 (takes/ and styles/ skipped, notes skipped)", len(p.Prompts()))
	}
	if len(p.Skipped) != 1 || !strings.HasSuffix(p.Skipped[0].Path, "notes.md") {
		t.Fatalf("skipped = %+v", p.Skipped)
	}
	k, _ := p.Lookup("kaelen")
	s, err := p.ResolveStyle(k.Style, k)
	if err != nil || s == nil || s.Name != "campaign" {
		t.Fatalf("default style: %+v %v", s, err)
	}
	b, _ := p.Lookup("borin")
	if s, err := p.ResolveStyle(b.Style, b); err != nil || s.Name != "campaign" {
		t.Fatalf("path style: %+v %v", s, err)
	}
	if _, err := p.ResolveStyle("missing", k); err == nil {
		t.Fatal("missing style must error")
	}
}

func TestDuplicateTitlesAreAnError(t *testing.T) {
	root := fixture(t)
	write(t, filepath.Join(root, "npcs", "kaelen.md"), "---\n---\nanother kaelen\n")
	p, _ := Find(root)
	if err := p.Index(); err == nil || !strings.Contains(err.Error(), `"kaelen"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestOrderPutsDependenciesFirst(t *testing.T) {
	root := fixture(t)
	p, _ := Find(root)
	_ = p.Index()
	ambush, _ := p.Lookup("ambush")
	kaelen, _ := p.Lookup("kaelen")
	bridge, _ := p.Lookup("bridge")
	borin, _ := p.Lookup("borin")
	out, err := p.Order([]*prompt.Prompt{ambush, kaelen, bridge, borin})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, pr := range out {
		titles = append(titles, pr.Title)
	}
	if got := strings.Join(titles, ","); got != "kaelen,bridge,borin,ambush" {
		t.Fatalf("order = %s", got)
	}
}

func TestOrderDetectsCycles(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "a.md"), "---\ncast: [b]\n---\na\n")
	write(t, filepath.Join(root, "b.md"), "---\ncast: [a]\n---\nb\n")
	p, _ := Find(root)
	_ = p.Index()
	if _, err := p.Order(p.Prompts()); err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("err = %v", err)
	}
}

func TestCollectFilesAndDirectories(t *testing.T) {
	root := fixture(t)
	got, err := Collect([]string{filepath.Join(root, "characters"), filepath.Join(root, "scenes", "ambush.md")})
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, pr := range got {
		titles = append(titles, pr.Title)
	}
	if s := strings.Join(titles, ","); s != "borin,kaelen,ambush" {
		t.Fatalf("collect = %s", s)
	}
	if _, err := Collect([]string{filepath.Join(root, "styles")}); err == nil {
		t.Fatal("a directory with no prompts must error")
	}
}
