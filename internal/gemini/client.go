// Package gemini generates images with Google's Gemini image models through
// the AgentKit's Google provider.
//
// The kit speaks generateContent, decodes inlineData parts into
// core.ImageBlock and keeps thought signatures; what it does not know about
// is image generation itself, so this package injects the image fields
// (responseModalities, imageConfig, seed, the search tool) into the wire
// payload through core.RequestOptions.OnPayload and reads the image back
// from the canonical message.
package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/agentfox/agentkit-go/catalog"
	"github.com/agentfox/agentkit-go/core"
	"github.com/agentfox/agentkit-go/provider"
	"github.com/agentfox/agentkit-go/provider/google"
)

// Image is raw image bytes with their MIME type.
type Image struct {
	Data []byte
	Mime string
}

// Block is one part of a model turn, in wire order, as stored in a take
// sidecar: text, an opaque thought signature, or an image.
type Block struct {
	Type      string // text | signature | image
	Text      string
	Signature string
	Image     *Image
}

// Turn is one completed exchange in a refinement thread: what the user
// sent and what the model answered, block by block.
type Turn struct {
	Text   string
	Images []Image
	Model  []Block
	// ModelID is the model that produced the answer; a replay on another
	// model strips the signatures, which is the kit's cross-model rule.
	ModelID string
}

// Request is one image generation.
type Request struct {
	Model       string
	System      string
	Text        string
	Images      []Image
	History     []Turn
	AspectRatio string
	Resolution  string
	Temperature *float64
	Seed        *int64
	Search      bool
}

// Result is what came back.
type Result struct {
	// Image is the final image (thought images are not included).
	Image *Image
	// Blocks is the whole model turn in wire order, for the sidecar.
	Blocks       []Block
	Text         string
	FinishReason string
	ResponseID   string
	ModelVersion string
	InputTokens  int64
	OutputTokens int64
	CostUSD      float64
}

// Options configure a Client. Every field is optional.
type Options struct {
	// Transport replaces the HTTP transport (tests).
	Transport http.RoundTripper
	// Env overrides environment variables for credential resolution.
	Env map[string]string
	// APIKey, when set, is used instead of the environment.
	APIKey  string
	Retries int
	Timeout time.Duration
	// Warnf receives provider warnings (transcript repairs); nil is silent.
	Warnf func(format string, args ...any)
}

// Client holds the provider registry.
type Client struct {
	opts Options
	reg  core.ProviderRegistry
}

// New creates a client. No network is touched.
func New(opts Options) *Client {
	if opts.Retries == 0 {
		opts.Retries = 2
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	reg := core.ProviderRegistry{}
	reg.Register(google.Provider(google.Options{}))
	return &Client{opts: opts, reg: reg}
}

// Registry exposes the provider registry for the style-derivation agent.
func (c *Client) Registry() core.ProviderRegistry { return c.reg }

// RequestOptions returns the per-call options (transport, env, retries)
// shared by every call this client makes.
func (c *Client) RequestOptions() core.RequestOptions {
	env := map[string]string{}
	for k, v := range c.opts.Env {
		env[k] = v
	}
	if c.opts.APIKey != "" {
		// Every name the kit consults, or a key left in the environment
		// under an earlier name would win over the flag.
		for _, v := range google.VendorAuth.Vars {
			env[v.Name] = c.opts.APIKey
		}
	}
	to := int(c.opts.Timeout / time.Millisecond)
	retries := c.opts.Retries
	return core.RequestOptions{Transport: c.opts.Transport, Env: env, TimeoutMs: &to, MaxRetries: &retries}
}

// CheckCredentials fails before any request when no Google credential is
// configured, naming the variables to set.
func (c *Client) CheckCredentials() error {
	if c.opts.APIKey != "" {
		return nil
	}
	env := provider.Env{Override: c.opts.Env}
	if c.opts.Env != nil {
		env.Getenv = func(string) string { return "" }
	}
	if provider.ResolveAuth(google.VendorAuth, env).State != provider.CredentialNone {
		return nil
	}
	var names []string
	for _, v := range google.VendorAuth.Vars {
		names = append(names, v.Name)
	}
	return fmt.Errorf("no Google API key: set one of %s (https://aistudio.google.com/apikey), or pass --api-key", strings.Join(names, ", "))
}

// Model resolves an id against the kit catalog. Image models are not in
// the catalog, so the result is a sibling clone of the Google default row;
// the fields that matter for an image call are overridden from Spec.
func (c *Client) Model(id string) (*core.Model, Spec, error) {
	spec, err := Lookup(id)
	if err != nil {
		return nil, spec, err
	}
	m, err := catalog.ResolveModel("google/" + spec.ID)
	if err != nil {
		return nil, spec, err
	}
	m.Name = spec.Name
	m.Input = []string{"text", "image"}
	m.Cost = core.Cost{Input: spec.InputPerMTok, Output: spec.OutputPerMTok}
	// Thinking is the model's own business: never send a thinkingConfig.
	m.ThinkingLevelMap = nil
	return m, spec, nil
}

// VisionModel resolves a text+vision model for style derivation.
func (c *Client) VisionModel(id string) (*core.Model, error) {
	return catalog.ResolveModel("google/" + strings.TrimPrefix(id, "google/"))
}

// Generate performs one image generation.
func (c *Client) Generate(ctx context.Context, req Request) (*Result, error) {
	if req.Model == "" {
		req.Model = DefaultModel
	}
	m, spec, err := c.Model(req.Model)
	if err != nil {
		return nil, err
	}
	if req.Resolution != "" && !spec.SupportsSize(req.Resolution) {
		return nil, fmt.Errorf("model %s does not produce %s images (it supports %s)", spec.ID, req.Resolution, strings.Join(spec.Sizes, ", "))
	}
	if n := len(req.Images); n > spec.MaxRefs {
		return nil, fmt.Errorf("%d reference images, but %s accepts at most %d", n, spec.ID, spec.MaxRefs)
	}

	opts := c.RequestOptions()
	opts.OnPayload = payloadHook(req)
	creq := core.Request{
		Messages:    buildMessages(req, m),
		Temperature: req.Temperature,
		Options:     opts,
	}
	if req.System != "" {
		creq.System = []core.ContentBlock{core.TextBlock{Text: req.System}}
	}
	msg := core.Complete(ctx, core.ClientFunc(c.reg.Dispatch), m, creq, core.ProviderStreamOptions{Warnf: c.opts.Warnf})
	if msg == nil {
		return nil, errors.New("gemini: the provider returned no message")
	}
	res := &Result{
		FinishReason: msg.RawStopReason,
		ResponseID:   msg.ResponseID,
		ModelVersion: msg.ResponseModel,
		InputTokens:  msg.Usage.InputTokens,
		OutputTokens: msg.Usage.OutputTokens,
		CostUSD:      msg.Usage.CostUSD,
	}
	if msg.StopReason == core.StopReasonError {
		return res, fmt.Errorf("gemini: %s", msg.ErrorMessage)
	}
	if msg.StopReason == core.StopReasonAborted {
		return res, ctx.Err()
	}
	for _, b := range msg.Content {
		switch v := b.(type) {
		case core.TextBlock:
			res.Blocks = append(res.Blocks, Block{Type: "text", Text: v.Text})
		case core.ThinkingBlock:
			if v.Signature != "" && v.Signature != google.ThoughtMarkerSignature {
				res.Blocks = append(res.Blocks, Block{Type: "signature", Signature: v.Signature})
			}
		case core.ImageBlock:
			data, err := base64.StdEncoding.DecodeString(v.Data)
			if err != nil {
				return res, fmt.Errorf("gemini: decoding the returned image: %w", err)
			}
			img := &Image{Data: data, Mime: v.MimeType}
			res.Blocks = append(res.Blocks, Block{Type: "image", Image: img})
			res.Image = img // the last image wins; thought images never reach here
		}
	}
	var texts []string
	for _, b := range res.Blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			texts = append(texts, strings.TrimSpace(b.Text))
		}
	}
	res.Text = strings.Join(texts, "\n")
	if res.Image == nil {
		reason := res.FinishReason
		if reason == "" {
			reason = "unknown"
		}
		if msg.StopReason == core.StopReasonRefusal {
			return res, fmt.Errorf("gemini: the request was refused (finishReason %s)%s", reason, quoteText(res.Text))
		}
		return res, fmt.Errorf("gemini: no image in the response (finishReason %s)%s", reason, quoteText(res.Text))
	}
	return res, nil
}

