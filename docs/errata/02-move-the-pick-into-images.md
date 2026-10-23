# Move the pick into images/

PRD 01 §5 says `pick` copies (never moves) a take to `<stem>/<stem>.png`,
and ADR 03 puts the pick and its sidecar inside the prompt's workspace.

`dndig pick` now **moves** the picked take's image into `images/` beside
the prompt file, named after the prompt and keeping the take's extension:

```
art/prompt.md
art/prompt/takes/002.jpg   ->   art/images/prompt.jpg
```

The pick sidecar is `art/prompt/pick.json`, so `images/` holds nothing but
approved images and is shared by every prompt in `art/`.

To keep the guarantees the copy gave:

- **The take keeps its sidecar.** `takes/002.json` stays in `takes/`, so
  status still counts it, numbering never reuses it and refine can still
  replay it. A take's image is looked up in `takes/` first, then in
  `images/` if that take is the pick (`Workspace.ImagePath`).
- **Picking again loses nothing.** The previous pick is moved back to
  `takes/<its source name>` before the new take moves in. The sidecar
  records the source name (`"source": "002.jpg"`).
- **Nothing is overwritten.** If an image is already at the pick path but
  no sidecar says which take it is, pick refuses and asks for it to be
  moved away.
- **The pick path does not depend on the sidecar.** It is whichever of
  `images/<stem>.png|.jpg|.webp|.gif` exists, so a lost `pick.json` does
  not orphan the reference that scenes use.

There is no migration: a project from before this change still has
`<stem>/<stem>.png`, which dndig no longer reads. Its takes are all still
in `takes/` (the old pick was a copy), so running `dndig pick` again moves
the take into `images/`; the old `<stem>.png` and `<stem>.json` can then
be deleted.
