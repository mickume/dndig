package cli

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type scripted struct {
	mu     sync.Mutex
	bodies [][]byte
	sse    string
	status int
}

func (s *scripted) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	s.bodies = append(s.bodies, b)
	s.mu.Unlock()
	status := s.status
	if status == 0 {
		status = 200
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(s.sse)), Request: r}, nil
}

func pngBase64(t *testing.T) (string, []byte) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < 8; i++ {
		img.Set(i, i, color.RGBA{200, 30, 30, 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	var b64 bytes.Buffer
	enc := json.NewEncoder(&b64)
	_ = enc.Encode(buf.Bytes()) // json encodes []byte as base64
	return strings.Trim(strings.TrimSpace(b64.String()), `"`), buf.Bytes()
}

func sseWithImage(b64 string) string {
	return `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Here.","thoughtSignature":"T"},{"inlineData":{"mimeType":"image/png","data":"` + b64 + `"},"thoughtSignature":"I"}]},"finishReason":"STOP"}],"responseId":"r","modelVersion":"gemini-3-pro-image","usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":1120}}` + "\n\n"
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := Main(args, &out, &errb)
	return code, out.String(), errb.String()
}

func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testEnv = map[string]string{"GEMINI_API_KEY": "test"}
	t.Cleanup(func() { testTransport, testEnv = nil, nil })
	code, out, errs := run(t, "init", dir)
	if code != 0 {
		t.Fatalf("init: %d %s %s", code, out, errs)
	}
	return dir
}

func TestInitStatusAndDryRun(t *testing.T) {
	dir := setup(t)
	for _, f := range []string{"dndig.yaml", "styles/campaign.md", "characters/kaelen.md", "scenes/ambush.md"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("init did not create %s", f)
		}
	}
	code, out, _ := run(t, "status", dir)
	if code != 0 || !strings.Contains(out, "characters/kaelen.md") || !strings.Contains(out, "needs: kaelen (no pick)") {
		t.Fatalf("status: %d\n%s", code, out)
	}
	code, out, _ = run(t, "generate", "--dry-run", filepath.Join(dir, "characters", "kaelen.md"))
	if code != 0 || !strings.Contains(out, "model:        gemini-3-pro-image") || !strings.Contains(out, "Art style: In the style of tabletop RPG") || !strings.Contains(out, "Kaelen, a wood-elf ranger") {
		t.Fatalf("dry-run: %d\n%s", code, out)
	}
	code, _, errs := run(t, "generate", "--dry-run", filepath.Join(dir, "scenes", "ambush.md"))
	if code != 1 || !strings.Contains(errs, "dndig pick characters/kaelen.md") {
		t.Fatalf("scene without pick: %d %s", code, errs)
	}
}

