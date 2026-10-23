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
	if w.ImagesDir() != filepath.Join(filepath.Dir(w.Dir), "images") || w.PickPath() != filepath.Join(w.ImagesDir(), "kaelen.png") {
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

func TestPickMovesAndPruneMoves(t *testing.T) {
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
	if _, err := os.Stat(w.TakePath("002.png")); err == nil {
		t.Fatal("pick must move the take out of takes/")
	}
	if w.ImagePath("002.png") != w.PickPath() || w.ImagePath("001.png") != w.TakePath("001.png") {
		t.Fatalf("image paths = %s %s", w.ImagePath("002.png"), w.ImagePath("001.png"))
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

func TestPruneKeepsThePicksAncestry(t *testing.T) {
	w := newWS(t)
	save(t, w, 1, "")
	child := Take{Number: 2, Parent: 1, Response: Response{Blocks: []Block{{Type: "image"}}}}
	if _, err := w.Save(child, []byte("png2"), "image/png"); err != nil {
		t.Fatal(err)
	}
	save(t, w, 3, "")
	if _, err := w.PickTake(2); err != nil {
		t.Fatal(err)
	}
	moved, err := w.Prune(false)
	if err != nil || len(moved) != 1 || moved[0] != 3 {
		t.Fatalf("pruned = %v, %v (the pick's parent must stay)", moved, err)
	}
	if _, err := w.Take(1); err != nil {
		t.Fatalf("ancestor gone: %v", err)
	}
}

// Picking another take returns the previous pick to takes/, so no image is
// ever lost, and the pick keeps the extension of its take.
func TestRepickReturnsThePreviousPick(t *testing.T) {
	w := newWS(t)
	save(t, w, 1, "")
	jpg := Take{Number: 2, Response: Response{Blocks: []Block{{Type: "image"}}}}
	if _, err := w.Save(jpg, []byte("jpg2"), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.PickTake(2); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(w.Dir), "images", "kaelen.jpg")
	if w.PickPath() != want {
		t.Fatalf("pick = %s, want %s", w.PickPath(), want)
	}
	p, err := w.ReadPick()
	if err != nil || p.Image != "kaelen.jpg" || p.Source != "002.jpg" || p.SHA256 != SHA256([]byte("jpg2")) {
		t.Fatalf("sidecar = %+v, %v", p, err)
	}
	if _, err := w.PickTake(2); err != nil {
		t.Fatalf("picking the pick again must be a no-op: %v", err)
	}
	if _, err := w.PickTake(1); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(w.TakePath("002.jpg")); string(data) != "jpg2" {
		t.Fatalf("the previous pick was not returned to takes/: %q", data)
	}
	if _, err := os.Stat(want); err == nil {
		t.Fatal("the previous pick is still in images/")
	}
	if data, _ := os.ReadFile(w.PickPath()); string(data) != "png1" || filepath.Base(w.PickPath()) != "kaelen.png" {
		t.Fatalf("new pick = %s %q", w.PickPath(), data)
	}
}

// An image at the pick path that no sidecar accounts for is never
// overwritten: it may be the only copy of a take.
func TestPickRefusesAnUnrecordedImage(t *testing.T) {
	w := newWS(t)
	save(t, w, 1, "")
	if err := os.MkdirAll(w.ImagesDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.ImagesDir(), "kaelen.png"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := w.PickTake(1); err == nil {
		t.Fatal("pick must refuse to replace an image it has no record of")
	}
	if data, _ := os.ReadFile(filepath.Join(w.ImagesDir(), "kaelen.png")); string(data) != "mine" {
		t.Fatalf("image was replaced: %q", data)
	}
}
