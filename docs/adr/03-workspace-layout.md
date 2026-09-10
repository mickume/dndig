# ADR 03 — One workspace per prompt file

## Context

1.2.2 wrote every image into a flat `artwork/`, renamed existing files with
a timestamp, and had no notion of approved vs. rejected output.

## Decision

Every prompt file `<dir>/<stem>.md` owns `<dir>/<stem>/`:

- `takes/NNN.png` + `NNN.json` — every candidate, append-only, monotonic
  numbering, full provenance in the sidecar (assembled prompt, system
  instruction, references with hashes, cast, settings, seed, model,
  usage, cost, finish reason, parent take, thread).
- `<stem>.png` + `<stem>.json` — the pick; a stable name other prompts and
  documents can rely on.
- `sheet.png` — optional turnaround.
- `discards/` — where `prune` moves unpicked takes.

User-supplied images live wherever the user wants (`refs/` by convention)
and are referenced by path relative to the prompt file.

## Consequences

- No output directory flag: the location follows the prompt, so a scene's
  results sit next to the scene. `--out` is deliberately absent.
- `git`-friendly: a project can ignore `**/takes/` and `**/discards/` and
  commit picks.
