package assemble

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickume/dndig/internal/project"
	"github.com/mickume/dndig/internal/prompt"
	"github.com/mickume/dndig/internal/workspace"
)

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (*project.Project, []byte) {
	t.Helper()
	root := t.TempDir()
	small := pngBytes(t, 16, 16)
	write(t, filepath.Join(root, "dndig.yaml"), []byte("style: campaign\n"))
	write(t, filepath.Join(root, "styles", "campaign.md"), []byte("---\nreferences: [../refs/cover.png]\n---\nPainterly gouache, muted palette.\n"))
	write(t, filepath.Join(root, "refs", "cover.png"), small)
	write(t, filepath.Join(root, "refs", "bridge.png"), small)
	write(t, filepath.Join(root, "characters", "kaelen.md"), []byte("---\nkind: character\naspect_ratio: \"2:3\"\n---\nKaelen, a wood-elf ranger.\n"))
	write(t, filepath.Join(root, "characters", "borin.md"), []byte("---\nkind: character\n---\nBorin, a dwarf.\n"))
	write(t, filepath.Join(root, "places", "tower.md"), []byte("---\nkind: location\n---\nA tower.\n"))
	write(t, filepath.Join(root, "scenes", "ambush.md"), []byte("---\nkind: scene\naspect_ratio: \"16:9\"\nresolution: 2K\ncast: [kaelen, borin]\nreferences: [tower, {path: ../refs/bridge.png, as: \"the old bridge\"}]\nseed: 7\n---\nKaelen and Borin ambushed on the bridge below the tower at dusk.\n"))
	proj, err := project.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := proj.Index(); err != nil {
		t.Fatal(err)
	}
	return proj, small
}

