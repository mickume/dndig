# Plan a run past missing picks

PRD 01 §6 says "a missing pick is an error that names the command to run",
and dndig stopped there: `generate` returned at the first prompt whose
`cast:` (or a title in `references:`) had no pick. On a fresh project
`dndig generate characters/ scenes/ --dry-run` therefore failed at the
first scene, although the run orders the characters first and, with
`--auto-pick`, would have picked them before the scenes are assembled.

A missing pick is still reported with the command to run, but:

- **A dry run plans past it.** Nothing is generated on a dry run, so a
  dependency's pick may not exist yet. It is shown as a placeholder in
  the image list, numbered like the pick it stands for, and the rest of
  the plan is printed:
  - `(pick of kaelen, generated earlier in this run)` when the dependency
    comes earlier in the same ordered run; without `--auto-pick` the plan
    also warns that nothing will pick it in time.
  - `(no pick of kaelen yet)` plus a warning naming `dndig pick` when it
    is outside the run.

  Warnings leave the exit code at 0. A prompt that cannot be planned at
  all (unknown cast member, unsupported resolution, too many references)
  is printed as an error in its place, the remaining prompts are still
  planned, and the dry run exits 1 listing every such prompt.
- **A real run keeps going.** A prompt that cannot be planned, a missing
  pick included, is skipped with its error and recorded among the
  run's failures alongside generation errors; the prompts after it still
  run, and the run exits 1 listing them at the end.

A placeholder plan is for display only (`Plan.Placeholders` counts its
placeholders; its request lacks those images). A real run never builds
one: it assembles each prompt only once the prompts before it have been
generated and, with `--auto-pick`, picked.
