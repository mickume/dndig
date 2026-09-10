// Package cli implements the dndig command line.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"net/http"

	"github.com/mickume/dndig/internal/gemini"
)

// testTransport and testEnv let the test suite run the real command paths
// against a scripted HTTP transport with no key in the environment.
var (
	testTransport http.RoundTripper
	testEnv       map[string]string
)

// version is set by the linker (see the Makefile).
var version = "dev"

const usage = `dndig — D&D campaign illustrations with Gemini, with continuity.

Usage:
  dndig generate <prompt.md|dir>...   generate takes for prompt files
  dndig refine   <prompt.md> "..."    continue a take with an edit instruction
  dndig sheet    <prompt.md>          make a turnaround sheet from the pick
  dndig pick     <prompt.md> <take>   approve a take as the reference image
  dndig prune    <prompt.md|dir>...   move unpicked takes to discards/
  dndig status   [dir]                prompts, takes, picks and missing casts
  dndig style    derive|show ...      derive a style from examples, or show one
  dndig init     [dir]                scaffold a project
  dndig version

Run "dndig <command> -h" for the command's flags. Flags may appear anywhere
after the command. Global flags: -v/--verbose, --debug, --api-key KEY.
Credentials: GEMINI_API_KEY (or GOOGLE_API_KEY).
`

// Main runs the CLI and returns the exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app := &app{ctx: ctx, out: stdout, err: stderr}
	cmd, rest := args[0], args[1:]
	var err error
	switch cmd {
	case "generate", "gen":
		err = app.generate(rest)
	case "refine":
		err = app.refine(rest)
	case "sheet":
		err = app.sheet(rest)
	case "pick":
		err = app.pick(rest)
	case "prune":
		err = app.prune(rest)
	case "status", "st":
		err = app.status(rest)
	case "style":
		err = app.style(rest)
	case "init":
		err = app.initProject(rest)
	case "version", "--version":
		fmt.Fprintf(stdout, "dndig %s\n", version)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "dndig: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, errUsage):
		fmt.Fprintf(stderr, "dndig: %v\n", err)
		return 2
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(stderr, "\ndndig: interrupted")
		return 130
	default:
		fmt.Fprintf(stderr, "dndig: %v\n", err)
		return 1
	}
}

var errUsage = errors.New("usage")

func usagef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, args...))
}

type app struct {
	ctx context.Context
	out io.Writer
	err io.Writer

	verbose bool
	debug   bool
	apiKey  string
}

// flags builds a FlagSet with the global flags attached.
func (a *app) flags(name, synopsis string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(a.err)
	fs.BoolVar(&a.verbose, "verbose", false, "print the assembled request and progress")
	fs.BoolVar(&a.verbose, "v", false, "shorthand for --verbose")
	fs.BoolVar(&a.debug, "debug", false, "print provider warnings and details")
	fs.StringVar(&a.apiKey, "api-key", "", "Google API key (overrides GEMINI_API_KEY)")
	fs.Usage = func() {
		fmt.Fprintf(a.err, "Usage: dndig %s\n\n", synopsis)
		fs.PrintDefaults()
	}
	return fs
}

// parse lets flags appear anywhere on the line ("dndig pick kaelen.md 2
// -v" and "dndig generate -v kaelen.md" both work): flags are hoisted in
// front of the positionals before the FlagSet sees them. A flag that takes
// a value consumes the next token unless it was written as --name=value.
func (a *app) parse(fs *flag.FlagSet, args []string) error {
	var flags, positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		f := fs.Lookup(name)
		if f == nil {
			continue // the FlagSet reports the unknown flag
		}
		if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && b.IsBoolFlag() {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return fs.Parse(append(flags, positionals...))
}

func (a *app) client() *gemini.Client {
	opts := gemini.Options{APIKey: a.apiKey, Transport: testTransport, Env: testEnv}
	if a.debug {
		opts.Warnf = func(format string, args ...any) { fmt.Fprintf(a.err, "  warning: "+format+"\n", args...) }
	}
	return gemini.New(opts)
}

func (a *app) logf(format string, args ...any) {
	if a.verbose || a.debug {
		fmt.Fprintf(a.err, format+"\n", args...)
	}
}

func joinRest(args []string) string { return strings.TrimSpace(strings.Join(args, " ")) }
