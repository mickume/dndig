// Package assemble turns a prompt file and its project into one image
// request: the style directive, the labelled reference images from the
// cast, the ad-hoc references, the lock line and the prompt body. It is a
// pure planning step, so `--dry-run` can print exactly what would be sent.
package assemble

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agentfox/agentkit-go/imagex"

	"github.com/mickume/dndig/internal/gemini"
	"github.com/mickume/dndig/internal/project"
	"github.com/mickume/dndig/internal/prompt"
	"github.com/mickume/dndig/internal/style"
	"github.com/mickume/dndig/internal/workspace"
)

// Plan is an assembled request plus the provenance that goes into the
// take sidecar.
type Plan struct {
	Prompt   *prompt.Prompt
	Request  gemini.Request
	Spec     gemini.Spec
	Style    *style.Style
	Images   []workspace.ImageRef
	Warnings []string
	// Placeholders counts the references that stand in for a pick that
	// does not exist yet (see Options). Such a plan is for display only:
	// its request lacks those images and must never be sent.
	Placeholders int
}

// Options adjust Generate for planning a whole run ahead of time.
type Options struct {
	// Placeholders turns a dependency without a pick into a placeholder
	// reference and a warning instead of an error, so a dry run can show
	// every prompt of a run before any of them is generated.
	Placeholders bool
	// Earlier reports whether a prompt is generated earlier in the same
	// run; its placeholder then names the pick that run will make.
	Earlier func(*prompt.Prompt) bool
	// AutoPick says the run picks a first take for every prompt without
	// one, so a dependency generated earlier will have a pick in time.
	AutoPick bool
}

// ModelFor resolves which model a prompt uses: the prompt's own, else the
// project's, else the default.
func ModelFor(p *prompt.Prompt, proj *project.Project) string {
	if p != nil && p.Model != "" {
		return p.Model
	}
	if proj != nil && proj.Config.Model != "" {
		return proj.Config.Model
	}
	return gemini.DefaultModel
}

// Generate plans a fresh generation of p.
func Generate(p *prompt.Prompt, proj *project.Project) (*Plan, error) {
	return GenerateWith(p, proj, Options{})
}

