# Choose takes with a flag

PRD 01, §9 specifies `dndig pick <prompt.md> <take>` while `dndig refine`
takes `--take N`: the same thing, a take number, written two ways.

The CLI now follows one rule. Positionals name what a command acts on
(prompt files, directories, images, the refine instruction); everything
else is a flag. A take is always chosen with `--take N`:

```
dndig pick   <prompt.md> --take N
dndig refine <prompt.md> "instruction" [--take N]
```

`refine` could not take a positional take instead: the take is optional
there and the instruction is free text, so `refine kaelen.md 3 more arrows`
would be ambiguous.

`--take` parses decimal, so the zero-padded numbers dndig prints
(`take 010`) can be pasted back; Go's `flag.Int` would read `010` as octal 8.
`dndig pick kaelen.md 4` is refused with a usage error that names
`dndig pick kaelen.md --take 4`.

In the same change, `dndig sheet` gained `--model ID`, so every command that
generates (`generate`, `refine`, `sheet`) shares `--model ID` and
`--dry-run`, and `-h` prints flags as the README spells them (`--take N`,
not `-take int`).
