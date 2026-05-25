# Tracking what we've sent — and what we haven't

The client needs to remember what *commands* are believed to be in flight at the server so it can correlate replies, recognise illusions, detect swallowed commands, and surface oddities. Not every in-flight command is something *we* sent: server-side abilities can force the character to act, and those forced actions need to be in the same queue so downstream code doesn't flag their effects as anomalies. The `Tracker` is the central data structure that holds them all.

## Note on language

This document leans hard on the MUD-domain meaning of "command": the line of plain text a player would type and send to the server — `kick rat`, `look`, `smile at guard`. That is **not** the same as `app.Effect` (the engine-dispatch abstraction implemented by `connection.Send`, `ui.PrintLine`, etc.). An Effect can carry a Command on the wire — `connection.Send.Bytes` is the literal command text — but the Tracker queue holds commands (strings), not effects. See [`docs/architecture/overview.md`](../architecture/overview.md) for the broader Event/Effect vocabulary.

## What it enables

- **Lag measurement.** Round-trip time from send to reply, sampled continuously. The observable surface is the `LagMeasured` derived event, produced by `LagWatcher`. LagWatcher does **not** go through the Tracker — GMCP heartbeats aren't commands in the domain sense, they're protocol traffic. See [Latency lives in LagWatcher](#latency-lives-in-lagwatcher) below.
- **Illusion detection.** Iron Realms attacks fake afflictions in the output stream. The signature is a "you have been afflicted with X" line with no game-state delta attached. The check is a Tracker query: "is there any pending command that would explain this affliction?". If `Find` returns nothing, the line is an illusion.
- **Swallowed-command detection.** "Amnesia" effects (and connection glitches) drop or reinterpret commands mid-stream. If we sent `attack; flex; smile` and only see effects of `flex` and `smile`, the first command vanished. The Tracker holds each outbound command long enough for a later command's resolution to surface the gap as a `Timeout` event.
- **Reorder detection.** Same machinery — pending commands with arrival expectations — flags commands whose effects arrived out of order, one of the patterns "scramble" afflictions produce.

Only the infrastructure is implemented end-to-end today; the resolvers for illusion / swallowed / reorder detection are the work the shape needs to generalise toward cleanly.

## Architecture

`processors/generic.Tracker` is a value (`*Tracker`), not a processor. Other processors take it as a dependency at construction time and consult or mutate it via methods:

- `List() []string` — copy of the queue, oldest-first. Answers "do we have anything pending?" / "how many?" / "what's in flight right now?" without mutating the queue.
- `Find(pred func(string) bool) (string, bool)` — read-only lookup; the call site for illusion detection and similar pure queries.
- `Resolve(pred func(string) bool, reply app.Event) []app.Event` — removes the first matching command, returns one `Resolved` event for it plus one `Timeout` for every command that preceded it; the caller appends the returned events to its batch. If nothing matches, the queue is left untouched and the slice is nil.
- `Record(command string)` — append a command to the queue. Called from two places: the `Recorder` processor (for our own outbound text sends), and world-specific processors (for server-forced actions they recognise in the output stream). Empty strings are dropped — see "Empty input" below.

The queue is `[]string` — plain command text. No wrapper struct, no timestamps, no effect wrappers.

`Resolved` and `Timeout` carry the command text and the triggering event, nothing else.

## Authoritative send signal: `connection.Sent`

`Recorder` does **not** read `batch.Effects`. Instead, the `Connection` endpoint (`platform/telnet`) returns a `connection.Sent` event from `Apply` after each successful wire write, carrying the original effect that was applied:

```go
type Sent struct {
    app.EventMarker
    Effect app.Effect
}
```

`Recorder` subscribes to this event, type-asserts the Effect against `connection.Send` (player text — not `connection.SendGMCP`, which is protocol traffic), and calls `tracker.Record(string(send.Bytes))`. The point: by the time `Sent` fires, the bytes have actually left the client. A later chain participant mutating `batch.Effects` (or the engine's `GuardedEvent` cycle-break dropping a forbidden effect) won't surface as a phantom queue entry, and a wire-write error won't either — both cases simply don't emit `Sent`.

### Accepted impurity: telnet negotiation replies

`generic.TelnetNegotiation` also emits its IAC reply bytes as `connection.Send` effects (rather than introducing a separate `SendIAC` type for what is, at the wire, "just write these bytes"). That means `Recorder` will record a handful of 3-byte IAC strings into the queue at session start, plus the occasional one mid-session when an option toggles (notably `Echo` around password prompts).

This is a deliberate trade. The cost is bounded (a few entries per session, never matched by any resolver predicate, so they surface as `Timeout` events ahead of the first real `Resolved`). The benefit is one fewer wire-write effect type and one fewer dispatch case in the telnet endpoint. The day a resolver makes the spurious `Timeout`s actually noisy is the day to revisit — either by splitting out `SendIAC` or by filtering IAC-prefixed bytes in `Recorder`. Until then, the simpler shape wins.

Apply-emitted events join the engine's local derived-event queue (see [`docs/architecture/overview.md`](../architecture/overview.md) on ordering), so `Sent` is guaranteed to flow through the chain before any new event arrives from the endpoint channel. The Recorder therefore registers the send before any server reply for it can be processed, and the buffered endpoint channel can never drop a `Sent` under load — there's no channel write involved in the first place. Tests drive `Apply` directly and assert on the returned events without needing a running `Run` loop.

## Chain participation

Two roles split the work, both placed in the processor chain:

- **One `Recorder`** (`generic.Recorder(tracker)`) — its trigger event is `connection.Sent` whose Effect is a `connection.Send`. No matching logic lives here.
- **Many resolvers** — world-specific processors for the harder mappings (a future `processors/achaea` resolver that knows "`You kick a rat.` resolves a pending `kick rat`"; an illusion detector that calls `tracker.Find` and decides). Each resolver inspects `batch.Event`, decides which queued command the event refers to, calls `tracker.Resolve(...)` or `tracker.Find(...)`, and appends any returned events to its batch.

Recorder reacts to `connection.Sent` events; resolvers react to whatever effect events their domain cares about. They never share an event type, so their ordering in the chain is not load-bearing for correctness — placement is just a readability concern.

## Latency lives in LagWatcher

`LagWatcher` is **outside** the Tracker entirely. GMCP heartbeats (Core.Ping, Core.KeepAlive) are protocol traffic, not commands a player types, so they don't belong in a "MUD commands in flight" queue.

LagWatcher keeps its own per-GMCP-message-ID FIFOs:

- It subscribes to `connection.Sent` events. For a latency-bearing GMCP send (Core.Ping, Core.KeepAlive today) it pushes `time.Now()` onto the FIFO for that message ID.
- It subscribes to `connection.GMCPFrame` events. For a latency-bearing reply it pops the oldest entry from the matching FIFO, computes the elapsed duration, emits `LagMeasured`, and dispatches `ui.SetLag`.

Per-message-ID FIFOs (rather than one shared FIFO) matter because Core.Ping and Core.KeepAlive interleave on the wire but only Core.Ping reliably gets a reply on Iron Realms. A single FIFO would let a Ping reply pop a stale KeepAlive timestamp and report 30-second lag.

## Recognising forced actions

World-specific processors own the parsing for forced-action lines. The shape:

1. A processor watches `connection.TextLine` (or `connection.Message`) events for the world's forced-action prefixes.
2. On a match where the forced command is observable ("Someone compells you to 'kick rat'."), the processor parses the command text out and calls `tracker.Record("kick rat")` — exactly what `Recorder` would have done if we'd sent it ourselves.
3. The subsequent effect line is resolved by the same world-specific resolver that handles "normal" effect lines — `tracker.Resolve` doesn't care whether the entry was added by `Recorder` or by direct `Record`.

If the *fact* of being forced is observable but the command isn't ("You feel your body being controlled by someone else."), there is no idiomatic way to record it today. We earlier considered an empty-string placeholder for this case but rejected it: empty player input (blank Enter to refresh a prompt) would collide with the placeholder. If illusion detection later needs to distinguish "we caused this" from "we don't know what caused this", the right move is a typed sentinel — for instance, a small `Forced{ /* tag */ }` value type the queue holds alongside strings — not an overloaded magic value.

## Empty input

A player pressing Enter on a blank input emits a `connection.Send{Bytes: nil}` — a routine interaction (refresh the prompt, advance a more-paged buffer) that the wire treats as a valid command but that has no useful Tracker semantics. `Recorder` drops it. `Tracker.Record("")` is also a no-op so a world-specific processor can't pollute the queue with empty entries either.

## Tick infrastructure

Heartbeats need a tick. `app/clock.Tick` is the contract event and `platform/clock.Ticker` is the endpoint that emits them. The Engine carries a `Sources []Endpoint` slice — emission-only endpoints that push events but never receive effects. The Ticker is the first source; the same hook is what a future "session-time elapsed" or "auto-save due" timer would use.

Tick is a coarse signal. Processors that need finer granularity than the engine's tick interval keep their own state and skip ticks that don't matter to them. This is why each heartbeat does its own elapsed-time check even though the Ticker fires once a second — the cadence belongs to the processor, not the clock.

## Open ends

- **No resolvers yet, so the queue grows unbounded.** `Recorder` populates the queue on every player command, but no resolver consumes from it in the current chain — `Find` callers don't exist either. Over a long-running session the slice grows by one entry per command and never shrinks. The leak is bounded by session length (sessions die regularly) and the size is trivial (tens of KB over hours of play), so it's a Note, not a blocker — but it's the most load-bearing reason to land at least one resolver soon.
- **Wall-clock timeouts.** Today a command exits the queue only by being shadowed by a later resolution. A wall-clock sweep — on each Tick, drop entries older than N seconds — would need a parallel timestamp structure (similar to LagWatcher's FIFO) rather than putting timestamps on the queue itself, so the timing-data overhead stays opt-in.
- **Text-command echo resolvers.** A generic resolver that matches a `TextLine` echoing the exact bytes of a pending command covers many simple commands. Worlds that decorate echoes (prompts, colour codes, paragraph reflow) need their own resolver instead.
- **Reconnection.** Pending state should drop when the connection drops. The hook is `connection.StateChanged{Connected: false}`; today the binary exits so the state goes with the session.
- **Fire-and-forget tagging.** Some commands and protocol sends don't expect a visible effect and shouldn't participate in correlation. A flag at `Record` time would let the queue skip them — useful once enough other commands are tracked that "always-shadowed-into-Timeout" entries become noise. The same flag in LagWatcher would close its small Core.KeepAlive FIFO leak on Iron Realms (KA timestamps that never pop because the server doesn't reply).
