# Agent Instructions

Instructions for coding agents working on this repository.

## Understand before you code

1. Read `README.md`.
2. Read `docs/prd/01-rebuild-dndig-in-go.md` (the plan this code implements),
   the ADRs in `docs/adr/` and the research notes in `docs/research/`.
3. `cmd/dndig` is the entry point; `internal/` holds the packages, each
   with its tests beside it. Tests build their fixture projects in
   temporary directories; there is no checked-in campaign.
4. Check git state: `git log --oneline -20`, `git status --short --branch`.

Read documents and code in depth; only read files tracked by git.

## Stack

- Go 1.26.5, standard library plus the AgentKit
  (`github.com/agentfox/agentkit-go`, consumed through a `replace` to
  `github.com/agent-fox-dev/coder`; see `docs/adr/01`). No other
  dependencies: adding one needs an ADR.
- Google Gemini image models only, through the kit's `provider/google`.

## Quality commands

| Command | What it does |
|---|---|
| `make check` | fmt + vet + lint + test — run before committing |
| `make test` | `go test ./...` — offline, no API key |
| `make build` | `./bin/dndig` |

Every test must run without a key or network: model calls go through a fake
`http.RoundTripper` or `provider/faux`.

## Conventions

- Conventional commits (`feat:`, `fix:`, `docs:`, `test:`, `refactor:`,
  `chore:`). No AI attribution lines.
- One coherent change per session; fix broken behaviour before adding new
  behaviour.
- User-facing behaviour, prompt-file fields and CLI flags are documented in
  `README.md` in the same change. Spec divergences go to `docs/errata/`.
- PRDs: `docs/prd/NN-imperative-verb-phrase.md`; ADRs:
  `docs/adr/NN-imperative-verb-phrase.md`; next NN = max + 1, zero-padded.
