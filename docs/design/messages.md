# Server messages as a first-class event

Server output in a MUD has a natural turn boundary: a stretch of text plus the GMCP that updates state, ending in a prompt. Today the engine sees this stream as independent `TextLine`, `GMCPFrame`, and `Prompt` events — each its own batch, each its own pass through the chain. That's fine for stateless transformations, but every feature that wants the *turn as a unit* (TunnelVision attack consolidation, illusion detection, server-side misframing fixes) reinvents cross-batch buffering inside a processor. This document is the design that replaces those ad-hoc buffers with a typed `Message` event.

## Vocabulary: orphan GMCP

A GMCP frame is "orphan" when it arrives without a closing `Prompt` to bracket it as part of a turn. The term is only meaningful from the aggregator's perspective — at the wire level the frame is perfectly well-formed, but it has no prompt to attach to, so any "bundle until Prompt" scheme would either strand it indefinitely or attach it to a later, unrelated turn.

Empirical incidence: 3 of 35 GMCP frames in a 60-second probe session were orphans — the `Char.Items.List` / `Comm.Channel.Players` / `IRE.Rift.List` triple that arrives 112-116ms after the login prompt, in response to the world's `Char.Items.Inv` / `Comm.Channel.Players` / `IRE.Rift.Request` queries from the `Char.Name` handler. The server has data to return but no game-state change to announce, so it omits the prompt. Future client-initiated GMCP queries (`Room.Request`, on-demand `Char.Items.Inv`, …) will produce the same pattern. Orphans are a structural feature of the protocol, not a glitch.

## What's not used: GMCP-only bundling

An earlier sketch proposed making the aggregator the sole carrier of GMCP — bundle every frame into a `Message`, emit nothing immediately. We rejected it: in the probe session the three orphan frames sat unflushed for 6.3 seconds, until the user happened to type a command. Stranding state updates that long is unacceptable for vitals, inventory, target tracking. So GMCP must flow through immediately *as well as* land in the Message.

## What's not used: an aggregator that owns delivery

We also considered making the aggregator gate delivery — *every* `TextLine` / `GMCPFrame` becomes "in flight" until a Prompt closes the turn, then the whole bundle goes to downstream processors as one batch. Rejected: every existing processor that consumes `TextLine` or `GMCPFrame` would need to migrate to `Message`, and the migration would have to be atomic. Worse, processors that legitimately want per-line behaviour (raw log, generic.Output) gain a `Message`-shaped detour for no benefit. Additive beats invasive.

## The `Message` event

```go
type Message struct {
    app.EventMarker
    Lines  []connection.TextLine
    GMCP   []connection.GMCPFrame
    Prompt connection.Prompt
}
```

A `Message` is the contents of one turn: every text line and GMCP frame the aggregator observed since the previous prompt, followed by the closing prompt itself. Processors that want a turn-level view (TunnelVision rewrite, illusion detection, message splitter) handle `Message`. Processors that want the raw stream (output rendering, raw log, vitals state updater) keep handling the individual events. Both paths receive the same wire bytes — `Message` is additive, not authoritative.

## Dual delivery

For each underlying event:

- `TextLine` → engine emits `TextLine` immediately; aggregator buffers.
- `GMCPFrame` → engine emits `GMCPFrame` immediately; aggregator buffers.
- `Prompt` → engine emits `Prompt` immediately; aggregator buffers, then emits a derived `Message{Lines, GMCP, Prompt}` carrying the whole bundle.

The aggregator never *swallows* an event — every raw event still flows to downstream processors. The `Message` is a derived event appended to the batch that closed it. Downstream processors see it on the next loop iteration after the engine has drained the prompt's batch.

## Handling orphans

The aggregator does *not* try to detect or specially treat orphan frames. It buffers every GMCP frame as it arrives; whenever a Prompt eventually closes the buffer, whatever was queued — including any orphans accumulated since the previous prompt — joins that Message.

This is intentional and good enough:

- The *state-update path* (immediate `GMCPFrame` events) fires the moment the orphan arrives. Vitals, inventory, channels — all current as of the orphan's timestamp, regardless of how long the next prompt takes.
- The *turn-correlation path* (`Message`) sees the orphans attached to whatever prompt closes the buffer. Game logic that reads `Message` may see GMCP that "belongs" to a prior interaction. Illusion detection should evaluate based on what arrived during the relevant time window, not on inclusion in the bundle.

Two future refinements are noted but not built:

- **User-input flush:** when the next `ui.Input` arrives, the aggregator emits whatever's buffered as a standalone `Message` (Lines empty, Prompt zero-value) and resets. This aligns Message boundaries with user-perceptible turn boundaries during long idle stretches.
- **Timeout flush:** if no prompt arrives within N ms of the last buffered event, emit a standalone `Message`. Adds non-determinism to tests; only worth adding if a real consumer (illusion detection) needs it.

