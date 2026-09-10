# PRD 01 — Rebuild dndig in Go on AgentKit

Status: accepted 2026-09-10. Supersedes the Python implementation (1.2.2),
archived on branch `archive/python-v1.2.2`.

## 1. Problem

dndig generates D&D campaign illustrations from Markdown prompt files with
Google Gemini. What it does is right; how it supports a campaign is not:

1. **Continuity is manual.** A character is iterated 3+ times; the approved
   image must then appear unchanged in scenes with other characters and
   NPCs. Today that means copying files by hand and hoping the next run does
   not replace them.
2. **No iteration model.** Every run overwrites or renames; nothing records
   which take was approved, and "refine the last one" cannot be expressed.
3. **Output is one flat directory** mixing reference images, picks and
   rejects, with no provenance.
4. **Style is an unstructured text file** that cannot be derived from
   examples.
5. **The model id it targets (`gemini-3-pro-image-preview`) was shut down on
   2026-06-25**, so the tool no longer works at all.

## 2. Goals

- G1. Same core workflow: Markdown prompt + frontmatter → images, single
  file or directory, with dependency ordering.
- G2. First-class **continuity**: an approved take becomes the entity's
  reference; scenes name a `cast` and get those references, labelled, in
  the request; refinement continues a thread with the previous image and
  its thought signatures in context.
- G3. A deliberate **workspace layout**: takes (all candidates), the pick
  (approved), discards, and external reference images each have one place,
  and every generated image has a JSON sidecar with full provenance.
- G4. **Style directives** as structured files, reusable across prompts, and
  a mode that **derives** one from example images.
- G5. Google only. Built on the AgentKit (`agent-fox-dev/coder`) Google
  provider; no vendor SDK.
- G6. Canonical Go: `cmd/dndig` + `internal/...`, stdlib only besides the
  kit, `make check` green, offline test suite.

Non-goals: other providers; a GUI; the Interactions API (the kit speaks
`generateContent`, which remains fully supported; local threads do the job of
`previous_interaction_id` without a 55-day expiry); Imagen.

## 3. Research summary (see `docs/research/`)

- `seed` is accepted by Gemini image models but Google states determinism
  is best-effort and no image doc lists it as a technique. Even a perfect
  seed only reproduces the same request; it cannot carry a character into a
  different scene. **Continuity is done with reference images and prompt
  labelling, not seeds.** dndig records the seed it sent for the record.
- Official techniques: up to 14 reference images on `gemini-3-pro-image`
  (≤6 objects with high fidelity, ≤5 humans); prompt formula
  `[reference images] + [relationship instruction] + [new scenario]`; label
  each image by role and name the character; multi-turn editing with the
  previous output and all thought signatures replayed; character sheets
  (front/side/back) generated from the approved portrait.
- `candidateCount` > 1 is rejected: N takes = N requests. Output is PNG.
- Style: describe medium, brushwork, palette, lighting, colour grade; a style
  reference image labelled "style only" works; `systemInstruction` is not
  documented for image models, so the style block also goes into the prompt.

## 4. Tech stack

- Go 1.26.5, module `github.com/mickume/dndig`.
- `github.com/agentfox/agentkit-go` via a `replace` to
  `github.com/agent-fox-dev/coder@<commit>` (ADR 01): `catalog` for model
  resolution (sibling clone of the Google row, cost overridden),
  `provider/google` for the wire, `core` for messages/requests,
  `imagex` for image sniffing and size budgeting, `agentkit.Agent` +
  `schema` for the style-derivation agent, `provider/faux` for tests.
- No other dependencies. Frontmatter and `dndig.yaml` use a small YAML
  subset parser (scalars, quoted strings, flow and block lists, comments).

## 5. Workspace layout

```
<project>/                     # root = nearest ancestor with dndig.yaml, else the prompt's dir
  dndig.yaml                   # optional: model, style, workers, output defaults
  styles/
    campaign.md                # style directive (frontmatter + directive text)
  characters/
    kaelen.md                  # prompt file (kind: character)
    kaelen/                    # workspace, created by dndig, named after the prompt file stem
      takes/
        001.png  001.json      # every candidate, immutable, with sidecar
        002.png  002.json      # a refinement records parent: 1 and the thread
      kaelen.png               # the PICK — the continuity reference for `cast: [kaelen]`
      kaelen.json              # pick provenance (which take, when)
      sheet.png  sheet.json    # optional character sheet, also used as reference
      discards/                # unpicked takes after `dndig prune`
  scenes/
    ambush.md                  # cast: [kaelen, borin]; references: [../refs/bridge.jpg]
    ambush/takes/...  ambush/ambush.png
  refs/                        # user-supplied images (places, props, style examples)
```

Rules: nothing under `takes/` is ever overwritten; take numbers are
monotonic per prompt; `pick` copies (never moves) a take; `prune` moves
unpicked takes to `discards/` (`--delete` removes them); the pick file name is
stable so documents can link to it.

## 6. Prompt file format

