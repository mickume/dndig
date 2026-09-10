package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeRT answers every request with a scripted SSE body and keeps what was
// sent.
type fakeRT struct {
	sse    string
	status int
	bodies [][]byte
	urls   []string
	keys   []string
}

func (f *fakeRT) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	f.bodies = append(f.bodies, b)
	f.urls = append(f.urls, r.URL.String())
	f.keys = append(f.keys, r.Header.Get("x-goog-api-key"))
	status := f.status
	if status == 0 {
		status = 200
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(f.sse)), Request: r}, nil
}

const okSSE = `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"thinking...","thought":true},{"inlineData":{"mimeType":"image/png","data":"AAAA"},"thought":true}]},"index":0}],"responseId":"r1","modelVersion":"gemini-3-pro-image"}

data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Here is Kaelen.","thoughtSignature":"TSIG"},{"inlineData":{"mimeType":"image/png","data":"iVBORw0KGgo="},"thoughtSignature":"ISIG"}]},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":1120,"totalTokenCount":1220}}

`

func newClient(rt *fakeRT) *Client {
	return New(Options{Transport: rt, Env: map[string]string{"GEMINI_API_KEY": "test-key"}, Retries: 0})
}

func TestGenerateSendsImageConfigAndDecodesTheFinalImage(t *testing.T) {
	rt := &fakeRT{sse: okSSE}
	c := newClient(rt)
	temp := 0.8
	seed := int64(42)
	res, err := c.Generate(context.Background(), Request{
		Model: "gemini-3-pro-image", System: "Painterly.", Text: "Kaelen the ranger",
		Images:      []Image{{Data: []byte("ref"), Mime: "image/jpeg"}},
		AspectRatio: "2:3", Resolution: "2K", Temperature: &temp, Seed: &seed, Search: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rt.urls[0], "/v1beta/models/gemini-3-pro-image:streamGenerateContent") || rt.keys[0] != "test-key" {
		t.Fatalf("url = %s key = %q", rt.urls[0], rt.keys[0])
	}
	var body map[string]any
	if err := json.Unmarshal(rt.bodies[0], &body); err != nil {
		t.Fatal(err)
	}
	gc := body["generationConfig"].(map[string]any)
	ic := gc["imageConfig"].(map[string]any)
	if ic["aspectRatio"] != "2:3" || ic["imageSize"] != "2K" || gc["seed"] != float64(42) || gc["temperature"] != 0.8 {
		t.Fatalf("generationConfig = %v", gc)
	}
	if mods, _ := json.Marshal(gc["responseModalities"]); string(mods) != `["IMAGE","TEXT"]` {
		t.Fatalf("responseModalities = %s", mods)
	}
	if _, ok := gc["maxOutputTokens"]; ok {
		t.Fatalf("maxOutputTokens must not be sent: %v", gc)
	}
	if tools, _ := json.Marshal(body["tools"]); string(tools) != `[{"googleSearch":{}}]` {
		t.Fatalf("tools = %s", tools)
	}
	if sys, _ := json.Marshal(body["systemInstruction"]); !strings.Contains(string(sys), "Painterly.") {
		t.Fatalf("systemInstruction = %s", sys)
	}
	contents := body["contents"].([]any)
	parts := contents[0].(map[string]any)["parts"].([]any)
	if len(parts) != 2 || parts[0].(map[string]any)["text"] != "Kaelen the ranger" {
		t.Fatalf("parts = %v", parts)
	}
	if inline := parts[1].(map[string]any)["inlineData"].(map[string]any); inline["mimeType"] != "image/jpeg" || inline["data"] != "cmVm" {
		t.Fatalf("inline = %v", inline)
	}

	if res.Image == nil || res.Image.Mime != "image/png" || string(res.Image.Data) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("image = %+v", res.Image)
	}
	if res.Text != "Here is Kaelen." || res.FinishReason != "STOP" || res.ResponseID != "r1" {
		t.Fatalf("res = %+v", res)
	}
	var kinds []string
	for _, b := range res.Blocks {
		kinds = append(kinds, b.Type)
	}
	if got := strings.Join(kinds, ","); got != "text,signature,image,signature" {
		t.Fatalf("blocks = %s (thought parts must be dropped, signatures kept in order)", got)
	}
	if res.OutputTokens != 1120 || res.CostUSD <= 0.13 || res.CostUSD > 0.14 {
		t.Fatalf("usage = %d tokens, $%.4f (want the Pro output rate)", res.OutputTokens, res.CostUSD)
	}
}