// GenerateWith is Generate with options.
func GenerateWith(p *prompt.Prompt, proj *project.Project, opts Options) (*Plan, error) {
	modelID := ModelFor(p, proj)
	spec, err := gemini.Lookup(modelID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", proj.Rel(p.Path), err)
	}
	if !spec.SupportsSize(p.Resolution) {
		return nil, fmt.Errorf("%s: resolution %s is not available on %s (choose %s)", proj.Rel(p.Path), p.Resolution, spec.ID, strings.Join(spec.Sizes, ", "))
	}
	st, err := proj.ResolveStyle(p.Style, p)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", proj.Rel(p.Path), err)
	}
	plan := &Plan{Prompt: p, Spec: spec, Style: st}
	plan.Request = gemini.Request{
		Model: spec.ID, AspectRatio: p.AspectRatio, Resolution: p.Resolution,
		Temperature: p.Temperature, Seed: p.Seed, Search: p.Search,
	}
	if st != nil {
		plan.Request.System = strings.TrimSpace(st.Directive)
	}

	var lines []string
	people := 0
	add := func(path, role, sentence string) error {
		img, ref, err := loadImage(path, proj)
		if err != nil {
			return fmt.Errorf("%s: %w", proj.Rel(p.Path), err)
		}
		n := len(plan.Images) + 1
		ref.Role = role
		ref.Label = fmt.Sprintf("Image %d", n)
		plan.Request.Images = append(plan.Request.Images, img)
		plan.Images = append(plan.Images, ref)
		lines = append(lines, fmt.Sprintf("Image %d is %s", n, sentence))
		return nil
	}
	// unpicked handles a dependency with no pick: an error, or with
	// opts.Placeholders a placeholder in the image list plus a warning.
	unpicked := func(dep *prompt.Prompt, name, role, sentence, problem string) error {
		if !opts.Placeholders {
			return fmt.Errorf("%s: %s", proj.Rel(p.Path), problem)
		}
		where := fmt.Sprintf("(no pick of %s yet)", name)
		if opts.Earlier != nil && opts.Earlier(dep) {
			where = fmt.Sprintf("(pick of %s, generated earlier in this run)", name)
			if !opts.AutoPick {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s is generated earlier in this run, but only --auto-pick picks a take during a run; without it this prompt fails", dep.Title))
			}
		} else {
			plan.Warnings = append(plan.Warnings, problem)
		}
		n := len(plan.Images) + 1
		plan.Images = append(plan.Images, workspace.ImageRef{Path: where, Role: role, Label: fmt.Sprintf("Image %d", n)})
		lines = append(lines, fmt.Sprintf("Image %d is %s", n, sentence))
		plan.Placeholders++
		return nil
	}

	// Cast members first, in the order the prompt lists them, so "Image 1"
	// is the first character named.
	for _, name := range p.Cast {
		member, ok := proj.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("%s: cast member %q is not a prompt in %s", proj.Rel(p.Path), name, proj.Root)
		}
		ws := workspace.For(member.Path)
		if member.Kind == prompt.KindCharacter || member.Kind == prompt.KindOther {
			people++
		}
		if !ws.HasPick() {
			problem := fmt.Sprintf("%s has no approved image yet; generate it and run: dndig pick %s --take N", member.Title, proj.Rel(member.Path))
			if err := unpicked(member, name, "cast:"+member.Title, memberSentence(member), problem); err != nil {
				return nil, err
			}
			continue
		}
		if err := add(ws.PickPath(), "cast:"+member.Title, memberSentence(member)); err != nil {
			return nil, err
		}
		if ws.HasSheet() {
			if err := add(ws.SheetPath(), "sheet:"+member.Title, fmt.Sprintf("%s again, a turnaround sheet of the same %s from more angles.", member.Title, kindWord(member.Kind))); err != nil {
				return nil, err
			}
		}
	}
	for _, r := range p.References {
		path, sentence := r.Path, ""
		role := "reference"
		if r.Title != "" {
			target, ok := proj.Lookup(r.Title)
			if !ok {
				return nil, fmt.Errorf("%s: reference %q is neither a file nor a prompt title", proj.Rel(p.Path), r.Title)
			}
			ws := workspace.For(target.Path)
			if !ws.HasPick() {
				problem := fmt.Sprintf("%s has no approved image yet; run: dndig pick %s --take N", target.Title, proj.Rel(target.Path))
				if err := unpicked(target, r.Title, "cast:"+target.Title, memberSentence(target), problem); err != nil {
					return nil, err
				}
				continue
			}
			path = ws.PickPath()
			role = "cast:" + target.Title
			sentence = memberSentence(target)
		} else {
			as := r.As
			if as == "" {
				as = strings.TrimSuffix(filepath.Base(r.Path), filepath.Ext(r.Path))
			}
			sentence = fmt.Sprintf("a reference for %s.", as)
		}
		if err := add(path, role, sentence); err != nil {
			return nil, err
		}
	}
	if st != nil {
		for _, ref := range st.References {
			if err := add(ref, "style", "a style reference only: borrow its palette, brushwork and lighting, not its content."); err != nil {
				return nil, err
			}
		}
	}
	if n := len(plan.Images); n > spec.MaxRefs {
		return nil, fmt.Errorf("%s: %d reference images, but %s accepts at most %d", proj.Rel(p.Path), n, spec.ID, spec.MaxRefs)
	}
	if people > spec.MaxPeople {
		return nil, fmt.Errorf("%s: %d characters in the cast, but %s preserves at most %d likenesses", proj.Rel(p.Path), people, spec.ID, spec.MaxPeople)
	}
	if people > 3 {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("%d characters in one scene: likeness fidelity drops past 3; consider building the scene up in refinements", people))
	}

	plan.Request.Text = composeText(st, lines, people >= 2, p.Body)
	return plan, nil
}

// composeText renders the user text: style block, labelled image list,
// lock line, then the prompt body.
func composeText(st *style.Style, imageLines []string, lock bool, body string) string {
	var b strings.Builder
	if st != nil {
		b.WriteString("Art style: ")
		b.WriteString(strings.TrimSpace(st.Directive))
		b.WriteString("\n\n")
	}
	if len(imageLines) > 0 {
		b.WriteString("Reference images, in order:\n")
		for _, l := range imageLines {
			b.WriteString(l)
			b.WriteString("\n")
		}
		if lock {
			b.WriteString("Do not blend facial traits, swap clothing or duplicate characters; each named character appears exactly once.\n")
		}
		b.WriteString("\n")
	}
	b.WriteString(strings.TrimSpace(body))
	return b.String()
}