```markdown
---
title: kaelen                  # default: file stem; must be unique in the project
kind: character                # character | scene | location | item | other
style: campaign                # styles/<name>.md, or a path
aspect_ratio: "2:3"            # 14 supported ratios
resolution: 2K                 # 512 | 1K | 2K | 4K (model-dependent)
model: gemini-3-pro-image      # optional override
takes: 3                       # candidates per run (was `batch`)
temperature: 0.8               # optional; sent only when set
seed: 42                       # optional; recorded, best-effort
cast: [borin, kaelen]          # entities whose picks become labelled references
references: [../refs/bridge.jpg] # ad-hoc images (objects/places), relative to the file
search: false                  # googleSearch grounding
---
Kaelen is a wood-elf ranger ... (the prompt; for characters, the character bible)
```

`instructions:` and `batch:` are accepted as aliases of `style:` and
`takes:` for 1.2.2 files. A prompt's `cast` and `references` naming another
prompt's title create a dependency; directory runs are topologically ordered
and a missing pick is an error that names the command to run.

## 7. Request assembly (the continuity model)

For every generation the request is built by one pure function
(`internal/assemble`) and can be printed with `--dry-run`:

1. `systemInstruction` = style directive text (if any).
2. User parts, in order: **prompt preamble**, then reference images, then
   the scenario.
   - Preamble opens with the style block again ("Art style: …") because
     systemInstruction is undocumented for image models.
   - For each cast member, in cast order: `Image N is <Name> (<kind>): keep
     face, hair, build, clothing and signature props exactly as shown.` A
     member with a sheet contributes two images (`Image N`, `Image N+1`,
     "the same character, turnaround sheet").
   - For each ad-hoc reference: `Image N is a reference for <basename>`
     (or the caption from `references: [{path, as: "the bridge"}]`).
   - Style references from the style file: `Image N is a style reference
     only: borrow palette, brushwork and lighting, not its content.`
   - A lock line for scenes with ≥2 characters: "Do not blend facial traits,
     swap clothing or duplicate characters."
   - Then the prompt body.
3. `generationConfig`: `responseModalities: [IMAGE, TEXT]`,
   `imageConfig{aspectRatio, imageSize}`, `seed` and `temperature` only when
   set; `tools: [{googleSearch:{}}]` only when `search: true`.
4. Limits enforced before the call: ≤14 images, ≤5 cast members, the
   model's resolution set, per-image base64 budget (downscale via `imagex`
   only when a reference exceeds it).

**Refinement** (`dndig refine`): history = original user turn (as assembled
and recorded in the take's sidecar) → model turn (the take's image, its text
and thought signatures) → new user turn with the instruction and the standing
rule "keep everything else exactly the same". Chains of refinements replay
the whole chain. The new take records `parent` and the thread.

**Sheets** (`dndig sheet`): one request with the pick as reference asking
for a front/side/back turnaround on neutral background, saved as
`sheet.png`; scenes then send both.

## 8. Style derivation

`dndig style derive styles/mine.md refs/a.png refs/b.png …` runs an
AgentKit agent on a vision model (default `gemini-3.1-pro-preview`,
override `--model`) with the example images in the first user message and
exactly one tool, `submit_style`, whose schema requires: `summary`,
`medium`, `brushwork`, `palette` (list), `lighting`, `composition`,
`texture`, `mood`, `avoid` (list), `directive` (the paragraph to send to the
image model). The tool terminates the run; the result is rendered into the
style file (frontmatter: `name`, `derived_from`, `model`, `created`; body =
directive; a `## Notes` section with the fields). A style file may list
`references:` images to send as style references.

## 9. CLI

```
dndig generate <prompt.md|dir>... [--takes N] [--workers N] [--model ID] [--dry-run]
dndig refine   <prompt.md> [--take N] "instruction"
dndig sheet    <prompt.md>
dndig pick     <prompt.md> <take>
dndig prune    <prompt.md|dir>... [--delete]
dndig status   [dir]                       # prompts, takes, picks, missing casts
dndig style derive <out.md> <image>... [--model ID] [--name NAME]
dndig init     [dir]                       # dndig.yaml, styles/, example prompt
dndig version
```
Global flags: `--verbose`, `--debug`, `--api-key`. Credentials:
`GEMINI_API_KEY` / `GOOGLE_API_KEY` (kit order). Exit codes: 0 ok, 1 error,
2 usage, 130 interrupted.

## 10. Testing

- Unit: frontmatter, config, workspace numbering/pick/prune, ordering,
  assembly (golden preambles), sidecars.
- Provider: `internal/gemini` against a fake `http.RoundTripper` — asserts
  the exact JSON payload (golden) and decodes SSE with thought parts,
  signatures and a final image; error paths (`IMAGE_SAFETY`, no image).
- Style derive: `provider/faux` scripting the `submit_style` call.
- CLI: `--dry-run` end-to-end on a fixture project in `testdata/`.
- No test needs a key or network.

## 11. Delivery

Branch `claude/dndig-golang-rebuild-xdpvgt` (the rebuild), conventional
commits, `make check` green. `main` is left untouched for the user to merge.
