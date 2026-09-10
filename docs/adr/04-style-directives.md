# ADR 04 — Style directives are files that can be derived

## Context

Style consistency across a campaign needs one text reused verbatim, and the
research shows the effective vocabulary is medium, brushwork, palette,
lighting, colour grade, composition, mood and what to avoid. Whether image
models honour `systemInstruction` is undocumented.

## Decision

- A style is `styles/<name>.md`: frontmatter (`name`, `derived_from`,
  `model`, `created`, optional `references:` style images) and a body that
  is the directive sent to the image model.
- The directive is sent as `systemInstruction` AND as the first paragraph of
  the user prompt ("Art style: …"), so it works either way.
- `dndig style derive` builds one from example images with an AgentKit
  agent on a vision model and a single terminating tool whose schema fixes
  the fields; the rendered file is human-editable.

## Consequences

- Style examples are not sent to the image model by default (they cost
  input tokens and can leak content); listing them under `references:` in
  the style file opts in, labelled "style reference only".
