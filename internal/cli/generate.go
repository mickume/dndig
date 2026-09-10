package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mickume/dndig/internal/assemble"
	"github.com/mickume/dndig/internal/gemini"
	"github.com/mickume/dndig/internal/project"
	"github.com/mickume/dndig/internal/prompt"
	"github.com/mickume/dndig/internal/workspace"
)

const defaultWorkers = 2

func (a *app) generate(args []string) error {
	fs := a.flags("generate", "generate [flags] <prompt.md|dir>...")
	takes := fs.Int("takes", 0, "candidates per prompt (overrides the prompt's takes:)")
	workers := fs.Int("workers", 0, "concurrent requests (default: dndig.yaml workers, else 2)")
	model := fs.String("model", "", "image model (overrides prompt and dndig.yaml)")
	dryRun := fs.Bool("dry-run", false, "print what would be sent and stop")
	autoPick := fs.Bool("auto-pick", false, "pick the first take of a prompt that has no pick yet (lets a directory run feed its own scenes)")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return usagef("generate needs at least one prompt file or directory")
	}
	if *takes < 0 || *takes > prompt.MaxTakes {
		return usagef("--takes must be 1 to %d", prompt.MaxTakes)
	}
	if *workers < 0 || *workers > 16 {
		return usagef("--workers must be 1 to 16")
	}
	prompts, err := project.Collect(fs.Args())
	if err != nil {
		return err
	}
	proj, err := project.Find(prompts[0].Path)
	if err != nil {
		return err
	}
	if err := proj.Index(); err != nil {
		return err
	}
	for _, p := range prompts {
		if *model != "" {
			p.Model = *model
		}
		if *takes > 0 {
			p.Takes = *takes
		}
	}
	ordered, err := proj.Order(prompts)
	if err != nil {
		return err
	}
	if *dryRun {
		for _, p := range ordered {
			plan, err := assemble.Generate(p, proj)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "== %s (%d take(s))\n%s\n", proj.Rel(p.Path), p.Takes, plan.Describe(proj))
		}
		return nil
	}
	client := a.client()
	if err := client.CheckCredentials(); err != nil {
		return err
	}
	n := *workers
	if n == 0 {
		n = proj.Config.Workers
	}
	if n == 0 {
		n = defaultWorkers
	}
	var failed []string
	total := 0
	for i, p := range ordered {
		if len(ordered) > 1 {
			fmt.Fprintf(a.out, "[%d/%d] %s\n", i+1, len(ordered), proj.Rel(p.Path))
		} else {
			fmt.Fprintf(a.out, "%s\n", proj.Rel(p.Path))
		}
		plan, err := assemble.Generate(p, proj)
		if err != nil {
			return err
		}
		if a.verbose {
			fmt.Fprintln(a.err, plan.Describe(proj))
		}
		for _, w := range plan.Warnings {
			fmt.Fprintf(a.err, "  warning: %s\n", w)
		}
		saved, err := a.runTakes(client, proj, p, plan, p.Takes, n, 0, "")
		total += len(saved)
		if a.ctx.Err() != nil {
			return a.ctx.Err()
		}
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", proj.Rel(p.Path), err))
			if len(saved) == 0 {
				continue
			}
		}
		ws := workspace.For(p.Path)
		if *autoPick && !ws.HasPick() && len(saved) > 0 {
			pk, err := ws.PickTake(saved[0].Number)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.out, "  picked take %03d -> %s\n", pk.Take, proj.Rel(ws.PickPath()))
		}
	}
	fmt.Fprintf(a.out, "\n%d image(s) generated from %d prompt(s).\n", total, len(ordered))
	if len(failed) > 0 {
		return errors.New(strings.Join(failed, "\n"))
	}
	return nil
}

// runTakes generates count takes for a plan with bounded concurrency and
// saves each as it completes. parent and instruction are recorded on
// refinement takes; purpose marks sheet takes.
func (a *app) runTakes(client *gemini.Client, proj *project.Project, p *prompt.Prompt, plan *assemble.Plan, count, workers, parent int, instruction string) ([]workspace.Take, error) {
	return a.runTakesWithPurpose(client, proj, p, plan, count, workers, parent, instruction, "")
}