func TestGenerateWithoutOptionalFieldsSendsNone(t *testing.T) {
	rt := &fakeRT{sse: okSSE}
	if _, err := newClient(rt).Generate(context.Background(), Request{Text: "x", AspectRatio: "1:1", Resolution: "1K"}); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(rt.bodies[0], &body)
	gc := body["generationConfig"].(map[string]any)
	for _, k := range []string{"seed", "temperature", "thinkingConfig"} {
		if _, ok := gc[k]; ok {
			t.Fatalf("%s must be absent when unset: %v", k, gc)
		}
	}
	if _, ok := body["tools"]; ok {
		t.Fatal("tools must be absent unless search is on")
	}
	if _, ok := body["systemInstruction"]; ok {
		t.Fatal("systemInstruction must be absent without a style")
	}
	if !strings.Contains(rt.urls[0], DefaultModel) {
		t.Fatalf("default model not used: %s", rt.urls[0])
	}
}

func TestRefinementReplaysTheThreadWithSignatures(t *testing.T) {
	rt := &fakeRT{sse: okSSE}
	prev := Turn{
		Text:   "Kaelen the ranger",
		Images: []Image{{Data: []byte("ref"), Mime: "image/jpeg"}},
		Model: []Block{
			{Type: "text", Text: "Here is Kaelen."},
			{Type: "signature", Signature: "TSIG"},
			{Type: "image", Image: &Image{Data: []byte("img1"), Mime: "image/png"}},
			{Type: "signature", Signature: "ISIG"},
		},
		ModelID: "gemini-3-pro-image",
	}
	_, err := newClient(rt).Generate(context.Background(), Request{
		Model: "gemini-3-pro-image", Text: "Make the beard longer; keep everything else exactly the same.",
		History: []Turn{prev}, AspectRatio: "2:3", Resolution: "2K",
	})
	if err != nil {
		t.Fatal(err)
	}
	body := string(rt.bodies[0])
	var parsed struct {
		Contents []struct {
			Role  string           `json:"role"`
			Parts []map[string]any `json:"parts"`
		} `json:"contents"`
	}
	if err := json.Unmarshal(rt.bodies[0], &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Contents) != 3 || parsed.Contents[0].Role != "user" || parsed.Contents[1].Role != "model" || parsed.Contents[2].Role != "user" {
		t.Fatalf("contents roles wrong: %s", body)
	}
	model := parsed.Contents[1].Parts
	var seq []string
	for _, p := range model {
		switch {
		case p["inlineData"] != nil:
			seq = append(seq, "image")
		case p["text"] != nil:
			seq = append(seq, "text")
		case p["thoughtSignature"] != nil:
			seq = append(seq, "sig:"+p["thoughtSignature"].(string))
		}
	}
	if got := strings.Join(seq, ","); got != "text,sig:TSIG,image,sig:ISIG" {
		t.Fatalf("model turn replayed as %s", got)
	}
	if !strings.Contains(body, `"data":"aW1nMQ=="`) {
		t.Fatalf("previous image not replayed: %s", body)
	}
	if last := parsed.Contents[2].Parts[0]["text"]; last != "Make the beard longer; keep everything else exactly the same." {
		t.Fatalf("instruction = %v", last)
	}
}

