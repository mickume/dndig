package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickume/dndig/internal/assemble"
	"github.com/mickume/dndig/internal/gemini"
	"github.com/mickume/dndig/internal/project"
	"github.com/mickume/dndig/internal/style"
	"github.com/mickume/dndig/internal/stylederive"
)

//go:embed templates/*
var templates embed.FS

func (a *app) style(args []string) error {
	if len(args) == 0 {
		return usagef("style needs a subcommand: derive or show")
	}
	switch args[0] {
	case "derive":
		return a.styleDerive(args[1:])
	case "show":
		return a.styleShow(args[1:])
	default:
		return usagef("unknown style subcommand %q (derive, show)", args[0])
	}
}

func (a *app) styleDerive(args []string) error {
	fs := a.flags("style derive", "style derive [flags] <out.md> <image>...")
	model := fs.String("model", "", "vision model (default: dndig.yaml vision_model, else "+gemini.DefaultVisionModel+")")
	name := fs.String("name", "", "style name (default: the output file's stem)")
	hint := fs.String("hint", "", "context for the analysis, e.g. \"grim northern campaign\"")
	force := fs.Bool("force", false, "overwrite an existing style file")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return usagef("style derive needs an output file and at least one example image")
	}
	out, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		return err
	}
	if _, err := os.Stat(out); err == nil && !*force {
		return fmt.Errorf("%s exists; pass --force to overwrite", out)
	}
	proj, _ := project.Find(filepath.Dir(out))
	if proj == nil {
		proj = &project.Project{Root: filepath.Dir(out)}
	}
	var examples []gemini.Image
	var paths []string
	for _, arg := range fs.Args()[1:] {
		matches, _ := filepath.Glob(arg)
		if len(matches) == 0 {
			matches = []string{arg}
		}
		for _, m := range matches {
			img, err := assemble.LoadImage(m)
			if err != nil {
				return err
			}
			abs, _ := filepath.Abs(m)
			examples = append(examples, img)
			paths = append(paths, abs)
		}
	}
	modelID := *model
	if modelID == "" {
		modelID = proj.Config.VisionModel
	}
	if modelID == "" {
		modelID = gemini.DefaultVisionModel
	}
	client := a.client()
	if err := client.CheckCredentials(); err != nil {
		return err
	}
	deps, err := stylederive.DepsFor(client, modelID)
	if err != nil {
		return err
	}
	if a.debug {
		deps.Warnf = func(format string, args ...any) { fmt.Fprintf(a.err, "  "+format+"\n", args...) }
	}
	fmt.Fprintf(a.out, "deriving a style from %d image(s) with %s...\n", len(examples), modelID)
	derived, usage, err := stylederive.Derive(a.ctx, deps, examples, *hint)
	if err != nil {
		return err
	}
	styleName := *name
	if styleName == "" {
		styleName = strings.TrimSuffix(filepath.Base(out), filepath.Ext(out))
	}
	text := style.Render(styleName, derived, paths, modelID, time.Now(), filepath.Dir(out))
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(a.out, "wrote %s  (%s; $%.4f)\n\n%s\n", proj.Rel(out), derived.Summary, usage.CostUSD, derived.Directive)
	return nil
}

func (a *app) styleShow(args []string) error {
	fs := a.flags("style show", "style show <name|path> [--from prompt.md]")
	from := fs.String("from", "", "resolve the name relative to this prompt file's project")
	if err := a.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return usagef("style show needs a style name or path")
	}
	start := "."
	if *from != "" {
		start = *from
	}
	proj, err := project.Find(start)
	if err != nil {
		return err
	}
	st, err := proj.ResolveStyle(fs.Arg(0), nil)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no style named %q", fs.Arg(0))
	}
	fmt.Fprintf(a.out, "style %s (%s)\n", st.Name, proj.Rel(st.Path))
	for _, r := range st.References {
		fmt.Fprintf(a.out, "reference: %s\n", proj.Rel(r))
	}
	fmt.Fprintf(a.out, "\n%s\n", st.Directive)
	return nil
}