func (a *app) runTakesWithPurpose(client *gemini.Client, proj *project.Project, p *prompt.Prompt, plan *assemble.Plan, count, workers, parent int, instruction, purpose string) ([]workspace.Take, error) {
	ws := workspace.For(p.Path)
	first, err := ws.Reserve(count)
	if err != nil {
		return nil, err
	}
	if workers > count {
		workers = count
	}
	type outcome struct {
		take workspace.Take
		err  error
	}
	results := make([]outcome, count)
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	var mu sync.Mutex
	for i := 0; i < count; i++ {
		number := first + i
		wg.Add(1)
		go func(i, number int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if a.ctx.Err() != nil {
				results[i] = outcome{err: a.ctx.Err()}
				return
			}
			mu.Lock()
			a.logf("  take %03d: generating...", number)
			mu.Unlock()
			started := time.Now()
			res, err := client.Generate(a.ctx, plan.Request)
			if err != nil {
				results[i] = outcome{err: fmt.Errorf("take %03d: %w", number, err)}
				mu.Lock()
				fmt.Fprintf(a.err, "  take %03d failed: %v\n", number, err)
				mu.Unlock()
				return
			}
			take := buildTake(number, proj, p, plan, res, parent, instruction, purpose)
			saved, err := ws.Save(take, res.Image.Data, res.Image.Mime)
			if err != nil {
				results[i] = outcome{err: err}
				return
			}
			results[i] = outcome{take: saved}
			mu.Lock()
			fmt.Fprintf(a.out, "  take %03d -> %s  ($%.3f, %s)\n", number, proj.Rel(ws.TakePath(saved.Image)), res.CostUSD, time.Since(started).Round(time.Second))
			if a.verbose && res.Text != "" {
				fmt.Fprintf(a.err, "    model: %s\n", strings.ReplaceAll(res.Text, "\n", "\n           "))
			}
			mu.Unlock()
		}(i, number)
	}
	wg.Wait()
	var saved []workspace.Take
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		saved = append(saved, r.take)
	}
	return saved, errors.Join(errs...)
}

func buildTake(number int, proj *project.Project, p *prompt.Prompt, plan *assemble.Plan, res *gemini.Result, parent int, instruction, purpose string) workspace.Take {
	r := plan.Request
	t := workspace.Take{
		Number: number, Created: time.Now().UTC(), PromptFile: proj.Rel(p.Path), Title: p.Title, Kind: string(p.Kind), Purpose: purpose,
		Model: r.Model, AspectRatio: r.AspectRatio, Resolution: r.Resolution, Temperature: r.Temperature, Seed: r.Seed, Search: r.Search,
		System: r.System, Text: r.Text, Images: plan.Images, Parent: parent, Instruction: instruction,
		Response: workspace.Response{FinishReason: res.FinishReason, ResponseID: res.ResponseID, ModelVersion: res.ModelVersion},
		Usage:    workspace.Usage{InputTokens: res.InputTokens, OutputTokens: res.OutputTokens, CostUSD: res.CostUSD},
	}
	for _, b := range res.Blocks {
		t.Response.Blocks = append(t.Response.Blocks, workspace.Block{Type: b.Type, Text: b.Text, Signature: b.Signature})
	}
	return t
}

func (a *app) refine(args []string) error {
	fs := a.flags("refine", "refine [flags] <prompt.md> \"instruction\"")
	takeNo := fs.Int("take", 0, "take to continue (default: the pick, else the latest take)")
	count := fs.Int("takes", 1, "how many variants to generate")
	model := fs.String("model", "", "image model (a different model than the take's drops its thought signatures)")
	dryRun := fs.Bool("dry-run", false, "print what would be sent and stop")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return usagef("refine needs a prompt file and an instruction")
	}
	if *count < 1 || *count > prompt.MaxTakes {
		return usagef("--takes must be 1 to %d", prompt.MaxTakes)
	}
	p, proj, err := a.loadPrompt(fs.Arg(0))
	if err != nil {
		return err
	}
	if *model != "" {
		p.Model = *model
	}
	ws := workspace.For(p.Path)
	var take workspace.Take
	switch {
	case *takeNo > 0:
		take, err = ws.Take(*takeNo)
	default:
		if pk, perr := ws.ReadPick(); perr == nil {
			take, err = ws.Take(pk.Take)
		} else {
			var ok bool
			take, ok, err = ws.Latest()
			if err == nil && !ok {
				err = fmt.Errorf("%s has no takes yet; run: dndig generate %s", p.Title, proj.Rel(p.Path))
			}
		}
	}
	if err != nil {
		return err
	}
	instruction := joinRest(fs.Args()[1:])
	plan, err := assemble.Refine(p, proj, take, instruction)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s: refining take %03d\n", proj.Rel(p.Path), take.Number)
	if *dryRun {
		fmt.Fprintln(a.out, plan.Describe(proj))
		return nil
	}
	if a.verbose {
		fmt.Fprintln(a.err, plan.Describe(proj))
	}
	client := a.client()
	if err := client.CheckCredentials(); err != nil {
		return err
	}
	_, err = a.runTakes(client, proj, p, plan, *count, defaultWorkers, take.Number, instruction)
	return err
}

func (a *app) sheet(args []string) error {
	fs := a.flags("sheet", "sheet [flags] <prompt.md>")
	dryRun := fs.Bool("dry-run", false, "print what would be sent and stop")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("sheet needs exactly one prompt file")
	}
	p, proj, err := a.loadPrompt(fs.Arg(0))
	if err != nil {
		return err
	}
	plan, err := assemble.Sheet(p, proj)
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Fprintln(a.out, plan.Describe(proj))
		return nil
	}
	if a.verbose {
		fmt.Fprintln(a.err, plan.Describe(proj))
	}
	client := a.client()
	if err := client.CheckCredentials(); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s: turnaround sheet from the pick\n", proj.Rel(p.Path))
	saved, err := a.runTakesWithPurpose(client, proj, p, plan, 1, 1, 0, "", "sheet")
	if err != nil {
		return err
	}
	ws := workspace.For(p.Path)
	data, err := os.ReadFile(ws.TakePath(saved[0].Image))
	if err != nil {
		return err
	}
	if err := ws.SaveSheet(data); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "  sheet -> %s\n", proj.Rel(ws.SheetPath()))
	return nil
}

