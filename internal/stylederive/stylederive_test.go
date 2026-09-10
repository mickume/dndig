package stylederive

import (
	"context"
	"strings"
	"testing"

	"github.com/agentfox/agentkit-go/core"
	"github.com/agentfox/agentkit-go/provider/faux"

	"github.com/mickume/dndig/internal/gemini"
)

const submission = `{"summary":"Loose gouache storybook","medium":"gouache on paper","brushwork":"loose, visible strokes","palette":["ochre","teal","dusty rose"],"lighting":"golden-hour rim light","composition":"low horizons, centred figures","texture":"paper grain","mood":"warm, wistful","avoid":["photorealism","text"],"directive":"Paint in loose gouache on textured paper with visible directional brush strokes and soft edges. Use a muted palette of ochre, teal and dusty rose with warm golden-hour rim light and gentle atmospheric haze. Keep compositions simple with low horizons and centred figures. Preserve visible paper grain. The mood is warm and wistful. Not photorealistic, no text, no watermarks."}`

func deps(p *faux.Provider) Deps {
	return Deps{Model: faux.Model(), Registry: core.ProviderRegistry{faux.API: p.APIProvider()}}
}

func TestDeriveReturnsTheSubmittedStyle(t *testing.T) {
	p := faux.New(
		faux.FauxAssistantMessage(core.StopReasonToolUse, faux.FauxText("Looking..."), faux.FauxToolCall("c1", ToolName, submission)),
		faux.FauxAssistantMessage(core.StopReasonStop, faux.FauxText("done")),
	)
	d, _, err := Derive(context.Background(), deps(p), []gemini.Image{{Data: []byte("png"), Mime: "image/png"}}, "grim north")
	if err != nil {
		t.Fatal(err)
	}
	if d.Summary != "Loose gouache storybook" || len(d.Palette) != 3 || d.Avoid[1] != "text" || !strings.HasPrefix(d.Directive, "Paint in loose gouache") {
		t.Fatalf("derived = %+v", d)
	}
	if p.Calls() != 1 {
		t.Fatalf("calls = %d, want the run to stop at the terminating tool", p.Calls())
	}
	req := p.Requests()[0]
	if len(req.Tools) != 1 || req.Tools[0].Name != ToolName {
		t.Fatalf("tools = %+v", req.Tools)
	}
	user := req.Messages[0].(core.UserMessage)
	if len(user.Content) != 2 {
		t.Fatalf("user content = %+v, want text + image", user.Content)
	}
	if txt := user.Content[0].(core.TextBlock).Text; !strings.Contains(txt, "grim north") {
		t.Fatalf("hint missing: %s", txt)
	}
	if _, ok := user.Content[1].(core.ImageBlock); !ok {
		t.Fatal("image not sent")
	}
	if !strings.Contains(req.System[0].(core.TextBlock).Text, "art director") {
		t.Fatalf("system = %+v", req.System)
	}
}

func TestDeriveFailsWhenTheModelOnlyTalks(t *testing.T) {
	p := faux.New(faux.FauxAssistantMessage(core.StopReasonStop, faux.FauxText("It is painterly.")))
	_, _, err := Derive(context.Background(), deps(p), []gemini.Image{{Data: []byte("png"), Mime: "image/png"}}, "")
	if err == nil || !strings.Contains(err.Error(), "did not submit") || !strings.Contains(err.Error(), "painterly") {
		t.Fatalf("err = %v", err)
	}
}

func TestAShortDirectiveIsRejectedAndRetried(t *testing.T) {
	short := strings.Replace(submission, `"directive":"Paint in loose gouache`, `"directive":"Gouache. Paint in loose gouache`, 1)
	short = short[:strings.Index(short, `"directive":"`)+len(`"directive":"`)] + `too short"}`
	p := faux.New(
		faux.FauxAssistantMessage(core.StopReasonToolUse, faux.FauxToolCall("c1", ToolName, short)),
		faux.FauxAssistantMessage(core.StopReasonToolUse, faux.FauxToolCall("c2", ToolName, submission)),
	)
	d, _, err := Derive(context.Background(), deps(p), []gemini.Image{{Data: []byte("png"), Mime: "image/png"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if p.Calls() != 2 || d.Summary == "" {
		t.Fatalf("calls = %d, derived = %+v", p.Calls(), d)
	}
	// The error result reached the model as a tool result.
	second := p.Requests()[1]
	last := second.Messages[len(second.Messages)-1].(core.ToolResultMessage)
	if !last.IsError && !strings.Contains(last.Content.Text(), "40 words") {
		t.Fatalf("tool result = %+v", last)
	}
}

func TestDeriveNeedsExamples(t *testing.T) {
	if _, _, err := Derive(context.Background(), deps(faux.New()), nil, ""); err == nil {
		t.Fatal("no examples must fail")
	}
}
