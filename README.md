# dndig

Campaign illustrations for your D&D table, generated with Google's Gemini
image models, with the thing a campaign actually needs: **continuity**. A
character you approve once looks the same in every later scene, next to
the other characters and NPCs you approved.

dndig is a single Go binary built on the
[AgentKit](https://github.com/agent-fox-dev/coder) Google provider. It
speaks to `gemini-3-pro-image` (Nano Banana Pro) by default and needs
nothing but a [Gemini API key](https://aistudio.google.com/apikey).

## Install

```bash
git clone https://github.com/mickume/dndig.git
cd dndig
make install            # builds ./cmd/dndig into ~/go/bin/dndig
export GEMINI_API_KEY=...
```

`go install github.com/mickume/dndig/cmd/dndig@latest` does **not** work,
because the AgentKit is consumed through a `replace` directive
(see [docs/adr/01](docs/adr/01-consume-agentkit-through-a-replace-directive.md)).

## Five-minute tour

```bash
dndig init mycampaign && cd mycampaign
dndig generate characters/kaelen.md          # 3 takes → characters/kaelen/takes/001..003.png
dndig refine characters/kaelen.md "make the scar longer, hair darker"
dndig pick characters/kaelen.md 4            # take 4 becomes characters/kaelen/kaelen.png
dndig sheet characters/kaelen.md             # optional front/side/back turnaround sheet
dndig generate scenes/ambush.md              # cast: [kaelen] → her pick (and sheet) ride along
dndig status                                 # what has takes, picks, missing casts
dndig prune characters/kaelen.md             # unpicked takes → discards/
```

Everything a run sends can be inspected first with `--dry-run`, and every
generated image gets a JSON sidecar recording exactly what was sent.

## How continuity works

Google's own guidance (see [docs/research](docs/research/)) is that
consistency comes from **reference images with explicit roles** and from
**editing in a conversation** where the previous image stays in context —
not from reusing a `seed`, which Gemini treats as best-effort only. dndig
builds both in:

- **Picks are references.** `dndig pick` copies the approved take to a
  stable path, `characters/kaelen/kaelen.png`. A scene that lists
  `cast: [kaelen, borin]` sends both picks (and turnaround sheets when
  present) as the first images of the request, and the prompt names them:

  > Image 1 is kaelen (a character): keep the face, hair, build, clothing
  > and signature props exactly as shown. Image 2 is borin (a character):
  > … Do not blend facial traits, swap clothing or duplicate characters.

- **Refinement is a thread.** `dndig refine` replays the take's original
  request, the model's answer (image, text and thought signatures, in
  order) and your instruction, so the model *edits* rather than starts
  over. Refining a refinement replays the whole chain. The sidecar records
  `parent` and `instruction`.

- **Sheets** (`dndig sheet`) ask for a front/three-quarter/side/back
  turnaround of the pick, which travels with the pick into scenes.

- **Seeds** are sent when you set `seed:` and always recorded, with no
  promise of determinism.

Limits from the docs are enforced before a request goes out: at most 14
images, at most 5 characters in a cast (with a warning above 3, where
fidelity visibly drops), and only the resolutions the model supports.

## Workspace layout

```
mycampaign/
  dndig.yaml                 # optional project config (model, style, workers)
  styles/campaign.md         # style directive
  refs/                      # your own images (places, props, style examples)
  characters/kaelen.md       # a prompt file
  characters/kaelen/         # its workspace, created by dndig
    takes/001.png 001.json   # every candidate, never overwritten, with provenance
    kaelen.png  kaelen.json  # the pick — the reference other prompts use
    sheet.png                # optional turnaround sheet
    discards/                # unpicked takes after `dndig prune`
  scenes/ambush.md
  scenes/ambush/…
```

The root is the nearest ancestor with `dndig.yaml`; without one it is the
prompt's own directory. Prompt titles must be unique within a project.
A `.gitignore` with `**/takes/` and `**/discards/` keeps picks in git and
candidates out.

## Prompt files

```markdown
---
title: kaelen                 # default: file stem; the name used in cast:
kind: character               # character | scene | location | item | other
style: campaign               # styles/campaign.md, or a path relative to this file
aspect_ratio: "2:3"           # 1:1 2:3 3:2 3:4 4:3 4:5 5:4 9:16 16:9 21:9 1:4 4:1 1:8 8:1
resolution: 2K                # 512 (3.1 Flash only) | 1K | 2K | 4K
model: gemini-3-pro-image     # optional override
takes: 3                      # candidates per run, 1-8
temperature: 0.8              # optional; sent only when set
seed: 42                      # optional; recorded, best-effort
cast: [borin, kaelen]         # prompts whose picks become labelled references
references: [../refs/bridge.jpg, {path: ../refs/sword.png, as: "Kaelen's sword"}, tower]
search: false                 # Google Search grounding
---
The prompt. For a character, this is the character bible: build, hair,
scars, outfit, signature props. Keep facial description modest once a
reference image exists; text and image otherwise fight.
```

`references:` takes files (relative to the prompt file) or another
prompt's title, which resolves to that prompt's pick. `instructions:` and
`batch:` from dndig 1.x are accepted as aliases of `style:` and `takes:`.
Directory runs (`dndig generate scenes/`) process every prompt file in the
directory, dependencies first; `--auto-pick` picks the first take of any
prompt that has none, so a fresh directory can run end to end.

## Style directives

A style is `styles/<name>.md`: an optional frontmatter (`name`,
`references:` style images sent as "style reference only") and a body that
is the directive. It is sent as the system instruction **and** as the
first paragraph of every prompt, because image models are not documented
to honour system instructions.

Derive one from example images:

```bash
dndig style derive styles/grim.md refs/examples/*.jpg --hint "grim northern campaign"
```

A vision model (`gemini-3.1-pro-preview` by default; `vision_model:` in
`dndig.yaml` or `--model`) describes medium, brushwork, palette, lighting,
composition, texture, mood and what to avoid through a schema-checked
tool call, and the result is written as an editable style file.

## Commands

| Command | Does |
|---|---|
| `generate <prompt.md\|dir>... [--takes N] [--workers N] [--model ID] [--dry-run] [--auto-pick]` | generate candidates |
| `refine <prompt.md> "instruction" [--take N] [--takes N]` | continue a take (default: the pick, else the latest) |
| `sheet <prompt.md>` | turnaround sheet from the pick |
| `pick <prompt.md> <take>` | approve a take |
| `prune <prompt.md\|dir>... [--delete]` | move (or delete) unpicked takes |
| `status [dir]` | prompts, takes, picks, missing casts |
| `style derive <out.md> <image>... [--model ID] [--name N] [--hint ...]` | derive a style |
| `style show <name\|path>` | print a style |
| `init [dir]` | scaffold a project |

Global flags: `-v/--verbose` (prints the assembled request and progress),
`--debug`, `--api-key KEY`. Credentials: `GEMINI_API_KEY` (also
`GOOGLE_API_KEY`, `GOOGLE_GENERATIVE_AI_API_KEY`). Exit codes: 0, 1
error, 2 usage, 130 interrupted.

## Models

| Model | Sizes | References | Notes |
|---|---|---|---|
| `gemini-3-pro-image` (default) | 1K 2K 4K | 14 (≤5 people) | highest quality |
| `gemini-3.1-flash-image` | 512 1K 2K 4K | 14 (≤5 people) | fast drafts |
| `gemini-3.1-flash-lite-image` | 1K | 3 | cheapest |

Retired ids (`gemini-3-pro-image-preview`, `gemini-2.5-flash-image`) are
refused with the replacement named. Prices in the sidecars follow the
published per-token rates (`docs/research/01_gemini_image_api.md`).

## Development

```bash
make check     # fmt, vet, lint (golangci-lint if installed), test
make test      # offline: every model call is scripted
```

Plan and decisions: [docs/prd/01](docs/prd/01-rebuild-dndig-in-go.md),
[docs/adr/](docs/adr/), research notes in [docs/research/](docs/research/).
The Python 1.x implementation lives on the `archive/python-v1.2.2` branch.

## License

MIT — see [LICENSE](LICENSE).
