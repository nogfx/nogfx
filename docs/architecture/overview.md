# Architecture overview

This document describes how nogfx is organised: the package layout, how data flows end-to-end, and the contracts between the pieces. The migration that arrived at this shape is recorded in [`migration.md`](migration.md).

## How it works

Raw bytes arrive at one endpoint (the network or the terminal), get tokenised into typed **events**, flow through a chain of processors that can intercept and originate, and exit as typed **effects** that the destination endpoint applies. The same shape handles both directions, and the layering of processors (generic → game-specific → user scripts) is a property of how the chain is composed, not a separate concept.

A note on language. "Effect" is the engine-dispatch abstraction (`connection.Send`, `ui.PrintLine`, etc. — anything the engine applies to an endpoint). "Command" is reserved for the MUD-domain meaning: the line of plain text a player would type and send to the server. An Effect can carry a Command on the wire (`connection.Send.Bytes` is the literal command text), but the two are not the same thing. See [`tracking.md`](../design/tracking.md) for the consequences in the Tracker queue.

## Core property: endpoints have no project dependencies

The connection and the UI are pure endpoints. Each one:

- Emits the events it is responsible for (whatever it observes from its side of the world).
- Applies the effects directed at it (whatever falls within its capabilities).
- **Imports nothing else from the project beyond the `app/` port and message-type definitions.**

The connection does not know the UI exists. The UI does not know the connection exists. Neither knows about worlds, processors, scripts, GMCP, game state, or each other. All meaningful translation between "the server said X" and "the UI should show Y" happens in the processor chain.

This is the property that makes everything else fall out: worlds become portable (just a `Processor`), scripts intervene by editing the same data structure as everything else, and either endpoint can be replaced (a different terminal frontend, a different transport) without touching the other side.

## Package layout

```
nogfx/
  cmd/
    nogfx/
      main.go          — entry point; composition root only, no logic
      env.go           — runtime config (nogfx home dir, version constant)
  app/                 — abstract pipeline core
    batch.go           — Batch {Event, Events, Effects} envelope
    processor.go       — Processor signature + Chain composer
    event.go           — Event interface + EventMarker + GuardedEvent
    effect.go          — Effect interface + EffectMarker
    endpoint.go        — Endpoint interface satisfied by every platform impl
    engine.go          — pumps batches between endpoints, drains derived events;
                         carries Connection + UI plus a Sources slice of
                         emission-only endpoints (e.g. a Ticker) whose
                         events flow into the same channel
    connection/        — contract for the network endpoint
      events.go        — TextLine, Prompt, TelnetCommand, GMCPFrame,
                         StateChanged, Sent
      effects.go       — Send, SendGMCP, Reconnect, Disconnect
      iac.go           — IAC byte sequences (WillEcho, WontEcho, WillGMCP)
    ui/                — contract for the user-facing endpoint
      events.go        — Input, Resize, ReFormatting
      effects.go       — PrintLine, ReFormat, SetHealth, SetMana, AddVital,
                         SetVital, RemoveVital, SetCharacter, SetTarget,
                         SetRoom, SetLag, MaskInput, UnmaskInput
      line.go          — Line {Raw, Formatted, ID}
      snapshots.go     — Target snapshot
    clock/             — contract for the clock endpoint
      events.go        — Tick (periodic time signal for cadence-driven processors)
  platform/            — substrate adapters; bridge to external libraries
    telnet/            — Connection implementation; tokenisation
    gmcp/              — typed GMCP messages (parse / marshal)
    tui/               — UI implementation (tcell)
    clock/             — Ticker endpoint emitting clock.Tick at a fixed cadence
  processors/          — chain participants, generic and world-specific
    generic/           — MUD-agnostic processors (Input, Output,
                         TelnetNegotiation, Aggregator, KeepAlive, Ping,
                         LagWatcher, Tracker, Recorder, LogProcessor,
                         EventLogProcessor, MatchInput/Output, SplitInput,
                         RepeatInput)
    achaea/            — Achaea world: World, Character, Target, Learning,
                         TunnelVision, Bashing, agmcp dispatch
  internal/            — would-be standalone libraries, under development
    simpex/            — pattern matching (slated to move to its own module)
    navigation/        — graph and pathfinding (placeholder; will grow)
  tests/               — project-meta tests not tied to a single package
    architecture/      — dependency-rule test
```

