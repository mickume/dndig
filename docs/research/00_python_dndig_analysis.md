# Analysis of dndig 1.2.2 (Python)

## What it does

- One command: `dndig <prompt.md|dir>... [-o DIR] [-w N] [--summary]`.
- Prompt file = Markdown with a hand-parsed YAML-ish frontmatter
  (`title, aspect_ratio, resolution, temperature, batch, instructions,
  references`) and the prompt body.
- `instructions` -> a text file sent as `systemInstruction`.
- `references` -> up to 14 images sent as `inlineData` parts after the text.
- Model is hard-coded: `gemini-3-pro-image-preview`, `responseModalities:
  [IMAGE, TEXT]`, `imageConfig{imageSize, aspectRatio}`, `tools: googleSearch`.
- `batch` N -> N parallel streaming calls (ThreadPoolExecutor), each saving
  the first inline image it sees; a worker that gets no image is retried
  until N images exist. No seed, no candidateCount.
- Output: `artwork/<title>.jpeg` for batch 1 (an existing file is renamed
  with its mtime), `artwork/<title>_<ts>_<i>.jpeg` for batches. Optional
  `<title>_<ts>_metadata.json`.
- Directories: all `*.md` with frontmatter, topologically ordered so that a
  prompt whose `references` name another prompt's `title` runs after it.

## What is worth keeping

- The prompt-file idea (Markdown + frontmatter) and its field vocabulary.
- Path resolution relative to the prompt file.
- Reference images as the continuity primitive.
- Dependency ordering between prompts (a scene depends on its characters).
- Renaming instead of overwriting.

## What is weak

1. **Continuity is manual and lossy.** A character prompt produces
   `artwork/<title>.jpeg`; to reuse the character you must hand-copy the
   file next to the scene prompt and list it under `references`. Nothing
   records *which* iteration was approved, and a re-run of the character
   prompt silently replaces the approved image (renamed with a timestamp,
   but the scene now points at the new one).
2. **No iteration model.** "3+ iterations until happy" is 3+ runs that
   overwrite/rename each other; there is no notion of candidates, picks and
   discards, and no way to say "refine the last one" with the previous
   image in context.
3. **Output layout mixes everything.** References, picks, rejects and
   metadata all land in one flat `artwork/`.
4. **Frontmatter parser is ad hoc** (splits on the first `:`; only
   `references` is a list; quotes stripped by hand; `#` comments in the
   template are not stripped, so `aspect_ratio: "1:1" # ...` parses wrong).
5. **`googleSearch` grounding is always on** for an image model, with no
   option — it costs tokens and is not documented for image output.
6. **Batch semantics are odd:** N calls with identical inputs, each keeping
   only its first image; extra images in a stream are dropped.
7. **Style is a plain text file** with no structure, no provenance, and no
   way to derive one from examples.
8. Provider abstraction is nominal (Gemini only) but the code carries a
   client wrapper, validation and mime sniffing that a kit already provides.

## Behaviours to carry over (with changes)

| 1.2.2 | Rebuild |
|---|---|
| frontmatter fields | same names where they still make sense (`title`, `aspect_ratio`, `resolution`, `temperature`, `batch`, `style`/`instructions`, `references`), plus `cast`, `seed` |
| `instructions:` file | `style:` file (structured directive, see plan); `instructions:` accepted as alias |
| `references:` list | still supported for ad-hoc images; `cast:` resolves named characters to their approved reference images |
| dir processing + topo sort | kept; dependency = `cast` and `references` naming other prompts |
| `artwork/` flat | per-prompt candidate dirs, explicit `pick`, approved refs live with the entity |
| `--summary` JSON | always-on sidecar JSON per generated image (prompt hash, model, settings, references used) |
