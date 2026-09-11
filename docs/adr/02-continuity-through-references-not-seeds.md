# ADR 02 — Continuity through reference images and threads, not seeds

## Context

The request was to research whether reusing the seed of an approved
character image could carry the character into new scenes. Google's own
documentation says `seed` on Gemini models is best-effort and not
guaranteed; no image-generation doc lists it as a technique; and a seed can
at most reproduce the *same* request. The documented continuity techniques
are reference images with role labels, multi-turn editing with the previous
output and its thought signatures in context, and character sheets.

## Decision

1. The **pick** (approved take) of an entity is its reference image, stored
   at a stable path. Scenes list a `cast`; dndig sends each member's pick
   (and sheet when present) as `inlineData` parts and labels them by
   position and name in the prompt preamble, followed by a lock line for
   multi-character scenes.
2. **Refinement** replays the take's original request, the model turn
   (image, text, thought signatures) and the new instruction, so the model
   edits rather than regenerates. Threads are stored in take sidecars (file
   references, no base64), not in the kit's session log.
3. `seed` is sent when the user sets it and always recorded, with no claim
   of determinism.
4. Limits from the docs are enforced: ≤14 images, ≤5 cast members, a
   warning above 3 (community reports fidelity drops).

## Consequences

- A scene cannot be generated until its cast has picks; the error names
  `dndig pick`.
- Thought signatures on image parts must survive decoding; the kit's Google
  decoder dropped them; fixed in the kit (agent-fox-dev/agentkit-go commit da5b772, `fix(google): keep the thought signature carried on an image part`).