func memberSentence(m *prompt.Prompt) string {
	switch m.Kind {
	case prompt.KindCharacter:
		return fmt.Sprintf("%s (a character): keep the face, hair, build, clothing and signature props exactly as shown.", m.Title)
	case prompt.KindLocation:
		return fmt.Sprintf("%s (a location): keep its architecture, layout and setting as shown.", m.Title)
	case prompt.KindItem:
		return fmt.Sprintf("%s (an item): keep its shape, materials and details exactly as shown.", m.Title)
	default:
		return fmt.Sprintf("%s: keep it exactly as shown.", m.Title)
	}
}

func kindWord(k prompt.Kind) string {
	switch k {
	case prompt.KindCharacter:
		return "character"
	case prompt.KindLocation:
		return "location"
	case prompt.KindItem:
		return "item"
	default:
		return "subject"
	}
}

// Refine plans a follow-up turn on a take: the take's thread is replayed
// and the instruction is appended as the new user turn.
func Refine(p *prompt.Prompt, proj *project.Project, take workspace.Take, instruction string) (*Plan, error) {
	ws := workspace.For(p.Path)
	modelID := ModelFor(p, proj)
	spec, err := gemini.Lookup(modelID)
	if err != nil {
		return nil, err
	}
	chain, err := threadOf(ws, take)
	if err != nil {
		return nil, err
	}
	plan := &Plan{Prompt: p, Spec: spec}
	plan.Request = gemini.Request{
		Model: spec.ID, System: take.System, AspectRatio: take.AspectRatio, Resolution: take.Resolution,
		Temperature: p.Temperature, Seed: p.Seed, Search: take.Search,
	}
	if !spec.SupportsSize(take.Resolution) {
		plan.Request.Resolution = p.Resolution
	}
	for _, t := range chain {
		turn := gemini.Turn{Text: t.Text, ModelID: t.Model}
		for _, ref := range t.Images {
			path := absPath(proj, ref.Path)
			img, now, err := loadImage(path, proj)
			if err != nil {
				return nil, fmt.Errorf("take %d referenced %s: %w", t.Number, ref.Path, err)
			}
			// The thread must replay what the model actually saw. A pick
			// that changed since (a new pick, a regenerated sheet) would put
			// a different image under the same thought signatures.
			if ref.SHA256 != "" && now.SHA256 != ref.SHA256 {
				return nil, fmt.Errorf("take %d was generated with a different %s (%s has changed since); refine an older take or generate afresh", t.Number, ref.Role, ref.Path)
			}
			turn.Images = append(turn.Images, img)
		}
		for _, b := range t.Response.Blocks {
			blk := gemini.Block{Type: b.Type, Text: b.Text, Signature: b.Signature}
			if b.Type == "image" {
				// Through the same budget as a reference: a 4K take replayed
				// verbatim is over the inline request limit on its own.
				img, err := LoadImage(ws.ImagePath(b.Image))
				if err != nil {
					return nil, fmt.Errorf("take %d: %w", t.Number, err)
				}
				blk.Image = &img
			}
			turn.Model = append(turn.Model, blk)
		}
		plan.Request.History = append(plan.Request.History, turn)
	}
	text := strings.TrimSpace(instruction)
	if !strings.Contains(strings.ToLower(text), "keep") {
		text += " Keep everything else exactly the same: the same character, style, lighting and composition."
	}
	plan.Request.Text = text
	return plan, nil
}

// threadOf walks parents back to the root take and returns the chain in
// order, root first.
func threadOf(ws workspace.Workspace, take workspace.Take) ([]workspace.Take, error) {
	chain := []workspace.Take{take}
	seen := map[int]bool{take.Number: true}
	for cur := take; cur.Parent != 0; {
		if seen[cur.Parent] {
			return nil, fmt.Errorf("take %d: parent chain loops", take.Number)
		}
		parent, err := ws.Take(cur.Parent)
		if err != nil {
			return nil, err
		}
		seen[parent.Number] = true
		chain = append([]workspace.Take{parent}, chain...)
		cur = parent
	}
	return chain, nil
}