func quoteText(s string) string {
	if s == "" {
		return ""
	}
	return ": " + s
}

// buildMessages renders the thread and the new user turn as canonical
// messages. A model turn is reconstructed with the provenance the kit's
// same-model rule needs, so signatures survive a same-model replay and are
// stripped on a cross-model one.
func buildMessages(req Request, m *core.Model) core.Messages {
	var msgs core.Messages
	for _, t := range req.History {
		msgs = append(msgs, userMessage(t.Text, t.Images))
		var content core.Content
		for _, b := range t.Model {
			switch b.Type {
			case "text":
				content = append(content, core.TextBlock{Text: b.Text})
			case "signature":
				content = append(content, core.ThinkingBlock{Signature: b.Signature})
			case "image":
				if b.Image != nil {
					content = append(content, core.ImageBlock{Data: base64.StdEncoding.EncodeToString(b.Image.Data), MimeType: b.Image.Mime})
				}
			}
		}
		modelID := t.ModelID
		if modelID == "" {
			modelID = m.ID
		}
		msgs = append(msgs, core.AssistantMessage{
			Content: content, StopReason: core.StopReasonStop, RawStopReason: "STOP",
			Provider: m.Provider, API: m.API, Model: modelID, Timestamp: time.Now(),
		})
	}
	return append(msgs, userMessage(req.Text, req.Images))
}

func userMessage(text string, images []Image) core.UserMessage {
	content := core.Content{core.TextBlock{Text: text}}
	for _, img := range images {
		content = append(content, core.ImageBlock{Data: base64.StdEncoding.EncodeToString(img.Data), MimeType: img.Mime})
	}
	return core.UserMessage{Content: content, Timestamp: time.Now()}
}

// payloadHook adds the image-generation fields to the wire payload the
// kit built. The kit's request type is unexported, so the payload is
// re-encoded through a map; the kit marshals whatever is returned.
func payloadHook(req Request) func(payload any, m *core.Model) (any, error) {
	return func(payload any, _ *core.Model) (any, error) {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			return nil, err
		}
		gc, _ := body["generationConfig"].(map[string]any)
		if gc == nil {
			gc = map[string]any{}
		}
		gc["responseModalities"] = []string{"IMAGE", "TEXT"}
		ic := map[string]any{}
		if req.AspectRatio != "" {
			ic["aspectRatio"] = req.AspectRatio
		}
		if req.Resolution != "" {
			ic["imageSize"] = req.Resolution
		}
		if len(ic) > 0 {
			gc["imageConfig"] = ic
		}
		if req.Seed != nil {
			gc["seed"] = *req.Seed
		}
		// The kit clamps max_tokens for text models; an image model bills by
		// image and the field only limits the caption.
		delete(gc, "maxOutputTokens")
		delete(gc, "thinkingConfig")
		body["generationConfig"] = gc
		if req.Search {
			body["tools"] = []map[string]any{{"googleSearch": map[string]any{}}}
		}
		return body, nil
	}
}