## Misframed server output

Servers occasionally pack two logical responses into one GA-terminated frame (the wire-level concept of "prompt" and the game-level concept of "turn" diverge). The aggregator can't tell these apart — it just sees one prompt and emits one `Message`. Splitting is a world-level concern: a downstream processor reads `Message`, detects multi-turn content (typically by recognising the *start* pattern of a second turn embedded mid-bundle), and emits two derived `Message` events. The aggregator stays simple; the world owns the policy.

Conversely, a single logical turn split across two GA boundaries can be joined by a similar processor that buffers an "incomplete" Message until the rest arrives. Same pattern, opposite direction.

## Aggregator placement

The aggregator sits early in the chain, right after the negotiation processor and before any world processor. It needs to see every `TextLine` / `GMCPFrame` / `Prompt` event, and its derived `Message` event needs to reach the world and post-world processors. Generic input handling (input split, repeat) runs before it because input-event flow is independent.

```
preWorld:
  rawLog
  negotiation              (existing)
  aggregator               (NEW — emits Message on Prompt)
  input / split / repeat   (existing, input-side)
world:
  world processors          (state, TunnelVision, …)
  autologin                (existing)
postWorld:
  output, sessionLog        (existing)
```

## What this enables

- **TunnelVision attack consolidation, simpler.** The current implementation buffers attack/modifier `TextLine`s across batches in the processor's struct (`attackParts`), flushes on the next non-attack TextLine or Prompt. Rewriting against `Message`: walk `msg.Lines`, classify, emit one consolidated `PrintLine` per attack run, done. No cross-batch state.

- **Illusion detection.** The detector inspects `msg.Lines` for "X afflicts you" assertions and compares against `msg.Prompt`'s parsed vitals (no health drop = illusion). All evidence in one place.

- **Misframing splitter.** A small processor watches `Message`s, detects the rare double-turn payload, splits.

- **Future world features.** Anything that needs "the lines and GMCP that arrived together" — auto-tagging, log replay annotation, scripted condition triggers — gets a clean event to subscribe to.

## What this doesn't change

- The connection adapter (`platform/telnet`). It keeps emitting `TextLine`, `Prompt`, `GMCPFrame`, `TelnetCommand` as today.
- Existing processors that consume the per-event stream. Raw log, generic output, vitals state updater — all unchanged.
- The reformatting round-trip (`docs/design/reformatting.md`). `ReFormat` still replays scrollback by `Line` ID; how processors react to the resulting `ReFormatting` events is orthogonal to whether they also subscribe to `Message`.

## TunnelVision is not a Message consumer

The original sequencing called for migrating TunnelVision attack consolidation to consume `Message`. We rejected that during implementation, after discovering the visible-order constraint:

- TunnelVision must emit the consolidated summary **before** the prompt renders, so the user reads `summary, prompt` top-down — the conventional MUD UI layout that matches Nexus and Mudlet.
- A `Message` arrives as a *derived* event, processed in its own batch **after** the prompt's batch has been dispatched. By the time a Message-consumer runs, the prompt's `PrintLine` has already landed in scrollback.
- For TunnelVision to flush before the prompt renders, it must react to the `Prompt` event directly (and keep the cross-batch state that buffers an in-flight attack run).

So TunnelVision keeps its existing shape — `TextLine`-based suppression of attack/modifier lines, flush on Prompt or non-attack TextLine. `Message` is reserved for consumers that don't have the rendering-order constraint: illusion detection, misframing splitters, post-hoc analytics, log-replay annotators.

A prerequisite that did get fixed alongside the Aggregator: the telnet adapter used to deliver multi-line server responses as a single multi-line `Prompt` (no `TextLine` events at all), which silently broke TunnelVision in production since the migration. The adapter now splits a GA-terminated token on embedded `\r\n` before emitting events. TunnelVision's `TextLine` path is real again.

## Sequencing

1. Define `connection.Message`. *Done.*
2. Implement `processors/generic.Aggregator()`. *Done.*
3. Wire it into the chain in `cmd/nogfx/main.go`. *Done.*
4. Fix the telnet adapter so a GA-terminated multi-line token emits one `TextLine` per `\r\n`-delimited body line plus a `Prompt` for the last line. *Done; was a prerequisite for either TunnelVision or any line-aware Message consumer to work against Iron Realms servers.*
5. First net-new `Message` consumer (illusion detection or misframing splitter) as the worked example, validating the design end-to-end. *Open.*