Dependency direction. These rules are enforced by [`tests/architecture/architecture_test.go`](../../tests/architecture/architecture_test.go), which classifies each package by path and verifies its imports against an allow-list. The categories below match the constants defined there.

| Category | Packages | May import |
| --- | --- | --- |
| `app` | `app` | nothing else in the project |
| `contract` | `app/connection`, `app/ui`, `app/clock` | `app`, `lib` |
| `lib` | `internal/*` | nothing else in the project |
| `codec` | `platform/gmcp`, `platform/gmcp/*` | `lib` |
| `endpoint` | `platform/telnet`, `platform/tui`, `platform/clock` | `app`, `contract`, `lib` |
| `processors-generic` | `processors/generic` | `app`, `contract`, `lib`, `codec` |
| `processors-world` | `processors/<world>` and subpackages | `app`, `contract`, `lib`, `codec`, `processors-generic` |
| `cmd` | `cmd/*` | everything (composition root) |

Same-category imports are always allowed (e.g. `platform/gmcp/achaea` may import `platform/gmcp`). Tests under `tests/` sit outside the rule set.

Notes on what each rule expresses:

- **`app` and `lib` are leaves.** They depend on nothing in the project so that either can be extracted as a separate module without dragging the rest along.
- **Contracts live under `app/`** so the abstract trigger types (Event, Effect) sit next to the concrete event/effect vocabularies that implement them. Each contract (`app/connection`, `app/ui`, `app/clock`) is independent — they do not import each other, so adding a UI capability does not affect the connection package and vice versa. `app/clock` is the smallest of the three: a single `Tick` event for cadence-driven processors, no effects (the Ticker endpoint is emission-only).
- **`lib` is `internal/`** — would-be standalone libraries that depend on nothing in-project. The `internal/` location signals they are not part of any public surface yet; once a library stabilises (e.g. `simpex` is slated to move to its own module) it can be promoted out without changing the import direction.
- **The codec (`platform/gmcp`) does not import an endpoint.** GMCP messages are pure data; the wire transport (telnet) is separate. Worlds and processors can decode GMCP without dragging in telnet.
- **Endpoints do not import each other.** `platform/telnet` and `platform/tui` are siblings; if telnet ever needed to know about the UI it would mean we'd put logic in the wrong place.
- **`processors/generic` and `processors/<world>` share a parent** because both are chain participants implementing `app.Processor`. The asymmetry is in generality, expressed by the subdirectory — `generic/` is MUD-agnostic; `<world>/` carries world-specific state and feature processors. Worlds may import generic; generic may not import worlds.
- **Worlds do not import endpoints or the engine.** A world is just a bundle of processors that run against the contracts; it cannot reach down into a specific transport implementation or up into the engine wiring.
- **`app/` defines a single `Endpoint` interface** that both telnet and tui satisfy structurally. The Engine references it directly, so the abstract pipeline core stays free of endpoint-specific imports while still owning the engine wiring. `cmd/nogfx/main.go` is the only place that knows which concrete endpoint implementations are in use.
- **`cmd/nogfx/main.go` imports everything** and wires it together.

The repository root holds only directories and non-code files (README, LICENSE, go.mod, Makefile, etc.).

## Events, effects, and batches

The abstract shape in `app/`:

```go
type Event  interface { isEvent()  }   // marker only; concrete types live in contract packages
type Effect interface { isEffect() }   // marker only; concrete types live in contract packages

type Batch struct {
    Event   Event    // the trigger that started this batch
    Events  []Event  // derived events to be re-emitted as their own batches
    Effects []Effect // effects to dispatch to the endpoints
}

type Processor func(Batch) (Batch, error)
```

