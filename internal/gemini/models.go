package gemini

import (
	"fmt"
	"sort"
	"strings"
)

// Spec describes one Gemini image model: what it accepts and what it
// costs. The numbers come from docs/research/01_gemini_image_api.md and
// are the tool's own knowledge, not the kit's catalog (which has no image
// rows); they are overridden onto the resolved catalog model.
type Spec struct {
	ID      string
	Name    string
	Sizes   []string // wire spellings: 512, 1K, 2K, 4K
	MaxRefs int      // reference images per request
	// MaxPeople is how many people's likeness the docs say it preserves.
	MaxPeople int
	// InputPerMTok / OutputPerMTok are USD per million tokens. Output
	// tokens are how images are billed (1120 per 1K image, 1680 per 2K,
	// 2520 per 4K).
	InputPerMTok  float64
	OutputPerMTok float64
}

// DefaultModel is used when neither the prompt nor dndig.yaml names one.
const DefaultModel = "gemini-3-pro-image"

// DefaultVisionModel is the text+vision model `style derive` uses.
const DefaultVisionModel = "gemini-3.1-pro-preview"

var models = map[string]Spec{
	"gemini-3-pro-image": {
		ID: "gemini-3-pro-image", Name: "Nano Banana Pro",
		Sizes: []string{"1K", "2K", "4K"}, MaxRefs: 14, MaxPeople: 5,
		InputPerMTok: 2.0, OutputPerMTok: 120.0,
	},
	"gemini-3.1-flash-image": {
		ID: "gemini-3.1-flash-image", Name: "Nano Banana 2",
		Sizes: []string{"512", "1K", "2K", "4K"}, MaxRefs: 14, MaxPeople: 5,
		InputPerMTok: 0.5, OutputPerMTok: 60.0,
	},
	"gemini-3.1-flash-lite-image": {
		ID: "gemini-3.1-flash-lite-image", Name: "Nano Banana 2 Lite",
		Sizes: []string{"1K"}, MaxRefs: 3, MaxPeople: 1,
		InputPerMTok: 0.25, OutputPerMTok: 30.0,
	},
}

// retired maps model ids that no longer exist to advice.
var retired = map[string]string{
	"gemini-3-pro-image-preview":                "shut down 2026-06-25; use gemini-3-pro-image",
	"gemini-3.1-flash-image-preview":            "shut down 2026-06-25; use gemini-3.1-flash-image",
	"gemini-2.5-flash-image":                    "shuts down 2026-10-02; use gemini-3.1-flash-image",
	"gemini-2.5-flash-image-preview":            "retired; use gemini-3.1-flash-image",
	"gemini-2.0-flash-preview-image-generation": "retired; use gemini-3.1-flash-image",
}

// Lookup returns the spec for a model id. An unknown id that is not known
// to be retired gets a permissive spec (every size, 14 references), so a
// model released after this build still works.
func Lookup(id string) (Spec, error) {
	id = strings.TrimPrefix(id, "google/")
	if s, ok := models[id]; ok {
		return s, nil
	}
	if why, ok := retired[id]; ok {
		return Spec{}, fmt.Errorf("model %q: %s", id, why)
	}
	return Spec{
		ID: id, Name: id, Sizes: []string{"512", "1K", "2K", "4K"}, MaxRefs: 14, MaxPeople: 5,
		InputPerMTok: 2.0, OutputPerMTok: 120.0,
	}, nil
}

// Known lists the model ids this build knows, sorted.
func Known() []string {
	out := make([]string, 0, len(models))
	for id := range models {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// SupportsSize reports whether the model accepts an image size.
func (s Spec) SupportsSize(size string) bool {
	for _, v := range s.Sizes {
		if v == size {
			return true
		}
	}
	return false
}
