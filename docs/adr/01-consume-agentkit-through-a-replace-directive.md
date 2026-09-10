# ADR 01 — Consume AgentKit through a `replace` directive

## Context

The AgentKit's module path is `github.com/agentfox/agentkit-go`, but the
code lives at `github.com/agent-fox-dev/coder`. `go get` of the module path
fails (no such repository) and `go get` of the repository path fails
(module declares a different path). The kit's own nested modules use a
`replace` to a relative checkout, which would force every dndig user to
clone the kit next to dndig.

## Decision

`go.mod` requires `github.com/agentfox/agentkit-go v0.0.0` and replaces it
with `github.com/agent-fox-dev/coder <pseudo-version>`. Go accepts a
replacement whose `go.mod` declares the *original* path (the fork pattern),
so `go build` and `go test` work from a plain clone with no sibling
checkout; verified with Go 1.26.5.

## Consequences

- `go install github.com/mickume/dndig/cmd/dndig@latest` does not work
  (`go install pkg@version` ignores `replace`). Installation is
  `git clone && make install`. Documented in the README.
- Bumping the kit is `go mod edit -replace …@<new pseudo-version>`;
  `make kit-bump` does it.
- If the kit is ever published under its declared path the `replace` is
  deleted and nothing else changes.