func TestCrossModelRefinementStripsSignatures(t *testing.T) {
	rt := &fakeRT{sse: okSSE}
	prev := Turn{Text: "x", Model: []Block{{Type: "text", Text: "Hi"}, {Type: "signature", Signature: "TSIG"}}, ModelID: "gemini-3-pro-image"}
	if _, err := newClient(rt).Generate(context.Background(), Request{Model: "gemini-3.1-flash-image", Text: "y", History: []Turn{prev}, Resolution: "1K"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(rt.bodies[0]), "TSIG") {
		t.Fatalf("a signature from another model must not be replayed: %s", rt.bodies[0])
	}
}

func TestNoImageAndRefusalAreErrorsThatNameTheReason(t *testing.T) {
	for name, tc := range map[string]struct{ sse, want string }{
		"refused":  {`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"I cannot."}]},"finishReason":"IMAGE_SAFETY"}]}` + "\n\n", "refused (finishReason IMAGE_SAFETY): I cannot."},
		"no image": {`data: {"candidates":[{"content":{"role":"model","parts":[{"text":"Text only."}]},"finishReason":"NO_IMAGE"}]}` + "\n\n", "no image in the response (finishReason NO_IMAGE)"},
	} {
		t.Run(name, func(t *testing.T) {
			rt := &fakeRT{sse: tc.sse}
			_, err := newClient(rt).Generate(context.Background(), Request{Text: "x", Resolution: "1K"})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestHTTPErrorsSurface(t *testing.T) {
	rt := &fakeRT{sse: `{"error":{"code":400,"message":"Multiple candidates is not enabled","status":"INVALID_ARGUMENT"}}`, status: 400}
	_, err := newClient(rt).Generate(context.Background(), Request{Text: "x", Resolution: "1K"})
	if err == nil || !strings.Contains(err.Error(), "Multiple candidates") {
		t.Fatalf("err = %v", err)
	}
}

func TestModelValidation(t *testing.T) {
	c := newClient(&fakeRT{sse: okSSE})
	if _, err := c.Generate(context.Background(), Request{Model: "gemini-3-pro-image-preview", Text: "x"}); err == nil || !strings.Contains(err.Error(), "shut down") {
		t.Fatalf("retired model: %v", err)
	}
	if _, err := c.Generate(context.Background(), Request{Model: "gemini-3-pro-image", Text: "x", Resolution: "512"}); err == nil || !strings.Contains(err.Error(), "512") {
		t.Fatalf("unsupported size: %v", err)
	}
	if _, err := c.Generate(context.Background(), Request{Model: "gemini-3.1-flash-lite-image", Text: "x", Resolution: "1K", Images: make([]Image, 4)}); err == nil || !strings.Contains(err.Error(), "at most 3") {
		t.Fatalf("too many refs: %v", err)
	}
	m, spec, err := c.Model("google/gemini-3.1-flash-image")
	if err != nil || m.ID != "gemini-3.1-flash-image" || spec.Name != "Nano Banana 2" || m.Cost.Output != 60 || !m.SupportsImages() {
		t.Fatalf("model = %+v spec = %+v err = %v", m, spec, err)
	}
	if _, _, err := c.Model("gemini-9-image"); err != nil {
		t.Fatalf("an unknown future model must still resolve: %v", err)
	}
}

func TestCheckCredentials(t *testing.T) {
	if err := New(Options{Env: map[string]string{}}).CheckCredentials(); err == nil && envHasGoogleKey() {
		t.Skip("a real key is set in the environment")
	} else if err == nil {
		t.Fatal("expected an error without a key")
	}
	if err := New(Options{APIKey: "k"}).CheckCredentials(); err != nil {
		t.Fatal(err)
	}
	if err := New(Options{Env: map[string]string{"GOOGLE_API_KEY": "k"}}).CheckCredentials(); err != nil {
		t.Fatal(err)
	}
}

func TestAPIKeyFlagBeatsEveryEnvironmentName(t *testing.T) {
	rt := &fakeRT{sse: okSSE}
	c := New(Options{Transport: rt, APIKey: "flag-key", Env: map[string]string{"GOOGLE_GENERATIVE_AI_API_KEY": "old-env-key"}})
	if _, err := c.Generate(context.Background(), Request{Text: "x", Resolution: "1K"}); err != nil {
		t.Fatal(err)
	}
	if rt.keys[0] != "flag-key" {
		t.Fatalf("sent key %q, want the flag's", rt.keys[0])
	}
}
