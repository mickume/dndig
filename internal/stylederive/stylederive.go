// Package stylederive turns example images into a style directive with an
// AgentKit agent: a vision model looks at the examples and must hand back
// a structured description through one terminating tool, so the result is
// validated data rather than prose to parse.
package stylederive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	agentkit "github.com/agentfox/agentkit-go"
	"github.com/agentfox/agentkit-go/core"
	"github.com/agentfox/agentkit-go/schema"

	"github.com/mickume/dndig/internal/gemini"
	"github.com/mickume/dndig/internal/style"
)

// ToolName is the terminating tool the model must call.
const ToolName = "submit_style"

// Deps are the agent's wiring: injectable so a test can script the model.
type Deps struct {
	Model    *core.Model
	Registry core.ProviderRegistry
	Options  core.RequestOptions
	Warnf    func(format string, args ...any)
}

// DepsFor wires the real Gemini client.
func DepsFor(c *gemini.Client, modelID string) (Deps, error) {
	if modelID == "" {
		modelID = gemini.DefaultVisionModel
	}
	m, err := c.VisionModel(modelID)
	if err != nil {
		return Deps{}, err
	}
	return Deps{Model: m, Registry: c.Registry(), Options: c.RequestOptions()}, nil
}

const systemPrompt = `You are an art director describing the visual style of example illustrations so that an image model can paint NEW pictures in exactly that style.

Look only at HOW the examples are made, never at WHAT they depict: medium and technique, brushwork or linework, palette and colour grading, lighting, texture, composition habits, level of detail, mood. Use concrete, cinematographic vocabulary ("gouache", "visible directional brush strokes", "muted teal and ochre", "chiaroscuro", "golden-hour rim light", "soft film grain"). Do not name characters, places or events from the examples.

When you are done, call submit_style exactly once. The "directive" field is the paragraph that will be sent verbatim to the image model with every prompt: write it as an instruction (120-220 words), covering medium, brushwork, palette, lighting, texture and mood, ending with what to avoid (for example "not photorealistic, no text or watermarks").`

// Derive runs the agent over the examples. hint is optional guidance from
// the user ("this is for a grim northern campaign").
func Derive(ctx context.Context, d Deps, examples []gemini.Image, hint string) (style.Derived, core.Usage, error) {
	if len(examples) == 0 {
		return style.Derived{}, core.Usage{}, errors.New("at least one example image is required")
	}
	var got *style.Derived
	tool := core.Tool{
		Name:        ToolName,
		Description: "Submit the derived style. Call it exactly once, when the analysis is complete.",
		InputSchema: schema.Object(
			schema.Prop("summary", schema.String("One sentence naming the style, e.g. 'loose gouache storybook illustration'.")),
			schema.Prop("medium", schema.String("Medium and technique.")),
			schema.Prop("brushwork", schema.String("Brushwork or linework: edges, stroke visibility, rendering.")),
			schema.Prop("palette", schema.Array(schema.String(), "Dominant colours and colour grading, 3-6 entries.").MinItemsN(1)),
			schema.Prop("lighting", schema.String("Lighting and atmosphere.")),
			schema.Prop("composition", schema.String("Composition and framing habits.")),
			schema.Prop("texture", schema.String("Surface texture, grain, paper or canvas feel.")),
			schema.Prop("mood", schema.String("Emotional tone.")),
			schema.Prop("avoid", schema.Array(schema.String(), "What the style is NOT: things to avoid.").MinItemsN(1)),
			schema.Prop("directive", schema.String("The instruction paragraph sent to the image model with every prompt.")),
		).Closed(),
		Execute: func(_ context.Context, in json.RawMessage) core.ToolResult {
			var v struct {
				Summary, Medium, Brushwork, Lighting, Composition, Texture, Mood, Directive string
				Palette, Avoid                                                              []string
			}
			if err := json.Unmarshal(in, &v); err != nil {
				return core.ErrResult("invalid_arguments", err.Error())
			}
			if len(strings.Fields(v.Directive)) < 40 {
				return core.ErrResult("directive_too_short", "the directive must be a full paragraph of at least 40 words")
			}
			got = &style.Derived{
				Summary: v.Summary, Medium: v.Medium, Brushwork: v.Brushwork, Palette: v.Palette,
				Lighting: v.Lighting, Composition: v.Composition, Texture: v.Texture, Mood: v.Mood,
				Avoid: v.Avoid, Directive: v.Directive,
			}
			return core.ToolResult{OK: true, Data: map[string]any{"status": "recorded"}, Terminate: true}
		},
	}

	cfg := core.AgentConfig{
		Model:          d.Model,
		Providers:      d.Registry,
		SystemPrompt:   systemPrompt,
		RequestOptions: d.Options,
		ToolChoice:     core.ToolChoiceAuto,
		// The run ends when a VALID submission has been recorded — not when
		// the tool was merely called, because a rejected submission comes
		// back to the model as an error it is expected to correct.
		StopPolicy: agentkit.StopAny(
			agentkit.StopAfterTurns(4),
			func(sc core.StopContext) bool {
				if got == nil {
					return false
				}
				sc.SetReason(core.RunStopPolicy)
				return true
			},
		),
		Hooks: core.Hooks{OnError: func(err error) {
			if d.Warnf != nil {
				d.Warnf("style derive: %v", err)
			}
		}},
	}
	agent, err := agentkit.NewAgent(cfg)
	if err != nil {
		return style.Derived{}, core.Usage{}, err
	}
	if err := agent.RegisterTool(tool); err != nil {
		return style.Derived{}, core.Usage{}, err
	}

	content := core.Content{core.TextBlock{Text: userText(len(examples), hint)}}
	for _, img := range examples {
		content = append(content, core.ImageBlock{Data: base64.StdEncoding.EncodeToString(img.Data), MimeType: img.Mime})
	}
	res, err := agent.RunMessage(ctx, core.UserMessage{Content: content, Timestamp: time.Now()})
	if err != nil {
		return style.Derived{}, res.Usage, err
	}
	if got == nil {
		text := strings.TrimSpace(res.FinalText())
		if text == "" {
			text = "(no text)"
		}
		return style.Derived{}, res.Usage, fmt.Errorf("the model did not submit a style (stop reason %s): %s", res.StopReason, text)
	}
	return *got, res.Usage, nil
}

func userText(n int, hint string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Here are %d example illustration(s). Describe their shared visual style and call %s.", n, ToolName)
	if strings.TrimSpace(hint) != "" {
		fmt.Fprintf(&b, "\n\nContext from the user: %s", strings.TrimSpace(hint))
	}
	return b.String()
}
