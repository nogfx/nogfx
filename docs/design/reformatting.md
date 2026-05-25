# Output reformatting

Some features (TunnelVision, future highlighters, user-script formatters) need to change how output looks *after* it has already been printed — when the feature is toggled on or off, the user expects the existing scrollback to re-render under the new policy, not just future lines. This document captures the design that supports that.

## What's not used: tags

An earlier sketch had `ui.PrintLine` carry a `Tag` field so the UI could index its scrollback by classification ("attack", "weather", …) and a `Reformat` effect could target a specific tag. We rejected this. Tags require every emitter to maintain a taxonomy, processors and the UI both have to agree on the vocabulary, and classification ends up living in two places (the processor that emitted the line and the UI that stored the tag). Pattern-matching on the raw bytes — same as for live output — keeps classification in one place.

## The `Line` struct

`ui.PrintLine` carries a `Line` value rather than a bare `[]byte`:

```go
type Line struct {
    Raw       []byte // exactly what came off the wire
    Formatted []byte // what the UI should render
    ID        uint64 // opaque per-line identity assigned by the UI
}

type PrintLine struct {
    app.EffectMarker
    Line Line
}
```

Why the wrapper rather than three fields on `PrintLine` directly: processors only touch the fields they care about (most emitters set `Raw` and `Formatted` to the same bytes; only reformatters touch `Formatted` independently). The UI is free to add more fields — timestamps, ANSI metadata, source endpoint — without forcing every emitter to update.

`ID` is identity, not classification. Processors pass it through unchanged when rewriting; they never inspect it or branch on it. The UI assigns it on first print and uses it to find the right scrollback slot when an `UpdateLine` (or another `PrintLine` carrying the same ID) arrives.

## The round-trip

```
TunnelVision toggle
  → emit ui.ReFormat{}
       (effect, applied by the UI)
  → UI replays every scrollback line as ui.ReFormatting{Line} events,
    one event per line, in scrollback order
  → engine drains each ReFormatting event through the processor chain
  → processors that recognise the line emit a replacement ui.PrintLine
    (same ID → UI overwrites that slot)
```

Per-line granularity matches the existing one-event-per-batch model: the local FIFO queue drains them in order, no special path, processors stay identical to how they handle live `connection.TextLine` events (same patterns, same logic). If profiling later shows the per-line cost biting, a batched form can be added without changing the contract for processors that don't care.

### Scope is "everything"

`ui.ReFormat` carries no parameters:

```go
type ReFormat struct {
    app.EffectMarker
}
```

Every scrollback line replays. We considered scopes (all visible, last N) but each variant adds API surface for a use case that isn't measured. If a single ReFormat becomes too expensive in practice, we'll add scoping then with real numbers in hand — until then, one knob, one behaviour.

## Guarding against loops

The round-trip is intentionally re-entrant: a `ReFormatting` event flows through the same chain that emits `PrintLine` effects. If a processor mistakenly emits a `ReFormat` effect in response to a `ReFormatting` event, the next replay produces another `ReFormatting` flood, which re-enters the same code path. Easy footgun.

We bake the rule into the contract via an opt-in interface on events:

```go
type GuardedEvent interface {
    Event
    Forbids(Effect) bool
}
```

`ReFormatting.Forbids(ReFormat)` returns true. The engine, after each processor chain returns, checks `batch.Event` for `GuardedEvent` and walks `batch.Effects`: any forbidden effect is dropped with a log line. The bug isn't silent (the log makes it visible) but it doesn't loop.

A buggy processor doesn't take the session down — drop+log beats panic. If a class of cycle becomes recurring, the specific rule can escalate to a hard error later.

Most events implement nothing extra. `GuardedEvent` is the exception, not the norm.

## Why this composes

- The same processor that recognises a live attack line recognises it again on reformat — no duplicated logic.
- The UI stays declarative: it owns the scrollback (which it already does for rendering), it owns line IDs (an opaque identity it manages), but it knows nothing about *what* a line means.
- Worlds and scripts emit `ReFormat` whenever their classification policy changes; the round-trip handles the rest.
- The forbid-rule is local to the event type imposing it; no central registry.

## TunnelVision as the worked example

TunnelVision is the first feature built on the foundations:

1. On a live `connection.TextLine`, classify; attack and modifier lines accumulate in the processor's struct (cross-batch state) and the trigger event is nilled out; any other classification ends the run and a `ui.PrintLine` carrying the consolidated summary is appended to the batch's effects, so it lands in scrollback just before whatever line ended the run. `connection.Prompt` is also a flush trigger; interleaved GMCP frames (typical during a flurry) intentionally are not.
2. On toggle, emit `ui.ReFormat{}` — *not yet wired*; no toggle UX exists.
3. On `ui.ReFormatting{Line}`, reuse the same classifier; emit `ui.PrintLine{Line{Raw, Formatted, ID}}` with the new formatting and the same ID — *not yet implemented for TunnelVision*.

Cross-line consolidation in the reformat path is the open question. Live consolidation collapses N attack/modifier lines into one summary slot; the original raw bytes for the suppressed lines aren't preserved, so a toggle-off cannot reconstruct them. The reformat primitives overwrite a slot by ID but cannot delete or split slots, so reversing a live consolidation needs either richer primitives (delete) or a different live strategy (keep each line as its own slot and use the round-trip to compact in-place). That decision can wait until the toggle UX motivates it.

## Sequencing

The foundations and live TunnelVision consolidation have landed; remaining work is the toggle UX and TunnelVision's reformat-path handling. The original roadmap, for context:

1. Wrap the in-progress refactor (World API → `Processors()`, main.go composition). Touches the same emitters that step 2 will. *Done.*
2. Foundation: introduce `ui.Line`; change `ui.PrintLine` to carry it; update every emitter to populate `Raw` and `Formatted` (initially the same). UI assigns IDs on receive. *Done.*
3. Round-trip: add `ui.ReFormat` and `ui.ReFormatting` event. Wire the tui adapter to replay scrollback when `ReFormat` is applied. *Done.*
4. Guard: add `app.GuardedEvent`, post-chain check in the engine, `ReFormatting.Forbids(ReFormat) == true`. *Done.*
5. Restore TunnelVision attack consolidation. *Done in the live path*; the reformat-path handling is deferred (see worked-example notes above).