// Sheet plans a character turnaround sheet from the pick.
func Sheet(p *prompt.Prompt, proj *project.Project) (*Plan, error) {
	ws := workspace.For(p.Path)
	if !ws.HasPick() {
		return nil, fmt.Errorf("%s has no approved image yet; run: dndig pick %s --take N", p.Title, proj.Rel(p.Path))
	}
	spec, err := gemini.Lookup(ModelFor(p, proj))
	if err != nil {
		return nil, err
	}
	st, err := proj.ResolveStyle(p.Style, p)
	if err != nil {
		return nil, err
	}
	img, ref, err := loadImage(ws.PickPath(), proj)
	if err != nil {
		return nil, err
	}
	ref.Role, ref.Label = "cast:"+p.Title, "Image 1"
	res := p.Resolution
	if !spec.SupportsSize(res) {
		res = spec.Sizes[len(spec.Sizes)-1]
	}
	plan := &Plan{Prompt: p, Spec: spec, Style: st, Images: []workspace.ImageRef{ref}}
	plan.Request = gemini.Request{Model: spec.ID, AspectRatio: "16:9", Resolution: res, Images: []gemini.Image{img}}
	if st != nil {
		plan.Request.System = strings.TrimSpace(st.Directive)
	}
	body := fmt.Sprintf("A character turnaround sheet of %s, exactly the %s shown in Image 1: full-body front view, three-quarter view, side view and back view side by side, the same face, hair, build, clothing and props in every view, consistent neutral lighting, on a plain light background. No text, labels or decoration.", p.Title, kindWord(p.Kind))
	plan.Request.Text = composeText(st, []string{fmt.Sprintf("Image 1 is %s", memberSentence(p))}, false, body)
	return plan, nil
}

// Describe renders the plan for --dry-run and --verbose.
func (pl *Plan) Describe(proj *project.Project) string {
	var b strings.Builder
	r := pl.Request
	fmt.Fprintf(&b, "model:        %s (%s)\n", r.Model, pl.Spec.Name)
	fmt.Fprintf(&b, "image:        %s, %s", r.AspectRatio, r.Resolution)
	if r.Temperature != nil {
		fmt.Fprintf(&b, ", temperature %g", *r.Temperature)
	}
	if r.Seed != nil {
		fmt.Fprintf(&b, ", seed %d", *r.Seed)
	}
	if r.Search {
		b.WriteString(", search grounding")
	}
	b.WriteString("\n")
	if pl.Style != nil {
		fmt.Fprintf(&b, "style:        %s (%s)\n", pl.Style.Name, proj.Rel(pl.Style.Path))
	}
	if len(r.History) > 0 {
		fmt.Fprintf(&b, "thread:       %d earlier turn(s) replayed\n", len(r.History))
	}
	for _, ref := range pl.Images {
		fmt.Fprintf(&b, "%-13s %s (%s)\n", ref.Label+":", ref.Path, ref.Role)
	}
	for _, w := range pl.Warnings {
		fmt.Fprintf(&b, "warning:      %s\n", w)
	}
	if r.System != "" {
		fmt.Fprintf(&b, "\n--- system instruction ---\n%s\n", r.System)
	}
	fmt.Fprintf(&b, "\n--- prompt ---\n%s\n", r.Text)
	return b.String()
}

// loadImage is LoadImage plus the sidecar provenance.
func loadImage(path string, proj *project.Project) (gemini.Image, workspace.ImageRef, error) {
	img, err := LoadImage(path)
	if err != nil {
		return gemini.Image{}, workspace.ImageRef{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return gemini.Image{}, workspace.ImageRef{}, err
	}
	return img, workspace.ImageRef{Path: proj.Rel(path), SHA256: workspace.SHA256(data)}, nil
}

// LoadImage reads an image file, checks it is an image by its bytes, and
// fits it to the provider's inline budget (a 4K pick is downscaled to the
// kit's 2000px limit; anything that already fits is sent verbatim).
func LoadImage(path string) (gemini.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return gemini.Image{}, err
	}
	mime, ok := imagex.Sniff(data)
	if !ok {
		return gemini.Image{}, fmt.Errorf("%s is not a PNG, JPEG, GIF or WebP image", path)
	}
	if mime == imagex.MIMEWebP {
		// The kit cannot decode WebP; send it as-is if it fits the budget.
		if base64.StdEncoding.EncodedLen(len(data)) > imagex.MaxBase64Bytes {
			return gemini.Image{}, fmt.Errorf("%s is too large to send inline and WebP cannot be downscaled; convert it to PNG or JPEG", path)
		}
		return gemini.Image{Data: data, Mime: mime}, nil
	}
	res, err := imagex.Normalize(data, mime)
	if err != nil {
		return gemini.Image{}, fmt.Errorf("%s: %w", path, err)
	}
	return gemini.Image{Data: res.Data, Mime: res.MIMEType}, nil
}

func absPath(proj *project.Project, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(proj.Root, filepath.FromSlash(p))
}