A **Batch** is the unit of work flowing through the processor chain, anchored on a single triggering event. Each event arriving from an endpoint becomes its own batch; processors decide whether to act based on the type of `batch.Event`, may rewrite or suppress it (set `batch.Event = nil`), and may append **effects** for the endpoints to apply and **derived events** that the engine will re-emit as their own batches downstream.

**Ordering invariants**, enforced by the engine loop:

1. Within a batch, all `Effects` are applied (in order) before any derived `Events` are re-emitted.
2. Apply-consequence events (events `Apply` returns synchronously, e.g. `connection.Sent` after a wire write) and processor-derived events are re-emitted in their appended order, each as its own batch, before the engine returns to reading new events from the endpoint channel.
3. Any **endpoint-channel** event therefore lands *after* the entire chain reaction triggered by the original event has completed — including wire-level server replies, which arrive via the endpoint's `Run` goroutine, not via `Apply`.

The mechanism is a local FIFO queue of derived events (whether processor-emitted or apply-emitted) that the engine drains before reading the endpoint channel. This is why a processor can confidently append a derived event knowing the rest of the chain reaction will run to completion before any unrelated endpoint-channel event interleaves.

"Apply-consequence event" and "endpoint-channel event" are deliberately distinct terms: the first is a synchronous `Apply` return; the second covers anything an endpoint pushes onto the shared channel from its `Run` goroutine — including the *wire-level* server reply that often follows a `Send`, which is not the same thing as the synchronous `Sent`.