func (a *app) pick(args []string) error {
	fs := a.flags("pick", "pick <prompt.md> <take>")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return usagef("pick needs a prompt file and a take number")
	}
	p, proj, err := a.loadPrompt(fs.Arg(0))
	if err != nil {
		return err
	}
	var n int
	if _, err := fmt.Sscanf(fs.Arg(1), "%d", &n); err != nil || n < 1 {
		return usagef("take must be a positive number, got %q", fs.Arg(1))
	}
	ws := workspace.For(p.Path)
	pk, err := ws.PickTake(n)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.out, "%s: take %03d -> %s\n", p.Title, pk.Take, proj.Rel(ws.PickPath()))
	return nil
}

func (a *app) prune(args []string) error {
	fs := a.flags("prune", "prune [flags] <prompt.md|dir>...")
	remove := fs.Bool("delete", false, "delete unpicked takes instead of moving them to discards/")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return usagef("prune needs at least one prompt file or directory")
	}
	prompts, err := project.Collect(fs.Args())
	if err != nil {
		return err
	}
	proj, err := project.Find(prompts[0].Path)
	if err != nil {
		return err
	}
	for _, p := range prompts {
		ws := workspace.For(p.Path)
		done, err := ws.Prune(*remove)
		if err != nil {
			return err
		}
		verb := "moved to discards/"
		if *remove {
			verb = "deleted"
		}
		fmt.Fprintf(a.out, "%s: %d take(s) %s\n", proj.Rel(p.Path), len(done), verb)
	}
	return nil
}

// loadPrompt loads one prompt file and its indexed project.
func (a *app) loadPrompt(path string) (*prompt.Prompt, *project.Project, error) {
	p, err := prompt.Load(path)
	if err != nil {
		return nil, nil, err
	}
	proj, err := project.Find(p.Path)
	if err != nil {
		return nil, nil, err
	}
	if err := proj.Index(); err != nil {
		return nil, nil, err
	}
	return p, proj, nil
}

func (a *app) status(args []string) error {
	fs := a.flags("status", "status [dir]")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	proj, err := project.Find(dir)
	if err != nil {
		return err
	}
	if err := proj.Index(); err != nil {
		return err
	}
	if proj.HasConfig {
		fmt.Fprintf(a.out, "project %s (dndig.yaml)\n", proj.Root)
	} else {
		fmt.Fprintf(a.out, "directory %s (no dndig.yaml; run dndig init to create one)\n", proj.Root)
	}
	prompts := proj.Prompts()
	if len(prompts) == 0 {
		fmt.Fprintln(a.out, "no prompt files found")
	}
	for _, p := range prompts {
		ws := workspace.For(p.Path)
		takes, _ := ws.Takes()
		state := "no pick"
		if pk, err := ws.ReadPick(); err == nil {
			state = fmt.Sprintf("pick: take %03d", pk.Take)
		}
		if ws.HasSheet() {
			state += ", sheet"
		}
		var missing []string
		for _, dep := range p.DependsOn() {
			d, ok := proj.Lookup(dep)
			if !ok {
				missing = append(missing, dep+" (unknown)")
			} else if !workspace.For(d.Path).HasPick() {
				missing = append(missing, dep+" (no pick)")
			}
		}
		fmt.Fprintf(a.out, "%-40s %-10s %2d take(s)  %s", proj.Rel(p.Path), p.Kind, len(takes), state)
		if len(missing) > 0 {
			fmt.Fprintf(a.out, "  needs: %s", strings.Join(missing, ", "))
		}
		fmt.Fprintln(a.out)
	}
	if a.verbose {
		for _, s := range proj.Skipped {
			fmt.Fprintf(a.err, "skipped %s: %s\n", proj.Rel(s.Path), firstLine(s.Reason))
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func (a *app) initProject(args []string) error {
	fs := a.flags("init", "init [dir]")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() > 0 {
		dir = fs.Arg(0)
	}
	files := map[string]string{
		project.ConfigFile:                       "dndig.yaml",
		filepath.Join("styles", "campaign.md"):   "campaign.md",
		filepath.Join("characters", "kaelen.md"): "character.md",
		filepath.Join("scenes", "ambush.md"):     "scene.md",
	}
	for rel, tmpl := range files {
		path := filepath.Join(dir, rel)
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(a.out, "exists   %s\n", path)
			continue
		}
		data, err := templates.ReadFile("templates/" + tmpl)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(a.out, "created  %s\n", path)
	}
	if err := os.MkdirAll(filepath.Join(dir, "refs"), 0o755); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "\nNext: export GEMINI_API_KEY=..., then\n  dndig generate %s\n  dndig pick %s <take>\n  dndig generate %s\n",
		filepath.Join(dir, "characters", "kaelen.md"), filepath.Join(dir, "characters", "kaelen.md"), filepath.Join(dir, "scenes", "ambush.md"))
	return nil
}