func TestGeneratePickSceneRefineAndPrune(t *testing.T) {
	dir := setup(t)
	b64, raw := pngBase64(t)
	rt := &scripted{sse: sseWithImage(b64)}
	testTransport = rt

	kaelen := filepath.Join(dir, "characters", "kaelen.md")
	code, out, errs := run(t, "generate", "--takes", "2", kaelen)
	if code != 0 || !strings.Contains(out, "take 001 -> characters/kaelen/takes/001.png") || !strings.Contains(out, "take 002") {
		t.Fatalf("generate: %d\n%s\n%s", code, out, errs)
	}
	if len(rt.bodies) != 2 {
		t.Fatalf("requests = %d", len(rt.bodies))
	}
	got, _ := os.ReadFile(filepath.Join(dir, "characters", "kaelen", "takes", "001.png"))
	if !bytes.Equal(got, raw) {
		t.Fatal("saved image differs from the returned one")
	}
	side, _ := os.ReadFile(filepath.Join(dir, "characters", "kaelen", "takes", "001.json"))
	var take map[string]any
	_ = json.Unmarshal(side, &take)
	if take["model"] != "gemini-3-pro-image" || take["prompt_file"] != "characters/kaelen.md" || take["usage"].(map[string]any)["cost_usd"].(float64) <= 0 {
		t.Fatalf("sidecar = %s", side)
	}
	blocks := take["response"].(map[string]any)["blocks"].([]any)
	if len(blocks) != 4 || blocks[3].(map[string]any)["signature"] != "I" {
		t.Fatalf("blocks = %v", blocks)
	}

	code, out, _ = run(t, "pick", kaelen, "2")
	if code != 0 || !strings.Contains(out, "take 002 -> characters/kaelen/kaelen.png") {
		t.Fatalf("pick: %d %s", code, out)
	}

	scene := filepath.Join(dir, "scenes", "ambush.md")
	code, out, errs = run(t, "generate", "-v", scene)
	if code != 0 || !strings.Contains(out, "take 001 -> scenes/ambush/takes/001.png") {
		t.Fatalf("scene: %d\n%s\n%s", code, out, errs)
	}
	body := string(rt.bodies[2])
	if !strings.Contains(body, "Image 1 is kaelen (a character)") || strings.Count(body, `"inlineData"`) != 1 {
		t.Fatalf("scene request did not carry the pick:\n%s", body[:min(len(body), 600)])
	}
	if !strings.Contains(errs, "Image 1:      characters/kaelen/kaelen.png (cast:kaelen)") {
		t.Fatalf("verbose plan missing:\n%s", errs)
	}

	code, out, errs = run(t, "refine", kaelen, "add", "a", "scar")
	if code != 0 || !strings.Contains(out, "refining take 002") || !strings.Contains(out, "take 003 ->") {
		t.Fatalf("refine: %d\n%s\n%s", code, out, errs)
	}
	body = string(rt.bodies[3])
	if !strings.Contains(body, `"role":"model"`) || !strings.Contains(body, `"thoughtSignature":"I"`) || !strings.Contains(body, "add a scar Keep everything else") {
		t.Fatalf("refine request did not replay the thread:\n%s", body[:min(len(body), 800)])
	}
	side, _ = os.ReadFile(filepath.Join(dir, "characters", "kaelen", "takes", "003.json"))
	_ = json.Unmarshal(side, &take)
	if take["parent"] != float64(2) || take["instruction"] != "add a scar" {
		t.Fatalf("refined sidecar = %s", side)
	}

	code, out, errs = run(t, "sheet", kaelen)
	if code != 0 || !strings.Contains(out, "sheet -> characters/kaelen/sheet.png") {
		t.Fatalf("sheet: %d\n%s\n%s", code, out, errs)
	}

	code, out, _ = run(t, "prune", kaelen)
	if code != 0 || !strings.Contains(out, "2 take(s) moved to discards/") {
		t.Fatalf("prune: %d %s", code, out)
	}
	code, out, _ = run(t, "status", dir)
	if code != 0 || !strings.Contains(out, "pick: take 002, sheet") {
		t.Fatalf("status after: %d\n%s", code, out)
	}
}

func TestGenerateReportsFailuresAndKeepsGoing(t *testing.T) {
	dir := setup(t)
	testTransport = &scripted{sse: `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"no"}]},"finishReason":"IMAGE_SAFETY"}]}` + "\n\n"}
	code, _, errs := run(t, "generate", filepath.Join(dir, "characters", "kaelen.md"))
	if code != 1 || !strings.Contains(errs, "IMAGE_SAFETY") {
		t.Fatalf("code = %d errs = %s", code, errs)
	}
}

func TestUsageErrors(t *testing.T) {
	if code, _, _ := run(t); code != 2 {
		t.Fatal("no args must be a usage error")
	}
	if code, _, errs := run(t, "bogus"); code != 2 || !strings.Contains(errs, "unknown command") {
		t.Fatal("unknown command")
	}
	if code, _, _ := run(t, "pick", "x.md"); code != 2 {
		t.Fatal("pick without a take must be a usage error")
	}
	if code, out, _ := run(t, "version"); code != 0 || !strings.HasPrefix(out, "dndig ") {
		t.Fatal("version")
	}
}

func TestMissingCredentialsFailBeforeAnyRequest(t *testing.T) {
	dir := setup(t)
	testEnv = map[string]string{}
	rt := &scripted{}
	testTransport = rt
	code, _, errs := run(t, "generate", filepath.Join(dir, "characters", "kaelen.md"))
	if code != 1 || !strings.Contains(errs, "GEMINI_API_KEY") || len(rt.bodies) != 0 {
		t.Fatalf("code = %d errs = %s requests = %d", code, errs, len(rt.bodies))
	}
}

func TestFlagsMayFollowPositionals(t *testing.T) {
	dir := setup(t)
	code, out, _ := run(t, "generate", filepath.Join(dir, "characters", "kaelen.md"), "--dry-run", "--takes", "2", "--model=gemini-3.1-flash-image")
	if code != 0 || !strings.Contains(out, "(2 take(s))") || !strings.Contains(out, "model:        gemini-3.1-flash-image") {
		t.Fatalf("code = %d\n%s", code, out)
	}
	code, out, _ = run(t, "style", "show", "campaign", "--from", filepath.Join(dir, "characters", "kaelen.md"))
	if code != 0 || !strings.Contains(out, "style campaign (styles/campaign.md)") {
		t.Fatalf("style show: %d\n%s", code, out)
	}
	if code, _, errs := run(t, "generate", "--bogus", "x.md"); code != 1 && !strings.Contains(errs, "bogus") {
		t.Fatalf("unknown flag: %d %s", code, errs)
	}
}