func pick(t *testing.T, proj *project.Project, title string, data []byte) workspace.Workspace {
	t.Helper()
	p, _ := proj.Lookup(title)
	ws := workspace.For(p.Path)
	tk := workspace.Take{Number: 1, Title: title, Response: workspace.Response{Blocks: []workspace.Block{{Type: "image"}}}}
	if _, err := ws.Save(tk, data, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := ws.PickTake(1); err != nil {
		t.Fatal(err)
	}
	return ws
}

func TestGenerateCharacterWithStyleOnly(t *testing.T) {
	proj, _ := fixture(t)
	p, _ := proj.Lookup("kaelen")
	plan, err := Generate(p, proj)
	if err != nil {
		t.Fatal(err)
	}
	r := plan.Request
	if r.Model != "gemini-3-pro-image" || r.AspectRatio != "2:3" || r.Resolution != "1K" || r.System != "Painterly gouache, muted palette." {
		t.Fatalf("request = %+v", r)
	}
	if len(r.Images) != 1 || plan.Images[0].Role != "style" {
		t.Fatalf("images = %+v", plan.Images)
	}
	want := "Art style: Painterly gouache, muted palette.\n\nReference images, in order:\nImage 1 is a style reference only: borrow its palette, brushwork and lighting, not its content.\n\nKaelen, a wood-elf ranger."
	if r.Text != want {
		t.Fatalf("text =\n%s\nwant\n%s", r.Text, want)
	}
}

func TestSceneNeedsPicksAndLabelsThem(t *testing.T) {
	proj, small := fixture(t)
	scene, _ := proj.Lookup("ambush")
	if _, err := Generate(scene, proj); err == nil || !strings.Contains(err.Error(), "dndig pick characters/kaelen.md") {
		t.Fatalf("missing pick error = %v", err)
	}
	kws := pick(t, proj, "kaelen", small)
	pick(t, proj, "borin", small)
	pick(t, proj, "tower", small)
	if err := kws.SaveSheet(small); err != nil {
		t.Fatal(err)
	}
	plan, err := Generate(scene, proj)
	if err != nil {
		t.Fatal(err)
	}
	var roles []string
	for _, ref := range plan.Images {
		roles = append(roles, ref.Label+"="+ref.Role)
	}
	if got := strings.Join(roles, " "); got != "Image 1=cast:kaelen Image 2=sheet:kaelen Image 3=cast:borin Image 4=cast:tower Image 5=reference Image 6=style" {
		t.Fatalf("roles = %s", got)
	}
	text := plan.Request.Text
	for _, want := range []string{
		"Image 1 is kaelen (a character): keep the face, hair, build, clothing and signature props exactly as shown.",
		"Image 2 is kaelen again, a turnaround sheet of the same character from more angles.",
		"Image 4 is tower (a location): keep its architecture, layout and setting as shown.",
		"Image 5 is a reference for the old bridge.",
		"Image 6 is a style reference only",
		"Do not blend facial traits",
		"\n\nKaelen and Borin ambushed",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if plan.Request.Seed == nil || *plan.Request.Seed != 7 || plan.Request.Resolution != "2K" || plan.Request.AspectRatio != "16:9" {
		t.Fatalf("settings = %+v", plan.Request)
	}
	if plan.Images[0].SHA256 != workspace.SHA256(small) || plan.Images[0].Path != "characters/kaelen/kaelen.png" {
		t.Fatalf("provenance = %+v", plan.Images[0])
	}
	desc := plan.Describe(proj)
	if !strings.Contains(desc, "Image 3:      characters/borin/borin.png (cast:borin)") || !strings.Contains(desc, "seed 7") {
		t.Fatalf("describe =\n%s", desc)
	}
}

func TestLargeReferencesAreDownscaled(t *testing.T) {
	proj, _ := fixture(t)
	big := pngBytes(t, 2400, 1200)
	write(t, filepath.Join(proj.Root, "refs", "bridge.png"), big)
	p, _ := proj.Lookup("ambush")
	p.Cast = nil
	p.References = p.References[1:]
	plan, err := Generate(p, proj)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Request.Images[0].Data) >= len(big) {
		t.Fatal("a 2400px reference must be downscaled to the inline budget")
	}
}

func TestRefineReplaysTheChain(t *testing.T) {
	proj, small := fixture(t)
	p, _ := proj.Lookup("kaelen")
	ws := workspace.For(p.Path)
	root := workspace.Take{Number: 1, Title: "kaelen", Model: "gemini-3-pro-image", System: "S", Text: "T1", AspectRatio: "2:3", Resolution: "1K",
		Response: workspace.Response{Blocks: []workspace.Block{{Type: "text", Text: "ok"}, {Type: "signature", Signature: "A"}, {Type: "image"}, {Type: "signature", Signature: "B"}}}}
	if _, err := ws.Save(root, small, "image/png"); err != nil {
		t.Fatal(err)
	}
	child := workspace.Take{Number: 2, Parent: 1, Title: "kaelen", Model: "gemini-3-pro-image", System: "S", Text: "longer beard", AspectRatio: "2:3", Resolution: "1K",
		Response: workspace.Response{Blocks: []workspace.Block{{Type: "image"}, {Type: "signature", Signature: "C"}}}}
	saved, err := ws.Save(child, small, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Refine(p, proj, saved, "Add a scar")
	if err != nil {
		t.Fatal(err)
	}
	h := plan.Request.History
	if len(h) != 2 || h[0].Text != "T1" || h[1].Text != "longer beard" || h[0].ModelID != "gemini-3-pro-image" {
		t.Fatalf("history = %+v", h)
	}
	if len(h[0].Model) != 4 || h[0].Model[2].Image == nil || h[0].Model[3].Signature != "B" {
		t.Fatalf("root model turn = %+v", h[0].Model)
	}
	if plan.Request.System != "S" || !strings.HasPrefix(plan.Request.Text, "Add a scar Keep everything else") {
		t.Fatalf("request = %+v", plan.Request)
	}
}

func TestSheetUsesThePick(t *testing.T) {
	proj, small := fixture(t)
	p, _ := proj.Lookup("kaelen")
	if _, err := Sheet(p, proj); err == nil {
		t.Fatal("sheet without a pick must fail")
	}
	pick(t, proj, "kaelen", small)
	plan, err := Sheet(p, proj)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Request.Images) != 1 || plan.Images[0].Role != "cast:kaelen" || plan.Request.AspectRatio != "16:9" {
		t.Fatalf("plan = %+v", plan)
	}
	if !strings.Contains(plan.Request.Text, "turnaround sheet of kaelen") {
		t.Fatalf("text = %s", plan.Request.Text)
	}
}

func TestModelResolutionOrder(t *testing.T) {
	proj, _ := fixture(t)
	if ModelFor(&prompt.Prompt{}, proj) != "gemini-3-pro-image" {
		t.Fatal("default")
	}
	proj.Config.Model = "gemini-3.1-flash-image"
	if ModelFor(&prompt.Prompt{}, proj) != "gemini-3.1-flash-image" || ModelFor(&prompt.Prompt{Model: "x"}, proj) != "x" {
		t.Fatal("precedence")
	}
}

func TestRefineRefusesAChangedReference(t *testing.T) {
	proj, small := fixture(t)
	scene, _ := proj.Lookup("ambush")
	scene.Cast = []string{"kaelen"}
	scene.References = nil
	kws := pick(t, proj, "kaelen", small)
	plan, err := Generate(scene, proj)
	if err != nil {
		t.Fatal(err)
	}
	ws := workspace.For(scene.Path)
	take := workspace.Take{Number: 1, Model: "gemini-3-pro-image", Text: plan.Request.Text, Images: plan.Images, AspectRatio: "16:9", Resolution: "2K",
		Response: workspace.Response{Blocks: []workspace.Block{{Type: "image"}}}}
	saved, err := ws.Save(take, small, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Refine(scene, proj, saved, "darker"); err != nil {
		t.Fatalf("unchanged pick must replay: %v", err)
	}
	// A new pick for kaelen: the scene's thread no longer matches.
	other := pngBytes(t, 12, 12)
	if _, err := kws.Save(workspace.Take{Number: 2, Response: workspace.Response{Blocks: []workspace.Block{{Type: "image"}}}}, other, "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, err := kws.PickTake(2); err != nil {
		t.Fatal(err)
	}
	if _, err := Refine(scene, proj, saved, "darker"); err == nil || !strings.Contains(err.Error(), "has changed since") {
		t.Fatalf("err = %v", err)
	}
}

func TestRefineBudgetsTheReplayedImage(t *testing.T) {
	proj, _ := fixture(t)
	p, _ := proj.Lookup("kaelen")
	ws := workspace.For(p.Path)
	big := pngBytes(t, 2400, 2400)
	saved, err := ws.Save(workspace.Take{Number: 1, Model: "gemini-3-pro-image", Text: "T", Response: workspace.Response{Blocks: []workspace.Block{{Type: "image"}}}}, big, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Refine(p, proj, saved, "x")
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.Request.History[0].Model[0].Image; got == nil || len(got.Data) >= len(big) {
		t.Fatal("the replayed take image must be downscaled to the inline budget")
	}
}
