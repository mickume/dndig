# Research: Gemini API surface for image generation

Compiled 2026-09-10. Sources, most to least reliable: **(A)** official SDK
source fetched in full (`google.golang.org/genai` v1.71.0, `google-genai`
2.22.0, official GoogleCloudPlatform notebooks); **(B)** Google Cloud blog;
**(C)** search snippets of `ai.google.dev` pages, which the research sandbox
could not open; **(D)** a third-party mirror of the Nov-2025 doc. Items marked
UNVERIFIED were confirmed by neither A nor B.

## Models

| Name | Model ID | Status | Notes |
|---|---|---|---|
| Nano Banana Pro | `gemini-3-pro-image` | GA since 2026-05-28 (C) | Highest quality. 1K/2K/4K. Up to 14 reference images: up to 6 objects (high fidelity) + up to 5 humans. |
| Nano Banana 2 | `gemini-3.1-flash-image` | GA since 2026-05-28 (B, C) | Workhorse. 512/1K/2K/4K. Up to 14 reference images. Video-to-image input. |
| Nano Banana 2 Lite | `gemini-3.1-flash-lite-image` | GA 2026-06-30 (B) | 1K only. Best with at most 3 reference images. |
| Nano Banana | `gemini-2.5-flash-image` | shuts down 2026-10-02 (A) | do not target |
| — | `gemini-3-pro-image-preview`, `gemini-3.1-flash-image-preview` | shut down 2026-06-25 (C) | **dndig 1.2.2 targets the first of these and no longer works** |
| Imagen 4 | `imagen-4.0-*` | shut down 2026-08-17 (C) | not wanted |

No newer image model exists in the API; "Nano Banana Next" is speculation.

Pricing (C, consistent across several secondary sources; per output image):
`gemini-3-pro-image` $0.134 (1K/2K) / $0.24 (4K), i.e. $120 per 1M
output-image tokens; `gemini-3.1-flash-image` $0.045 / $0.067 / $0.101 /
$0.151 for 512/1K/2K/4K ($60/1M); `gemini-3.1-flash-lite-image` $0.034 (1K,
$30/1M). Images cost 747 / 1120 / 1680 / 2520 output tokens at
512 / 1K / 2K / 4K. Each input image is billed as 1120 tokens. There is no
free tier for image models.

## generateContent request (A unless noted)

```
POST https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent
x-goog-api-key: $GEMINI_API_KEY
```
(`:streamGenerateContent?alt=sse` takes the same body; image parts arrive as
`inlineData` chunks. This is what the agentkit Google provider speaks.)

```json
{
  "contents": [{"role":"user","parts":[{"text":"..."},{"inlineData":{"mimeType":"image/png","data":"<base64>"}}]}],
  "generationConfig": {
    "responseModalities": ["TEXT","IMAGE"],
    "imageConfig": {"aspectRatio": "16:9", "imageSize": "2K", "personGeneration": "ALLOW_ALL"},
    "thinkingConfig": {"thinkingLevel": "HIGH", "includeThoughts": true},
    "seed": 12345
  },
  "tools": [{"googleSearch": {}}]
}
```

- `responseModalities`: `"TEXT"`, `"IMAGE"`; `IMAGE` is required for image
  output, `["IMAGE"]` alone gives image-only output.
- `imageConfig.aspectRatio`: `1:1, 2:3, 3:2, 3:4, 4:3, 4:5, 5:4, 9:16, 16:9,
  21:9, 1:4, 4:1, 1:8, 8:1`.
- `imageConfig.imageSize`: `"512"` (no K, 3.1 Flash only), `"1K"` (default),
  `"2K"`, `"4K"`; uppercase K required. Pro: 1K/2K/4K. Lite: 1K only.
- `imageConfig.personGeneration`: `ALLOW_ALL | ALLOW_ADULT | ALLOW_NONE`.
  `outputMimeType`, `outputCompressionQuality` are Vertex-only; on the Gemini
  API output is always PNG.
- `thinkingConfig.thinkingLevel` (`HIGH`/`MINIMAL`) works on the 3.1 image
  models; Pro thinks by default; interim "thought images" come back with
  `"thought": true` and are not charged. Setting thinkingConfig on a model
  that does not support it is an error.
- `temperature`: accepted (0–2) by the 3.x image models, but the sampling
  parameters are deprecated from Gemini 3.6 on (accepted and ignored, later
  an error). Do not build behaviour on it.
- `seed`: int32, accepted; "best effort … not guaranteed". Not mentioned in
  any image doc. Safe to send; never promise determinism.
- `candidateCount` > 1 is rejected for image models ("Multiple candidates is
  not enabled for this model"). N variants = N requests.
- `systemInstruction`: the field exists, but no official image sample uses
  it and no doc states image models honour it. UNVERIFIED. Put style text in
  the user prompt as well.
- `tools: [{googleSearch:{}}]` is supported on Pro and 3.1 Flash image
  models (optional, costs tokens).
- Inline request body ≤ 20 MB; use the Files API above that.

## Response

```json
{"candidates":[{"content":{"role":"model","parts":[
   {"text":"...","thought":true},
   {"inlineData":{"mimeType":"image/png","data":"..."},"thought":true},
   {"text":"Here is ...","thoughtSignature":"<opaque>"},
   {"inlineData":{"mimeType":"image/png","data":"..."},"thoughtSignature":"<opaque>"}]},
 "finishReason":"STOP"}],
 "usageMetadata":{...},"modelVersion":"gemini-3-pro-image","responseId":"..."}
```

- Skip parts with `thought: true`; the last non-thought `inlineData` is the
  final image. Output MIME is `image/png`.
- `finishReason` values relevant to images: `STOP`, `IMAGE_SAFETY`,
  `IMAGE_RECITATION`, `IMAGE_OTHER`, `NO_IMAGE` (plus `finishMessage`).
- No per-image seed or other metadata is returned. All images carry a
  SynthID watermark.
- Multi-turn editing: resend the full `contents` history including the
  model turn with its `inlineData` and **every `thoughtSignature`**; the
  history must end with a user turn.

## Interactions API

GA since June 2026 and "recommended for new projects"; `generateContent` is
labelled legacy but fully supported. Interactions offers
`previous_interaction_id` (server-side history, 55-day retention on paid
tier) and JPEG output. The agentkit speaks `generateContent`, which is
sufficient: dndig keeps its own refinement history locally, which also does
not expire.

## Consequences for dndig

1. Default model `gemini-3-pro-image`; `gemini-3.1-flash-image` for drafts.
   Never the `-preview` ids.
2. Request: `responseModalities`, `imageConfig{aspectRatio,imageSize}`,
   optional `seed`, optional `googleSearch` tool; no `candidateCount`; no
   `thinkingConfig` unless the user asks; `temperature` only when set.
3. Decode: ignore thought parts, take the final image, keep thought
   signatures for refinement turns, save as `.png`, and treat a response
   with no image as a failure that names `finishReason`.
4. Continuity = reference images + labelled prompt; `seed` is recorded for
   the record only.