**Events** identify something that happened. Concrete event types live in the contract package owned by their originating endpoint — `connection.TextLine`, `connection.Prompt`, `connection.GMCPFrame`, `ui.Input`, `ui.Resize`, and so on. Processors may also synthesise their own derived events to represent recognised patterns or aggregated state (e.g. `connection.Message` bundling a turn's lines, GMCP frames, and prompt). All concrete events embed `app.EventMarker` to satisfy `app.Event`.

**Effects** identify something to be done. Concrete effect types live in the contract package owned by the endpoint that applies them — `connection.Send`, `connection.SendGMCP`, `connection.Disconnect`, `ui.PrintLine`, `ui.SetHealth`, `ui.AddVital`, `ui.SetLag`, and so on. All concrete effects embed `app.EffectMarker`. The engine routes each effect to the endpoint that owns its type by attempting dispatch through each endpoint in turn; processors never name a target explicitly.

Payload immutability: byte-carrying effects (`connection.Send.Bytes`, `connection.SendGMCP.Payload`) are read-only once appended to a batch. Producers may share a single backing array across emissions; downstream processors that need a transformed value append a *new* effect rather than editing the slice in place. The contract is documented on the types themselves; the heartbeat is a worked example of safe sharing.

Endpoints may synthesise their own derived events directly from `Apply` by returning them. The canonical case is `connection.Sent`, emitted after a successful wire write so the Recorder can register the send authoritatively. The engine queues apply-emitted events into the same local FIFO as processor-emitted derived events: they flow through the chain *before* any new event from the endpoint channel, but *after* the dispatch of every other effect in the same batch. A wire-write failure returns no event, so the Tracker never registers a phantom send.

(In the MUD-vocabulary sense, "command" means the line of text a player types — the bytes inside a `connection.Send`. The Tracker queue is about *those* commands; the engine itself never traffics in them as a distinct type. See [`docs/design/tracking.md`](../design/tracking.md).)

A processor's job is to read `batch.Event`, append events and effects as it sees fit, and return the batch. Processors run in a fixed order in the chain; later processors see what earlier ones produced. The same chain handles both directions — there is no "inbound" vs "outbound" mode. A `ui.Input` event triggers some processor to emit a `connection.Send`; a `connection.TextLine` event triggers some processor to emit `ui.PrintLine`. Many processors react in only one direction; some legitimately span both. Processors that need to consolidate across multiple events (e.g. paired suppression, multi-line summaries) must carry explicit state across batches, since each event arrives as its own independent batch.

## Processors

Every processor implements the same `Processor` signature. They differ along two independent axes — **source** (who wrote it and where it lives) and **phase** (when it runs in the chain).

### Source

**Generic processors** live in `processors/generic/` and apply to any GMCP-capable MUD. They know the standard event/effect vocabulary but nothing about any specific game's mechanics. The set today:

- **`Input`** — converts `ui.Input` events into `connection.Send` effects. The bridge that gets keystrokes onto the wire.
- **`Output`** — converts `connection.TextLine` / `connection.Prompt` events into `ui.PrintLine` effects. The mirror of Input on the server side.
- **`SplitInputProcessor`** — splits `connection.Send` effects on a configured separator (default `;`) into one effect per part.
- **`RepeatInputProcessor`** — expands a numeric prefix (`3 kick`) into the effect repeated that many times.
- **`TelnetNegotiation`** — replies to IAC WILL/WONT/DO/DONT according to a `NegotiationPolicy`. The negotiation state machine lives here rather than in the telnet endpoint, so policy is a chain concern.
- **`Aggregator`** — buffers `TextLine` and `GMCPFrame` events between prompts and emits a `connection.Message` derived event on each `Prompt`. Additive — the raw events still flow through. See [`messages.md`](../design/messages.md).
- **`KeepAlive` / `Ping`** — periodic GMCP heartbeats driven by `clock.Tick`. KeepAlive resets the server's idle timer; Ping is the bidirectional latency probe.
- **`LagWatcher`** — measures GMCP round-trip latency for the heartbeats above, per-message-ID. Emits `LagMeasured` and dispatches `ui.SetLag`. See [`tracking.md`](../design/tracking.md).
- **`Tracker` / `Recorder`** — the in-flight MUD-command queue and the processor that populates it from `connection.Sent` events. Resolvers (world-specific) consume from the queue. See [`tracking.md`](../design/tracking.md).
- **`AutoLogin`** — sends a `Char.Login` GMCP frame on first IAC WILL GMCP, using credentials supplied at construction.
- **`LogProcessor`** — appends the event/effect stream to a session log file.
- **`EventLogProcessor`** — writes one timestamped tagged line per batch trigger to a debug log. On by default in headless mode (the assistant's observation surface); opt-in elsewhere.
- **`MatchInput` / `MatchOutput`** — simpex-based pattern matchers that invoke a callback on a match.

**Game-specific processors** live in `processors/<world>/` and know how a particular MUD works. They decode the game's GMCP extensions, recognise game-specific patterns in text output, maintain rich game state, and emit effects that go beyond the generic baseline (custom vitals, target tracking, learning, tunnel vision, balance timing, …).

**User scripts** are also processors. They live in the user's configuration directory and are loaded at startup in a deterministic order (filename or config).

All three sources implement the same `Processor` signature and slot into the same chain. The source axis is about ownership and lifecycle, not about runtime behaviour.

### Composition

The chain is assembled by `cmd/nogfx/main.go` — the composition root. Worlds know only about themselves; they do not wrap scripts, own logging, or know which generic processors run around them. The composition root knows everything and decides the order.

```go
// cmd/nogfx/main.go
chain := app.Chain(append(append(append(
    preWorld,        // raw log, Input, SplitInput, RepeatInput, …
    worldProcs...),  // world.Processors()
    scripts...),     // user-loaded
    postWorld...,    // Output, session log, …
)...)
```

```go
// processors/achaea/achaea.go
type World struct { state *world; tv TunnelVision }

func New() *World { return &World{ ... } }

func (w *World) Processors() []app.Processor {
    return []app.Processor{
        newRewriteOutput(),       // drop server's blank-echo lines
        w.state.cmdprocess,       // Achaea GMCP dispatch + state mutation
        (&Learning{}).Processor(),
        NewBashing(w.state).Processor(),
        w.tv.rewriteProcessor(),
    }
}
```

The world contributes one slice; the composition root sandwiches it. There is no "Pre" / "Post" split on the world itself — what used to be Pre and Post is now whatever main.go wraps around the world's slice, plus the order within the world's slice itself.

Scripts go between the world and the generic output stage: they see the events the world has decoded and the effects it has staged, and can modify either before logging and `ui.PrintLine` commit them. A script that wants to run earlier or later doesn't yet have a knob — when one's needed it'll be a configuration choice at composition, not a world API change.

## The two ports: Connection and UI

The pipeline has two endpoints. They are asymmetric in their domain — one speaks a wire protocol, the other renders a terminal — but symmetric in their shape: each produces events from its outside world and applies effects directed at it. They are otherwise oblivious to anything else in the project.

The Engine also accepts a `Sources []Endpoint` slice for emission-only endpoints (today: `platform/clock.Ticker`, which emits `clock.Tick` on a cadence). Sources push into the same event channel as Connection and UI but receive no effects — they are the hook for things processors need from outside the wire but that don't belong to the connection or the UI, like a periodic time signal.

### Connection

Represents the network endpoint. Its read path is a **tokeniser** — raw bytes are turned into typed events at the boundary, so no processor sees raw bytes. The concrete event types live in the `app/connection/` contract package:

- `connection.TextLine` — one paragraph of output text.
- `connection.Prompt` — a GA-terminated prompt line. (Today this logic is buried in `engine.go`.)
- `connection.TelnetCommand` — IAC sequences.
- `connection.GMCPFrame` — the raw payload of a GMCP subnegotiation envelope. The connection does *not* decode message types; downstream world-specific dispatch (e.g. Achaea's `agmcp.Parse`) handles that, which keeps the connection ignorant of game-vocabulary GMCP messages.
- `connection.StateChanged` — connected, disconnected, errored.
- `connection.Sent` — feedback fired after a successful wire write, carrying the original effect so the Tracker can record what actually left the client.

Its apply path consumes effects from the same package:

- `connection.Send{Bytes}` — write to the wire. `Bytes` is the MUD-domain command (the line of text a player would type).
- `connection.SendGMCP{Payload}` — write a GMCP subnegotiation frame.
- `connection.Disconnect` — close the link. The endpoint stops emitting events and the engine shuts down once `Run` returns.

Tokenisation is part of `platform/telnet/` — it is the connection's read implementation, not a separate package. The connection package handles only the wire envelope (`IAC SB GMCP ... IAC SE` framing); typed GMCP messages live in the sibling `platform/gmcp/` package.

`Connection` replaces the existing `pkg.Client` interface. The rename clarifies what the port is — "the connection to the server" — and avoids the MUD-vocabulary confusion where "client" usually refers to the whole application.

### UI

Represents the terminal endpoint. Its read path produces events from the `app/ui/` contract package:

- `ui.Input{Bytes}` — a line the user submitted (after pressing enter; UI buffers keystrokes locally).
- `ui.Resize`, future `ui.WidgetClicked`, etc.

Its apply path consumes effects from the same package:

- `ui.PrintLine{Text}` — append a line to the scrollback.
- `ui.SetHealth{Value, Max}`, `ui.SetMana{Value, Max}` — primary vitals.
- `ui.AddVital{Name, Value, Max}`, `ui.SetVital{Name, Value, Max}`, `ui.RemoveVital{Name}` — auxiliary, named vitals.
- `ui.SetCharacter{Name, Title}`, `ui.SetTarget{*ui.Target}`, `ui.SetRoom{*ui.Room}` — structured state.
- `ui.MaskInput`, `ui.UnmaskInput` — input echo control.

The UI is *declarative*. It knows nothing about game mechanics — what Ki or Karma means, whether a player has died, what a room's exits denote. It applies the effects it receives, and emits events for what the user does. World-specific knowledge is pushed entirely into the processors that produce the effects.

### Named widgets and idempotency

The UI's named-widget effects (`AddVital`, `SetVital`, `RemoveVital`) are **idempotent**. `AddVital{"ki", v, m}` means "ensure the Ki vital exists with these values"; emitting it every pass where the game state changes is fine. State is owned by the emitter (the Achaea processor), not by the UI; the UI is a renderer.

This eliminates the need for handles, callbacks, or lifecycle events for widget management. If a user script wants to suppress a vital permanently, it becomes a processor that drops `AddVital{"ki", …}` effects as they pass. Layered processors mean this composes naturally without requiring the game processor to "know" the widget was suppressed.

## Worlds and rich state

A world is the bundle of game-specific processors plus the rich state they consult. For Achaea, `processors/achaea/` holds `Character` (with all the Achaea fields — vitals, balance, stances, Ki, Karma, etc.), `Target`, `Area`, and the methods that mutate them from typed GMCP events. The Achaea world is the only place that ever sees this rich state; effects going to the UI are the projections.

### Room duplication

`app/ui.Room` (the perceived room: description, exits, items, present npcs) and `internal/navigation.Node` (the graph node: id, coordinates, edges) are intentionally separate types that share the name "room" only in conversation. The translation between them happens inside the world adapter, because the mapping is game-specific. Do not introduce a shared `Room` interface — the duplication carries information about which concern owns the data.

## Data flow

There is one chain, and it handles both directions. The order is the same; what differs between directions is which events arrive and which effects the chain produces.

Inbound (something arrived from the server):

```
socket bytes
  → platform/telnet
       (tokenise; emit connection.TextLine, connection.Prompt,
        connection.TelnetCommand, connection.GMCPFrame)
  → app/Engine wraps the event into a Batch

  ── composition-root pre-world ──
  → raw log
  → processors/Input, SplitInput, RepeatInput (no-ops for inbound events)

  ── world.Processors() ──
  → world's output rewrite                  (drop server-echo blanks etc.)
  → world's GMCP dispatch + state mutation  (Achaea's agmcp.Parse, vitals, target)
  → world's feature processors              (Learning, Bashing, TunnelVision, …)

  ── scripts ──
  → user scripts               (may modify or originate any event or effect)

  ── composition-root post-world ──
  → processors/Output          (connection.TextLine / .Prompt → ui.PrintLine)
  → session log

  → app/Engine drains:
       ui.* effects → ui port Apply
       connection.* effects → connection port Apply
```

Outbound (something happened in the UI):

```
user presses enter
  → platform/tui emits ui.Input event
  → app/Engine wraps event into a Batch

  ── composition-root pre-world ──
  → raw log
  → processors/Input           (ui.Input → connection.Send)
  → processors/SplitInput      (separator split)
  → processors/RepeatInput     (repeat prefix)

  ── world.Processors() ──
  → world processors           (alias and macro expansion on connection.Send)

  ── scripts ──
  → user scripts

  ── composition-root post-world ──
  → processors/Output          (no-op for outbound)
  → session log

  → app/Engine drains:
       connection.* effects → connection port Apply (Send writes the bytes)
       ui.* effects → ui port Apply (e.g., a script asked to print a confirmation)
```

The same processor chain serves both. A processor that only cares about one direction reads only the events it recognises and emits only the effects it produces; processors that span both directions (the world's GMCP dispatch can react to server events and emit server effects) react in the order they appear in the chain.

## How it got here

The architecture is the outcome of a migration from a half-finished `Inoutput` refactor that left two processor signatures coexisting in `pkg/` and a number of types referenced but never defined. The full step-by-step record — what landed when, what decisions were made along the way, and which features needed redesign rather than mechanical conversion — is in [`migration.md`](migration.md).
