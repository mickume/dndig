package workspace

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newWS(t *testing.T) Workspace {
	t.Helper()
	root := t.TempDir()
	return For(filepath.Join(root, "characters", "kaelen.md"))
}

func save(t *testing.T, w Workspace, n int, purpose string) Take {
	t.Helper()
	tk := Take{Number: n, Created: time.Now(), Title: "kaelen", Model: "m", Purpose: purpose,
		Response: Response{Blocks: []Block{{Type: "text", Text: "ok"}, {Type: "signature", Signature: "S"}, {Type: "image"}}, FinishReason: "STOP"}}
	saved, err := w.Save(tk, []byte("png"+string(rune('0'+n))), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

func TestLayoutAndNumbering(t *testing.T) {
	w := newWS(t)
	if w.Dir != filepath.Join(filepath.Dir(filepath.Dir(w.PickPath())), "kaelen") || filepath.Base(w.PickPath()) != "kaelen.png" {
		t.Fatalf("layout = %+v pick=%s", w, w.PickPath())
	}
	n, err := w.Reserve(2)
	if err != nil || n != 1 {
		t.Fatalf("reserve = %d, %v", n, err)
	}
	a := save(t, w, 1, "")
	b := save(t, w, 2, "")
	if a.Image != "001.png" || b.Image != "002.png" || a.Response.Blocks[2].Image != "001.png" {
		t.Fatalf("names = %s %s blocks=%+v", a.Image, b.Image, a.Response.Blocks)
	}
	if n, _ := w.Reserve(1); n != 3 {
		t.Fatalf("next = %d, want 3", n)
	}
	ts, err := w.Takes()
	if err != nil || len(ts) != 2 || ts[1].Number != 2 || ts[1].Response.Text() != "ok" {
		t.Fatalf("takes = %+v, %v", ts, err)
	}
	if _, err := w.Save(Take{Number: 1}, []byte("x"), "image/png"); err == nil {
		t.Fatal("a take must never be overwritten")
	}
	latest, ok, _ := w.Latest()
	if !ok || latest.Number != 2 {
		t.Fatalf("latest = %+v", latest)
	}
}

func TestPickCopiesAndPruneMoves(t *testing.T) {
	w := newWS(t)
	save(t, w, 1, "")
	save(t, w, 2, "")
	save(t, w, 3, "sheet")
	if w.HasPick() {
		t.Fatal("no pick yet")
	}
	p, err := w.PickTake(2)
	if err != nil || p.Take != 2 {
		t.Fatalf("pick = %+v, %v", p, err)
	}
	data, _ := os.ReadFile(w.PickPath())
	if string(data) != "png2" || !w.HasPick() {
		t.Fatalf("pick file = %q", data)
	}
	if _, err := os.Stat(w.TakePath("002.png")); err != nil {
		t.Fatal("pick must copy, not move")
	}
	moved, err := w.Prune(false)
	if err != nil || len(moved) != 1 || moved[0] != 1 {
		t.Fatalf("pruned = %v, %v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(w.Dir, DiscardsDir, "001.json")); err != nil {
		t.Fatal("discard sidecar missing")
	}
	ts, _ := w.Takes()
	if len(ts) != 2 {
		t.Fatalf("remaining takes = %d (pick and sheet must stay)", len(ts))
	}
	if n, _ := w.Reserve(1); n != 4 {
		t.Fatalf("numbers must not be reused after a prune: %d", n)
	}
	if _, err := w.PickTake(9); err == nil {
		t.Fatal("picking a missing take must fail")
	}
}

func TestPruneDelete(t *testing.T) {
	w := newWS(t)
	save(t, w, 1, "")
	if _, err := w.Prune(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(w.TakePath("001.png")); err == nil {
		t.Fatal("not deleted")
	}
}
