# Research: character and scene continuity with Gemini image models

Compiled 2026-09-10. Claims are marked **[verified]** (primary text
fetched), **[snippet]** (official page quoted via search snippet only —
ai.google.dev and most Google doc hosts were unreachable from the research
sandbox) or **[community]** (practitioner reports).

## Model landscape

- Stable API image models: `gemini-3-pro-image` (Nano Banana Pro) and
  `gemini-3.1-flash-image` (Nano Banana 2), stable since 2026-05-28;
  `gemini-3.1-flash-lite-image` since 2026-06-30; `gemini-2.5-flash-image`
  is legacy. [verified: google-gemini/gemini-skills model list; Cloud blog]
- Google now positions the Interactions API as primary; `generateContent`
  is labelled "Legacy" in the docs nav but still served. [snippet]
- Imagen is deprecated; Google's migration target is the Gemini image
  models. [snippet]

## Seed: real technique or myth?

- `GenerateContentConfig.seed` exists: "If specified, repeated calls with
  the same seed will produce identical results." [verified: python-genai
  types.py]
- Google's parameter docs walk that back: "Deterministic output isn't
  guaranteed … the model makes a best effort to provide the same response
  for repeated requests." [snippet]
- Imagen had a real determinism contract; Gemini image models do not. The
  Nano Banana guides never list `seed` as a consistency technique. Field
  reports: identical requests with a fixed seed still differ. [community]
- Verdict: even a perfectly honoured seed only reproduces the SAME request.
  It does not carry a character into a different prompt. The documented
  substitute is reference images plus multi-turn context. dndig records the
  seed it sent in the sidecar for reproducibility, and nothing more.

## Officially recommended consistency techniques

1. **Reference images (multi-image composition)** — the primary technique.
   Up to 14 input images on Pro / Nano Banana 2 ("6 with high fidelity",
   "resemblance of up to 5 people"), 3 on Lite. Official prompt formula:
   `[Reference images] + [Relationship instruction] + [New scenario]`.
   Label images by role: "Use Image A for the character's pose, Image B for
   the art style, Image C for the background". "Do not assume the model
   knows whether image 2 is a character reference or a color reference."
   Fewer, cleaner references make instruction priority easier. Assign the
   character a NAME and refer to it in later prompts while changing only
   scene, action, lighting. Caveat: "character consistency is not always
   perfect". [verified cookbook + Cloud blog; snippets of blog.google]
2. **Multi-turn editing with the previous output in context.** On
   `generateContent` the caller keeps the whole `contents` history; images
   generated in the previous turn must be passed back as `inlineData`, and
   for Gemini 3 image models the **thought signatures** from previous
   responses must be returned. Edit prompts should say what changes and
   what stays exactly the same. [snippets; verified Cloud blog]
3. **Character sheets / turnarounds.** Google codelab "Generating Consistent
   Imagery with Gemini Nano Banana": approved portrait → character sheet
   (front/side/back, neutral background, captions) → scenes from prompt +
   assets. [snippet]
4. **Textual character bible.** Describe specifics (props, outfit, era)
   rather than keywords; but keep FACIAL description minimal when a
   reference image is present, or text and image fight and produce "a
   blended stranger". [verified Cloud blog; community]

## First-class identity feature?

None in the Gemini API. Imagen Subject Customization was the only
first-class subject reference and Imagen is deprecated.

## Style consistency

- Official: pass a style image and declare its role ("use the uploaded
  images as a strict style reference"); describe style in concrete
  cinematographic vocabulary (medium, brushwork, palette, film stock /
  colour grade, lighting such as "chiaroscuro", "golden hour backlighting",
  materiality). Multi-turn keeps style.
- Community: "extract then reuse" — have Gemini describe an example image
  as structured fields (palette, lighting, medium, camera, effects), then
  paste that block into every prompt. Fight realism bias with "NOT
  photorealistic". A fixed campaign-wide style paragraph reused verbatim
  makes every portrait "belong on the cover of the same book".
- `systemInstruction` is accepted by the models but there is NO official
  statement that image models honour it for style; putting the style
  block in the user prompt is the documented path. dndig therefore sends
  the style block in BOTH places (system instruction and prompt preamble).

## Multi-character scene recipes (community, convergent)

1. One clean, front-facing, evenly lit reference (or a small sheet) per
   character. Quality in equals quality out.
2. Assign roles by position: "Image 1 is Kaelen — keep her face, hair and
   armour exactly as shown; Image 2 is Borin — …".
3. Scaffold group scenes: two characters first, lock, then add the third in
   a follow-up turn; the model may swap features between four characters
   placed at once.
4. Lock lines: "do not blend facial traits, swap clothing or duplicate
   characters".
5. Name signature props or the model quietly drops them.
6. Change one variable per turn; re-anchor to the frozen references every
   few edits rather than chaining off drifted outputs.
7. Text first, then images, in official examples; "image N" refers to
   upload order.

## Temperature

Range 0–2. No official guidance ties temperature to consistency for image
models. Community reports lower values (0.1–0.3) make edits more
conservative. Harmless to expose; not a substitute for references.

## Recommended workflow (what dndig implements)

1. Campaign style block written once (or derived from examples), reused
   verbatim everywhere.
2. Iterate a portrait in one refinement thread with the previous image and
   its thought signatures replayed; "change X, keep everything else".
3. Approve a candidate → it becomes the character's reference; optionally
   generate a sheet (front/side/back) from it in the same thread.
4. Scenes: fresh request; style block + labelled reference images per cast
   member + relationship instruction + scenario + lock line. Stay ≤5
   people; expect fidelity to drop past 2–3.
5. Record the seed and every setting in a sidecar; do not rely on it for
   continuity.
