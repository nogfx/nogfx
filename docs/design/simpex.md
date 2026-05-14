# simpex — the pattern language

`pkg/simpex` is a homegrown pattern matcher used in place of `regexp` for triggers and line parsers throughout the world layer. It exists because MUD trigger patterns are typically simpler than what regex offers, and the simpler vocabulary is easier to read at a glance when patterns are sprinkled across world code.

## Syntax

| Token | Meaning |
| --- | --- |
| `{...}` | Capture group. Captures are returned in order as a `[][]byte`. |
| `*` | Wildcard. Matches any run of characters up to the next literal segment of the pattern. |
| `^` | Word. Matches one alphanumeric run (`[0-9A-Za-z]+`). |
| `_` | Single-rune wildcard. |
| `{{`, `}}`, `**`, `^^`, `__` | Literal `{`, `}`, `*`, `^`, `_` respectively. |

`simpex.Match(pattern, text) [][]byte` returns the captures on success, or `nil` if the pattern doesn't match.

## When to use it

- Triggers, parsers, and line classifiers in `pkg/world/*` and `pkg/process/*`.
- Anywhere the pattern is structurally simple (anchored from the start of the line, a few captures, a wildcard or two).

## When to reach for `regexp` instead

- The pattern needs alternation, backreferences, lookaround, or character classes beyond word/digit.
- The pattern is performance-critical and being applied to a high-throughput stream — `regexp` precompiles.

Defaulting to simpex keeps the world layer consistent. Falling back to `regexp` is fine when the shape genuinely doesn't fit; document why in the call site if it isn't obvious.

## Wiring it into the Processor chain

Don't call `simpex.Match` directly from a processor. Use the helpers in `pkg/process/match.go`:

```go
process.MatchInput("^ attacks {*}", func(match Match, ins, outs [][]byte) ([][]byte, [][]byte, error) {
    // match[lineIndex] gives you the captures from that line
})
```

`MatchInput` / `MatchInputs` runs against player input; `MatchOutput` / `MatchOutputs` runs against server output. Both return a `Processor` you can drop into a `ChainProcessor`.
