# Target architecture

This document describes the architecture nogfx is migrating toward. The current shape (Engine + Client + UI + chained Processors, all under `pkg/`) is described in [`../architecture/overview.md`](../architecture/overview.md) and remains the source of truth until the migration completes. Execution progress is tracked in [`target-migration.md`](target-migration.md).

## Motivation

The current `pkg/` directory mixes three concerns: application orchestration (`Engine`, the `Client` and `UI` interfaces), presentation projections (`Character`, `Target`), and runtime configuration (`variables.go`). World logic sits cleanly in `pkg/world/<world>/`, but the world processor reaches into the UI through typed setters that bypass the processor chain, and the in-flight `Inoutput` refactor is half-applied — two processor signatures coexist in the codebase.

The goal of the refactor is to make the data flow uniform end-to-end: raw bytes arrive at one endpoint (the network or the terminal), get tokenised into typed **events**, flow through a chain of processors that can intercept and originate, and exit as typed **commands** that the destination endpoint applies. The same shape handles both directions, and the layering of processors (generic → game-specific → user scripts) is a property of how the chain is composed, not a separate concept.

## Core property: endpoints have no project dependencies

The connection and the UI are pure endpoints. Each one:

- Emits the events it is responsible for (whatever it observes from its side of the world).
- Applies the commands directed at it (whatever falls within its capabilities).
- **Imports nothing else from the project beyond the `app/` port and message-type definitions.**

The connection does not know the UI exists. The UI does not know the connection exists. Neither knows about worlds, processors, scripts, GMCP, game state, or each other. All meaningful translation between "the server said X" and "the UI should show Y" happens in the processor chain.

This is the property that makes everything else fall out: worlds become portable (just a `Processor`), scripts intervene by editing the same data structure as everything else, and either endpoint can be replaced (a different terminal frontend, a different transport) without touching the other side.

## Package layout

```
nogfx/
  cmd/
    nogfx/
      main.go          — entry point; composition root only, no logic
  app/                 — abstract pipeline core
    batch.go           — Batch {Events, Commands} envelope
    processor.go       — Processor signature + Chain composer
    event.go           — Event interface + EventMarker
    command.go         — Command interface + CommandMarker
    engine.go          — pumps batches through the processor chain
  connection/          — contract for the network endpoint
    events.go          — TextLine, Prompt, TelnetCommand, GMCPFrame, StateChanged
    commands.go        — Send, Reconnect, Disconnect
    connection.go      — Connection port (deferred until step 3)
  ui/                  — contract for the user-facing endpoint
    events.go          — Input, Resize
    commands.go        — PrintLine, SetHealth, SetMana, AddVital, SetVital,
                         RemoveVital, SetCharacter, SetTarget, SetRoom,
                         MaskInput, UnmaskInput
    snapshots.go       — Target, Room (UI-facing snapshot types)
    ui.go              — UI port (deferred until step 3)
  platform/            — substrate adapters; implement endpoint ports
    telnet/            — Connection implementation; tokenisation; GMCP envelope
    gmcp/              — typed GMCP messages (parse / marshal)
    tui/               — UI implementation (tcell)
  processors/          — generic, world-agnostic processors
  worlds/              — game-specific processor bundles
    achaea/
      gmcp/            — Achaea GMCP extensions
      state/           — rich Character, Target, Area
      features/        — Learning, TunnelVision, bashing, procs/
      achaea.go        — composes generic + Achaea-specific processors
  lib/                 — general-purpose libraries; no app/platform dependencies
    simpex/            — pattern matching
    navigation/        — graph and pathfinding
```

Dependency direction:

- `app/` imports nothing else in the project — only the abstract pipeline, processor, event-, and command-interface shapes. It knows nothing about specific endpoints or their vocabularies.
- `connection/` and `ui/` import `app/` to satisfy the `Event` and `Command` interfaces. They do **not** import each other — that's the decoupling that lets either endpoint evolve independently.
- `lib/*` imports nothing else in the project.
- `platform/telnet` implements the `connection.Connection` port and imports only `app/`, `connection/`, and `lib/*`. It does not import `ui/`, `processors/`, `worlds/`, or other platform packages.
- `platform/tui` implements the `ui.UI` port and imports only `app/`, `ui/`, and `lib/*`. Symmetric to telnet — does not import `connection/` or anything else.
- `platform/gmcp` imports `app/` (so its message types can implement `app.Event`) and `lib/*`.
- `processors/` imports `app/`, `connection/`, `ui/`, `platform/gmcp`, `lib/*`. Generic processors translate between the two endpoint vocabularies, so they need both.
- `worlds/*` imports `app/`, `connection/`, `ui/`, `platform/gmcp`, `processors/`, `lib/*`. Same reasoning as processors: a world emits both server commands (Send) and UI commands.
- `cmd/nogfx/main.go` imports everything and wires it together.

The repository root holds only directories and non-code files (README, LICENSE, go.mod, Makefile, etc.).

## Events, commands, and batches

The abstract shape in `app/`:

```go
type Event   interface { isEvent()   }   // marker only; concrete types live in contract packages
type Command interface { isCommand() }   // marker only; concrete types live in contract packages

type Batch struct {
    Events   []Event
    Commands []Command
}

type Processor func(Batch) (Batch, error)
```

A **Batch** is the unit of work flowing through the processor chain. It starts with one or more triggering events (something arrived from the server, or the user pressed enter) and accumulates as processors run — each processor reads what's there, optionally appends events (derived facts, recognised patterns) and commands (intent for an endpoint), and passes the batch along. Keeping events alongside commands as the batch flows is the point: a script several stages downstream can look at `batch.Events` and see the whole story of *why* a given command was added, then decide to modify, drop, or augment it.

**Events** identify something that happened. Concrete event types live in the contract package owned by their originating endpoint — `connection.TextLine`, `connection.Prompt`, `connection.GMCPFrame`, `ui.Input`, `ui.Resize`, and so on. Processors may also synthesise their own events to represent recognised patterns or derived state (e.g. a world processor emitting `SkillUsed`). All concrete events embed `app.EventMarker` to satisfy `app.Event`.

**Commands** identify something to be done. Concrete command types live in the contract package owned by the endpoint that applies them — `connection.Send`, `connection.Reconnect`, `ui.PrintLine`, `ui.SetHealth`, `ui.AddVital`, and so on. All concrete commands embed `app.CommandMarker`. The engine routes each command to the endpoint that owns its type at the end of every batch; processors never name a target explicitly.

A processor's job is to read events, append events and commands as it sees fit, and return the batch. Processors run in a fixed order in the chain; later processors see what earlier ones produced. The same chain handles both directions — there is no "inbound" vs "outbound" mode. A `ui.Input` event triggers some processor to emit a `connection.Send`; a `connection.TextLine` event triggers some processor to emit `ui.PrintLine`. Many processors react in only one direction; some legitimately span both.

## Processors

Every processor implements the same `Processor` signature. They differ along two independent axes — **source** (who wrote it and where it lives) and **phase** (when it runs in the chain).

### Source

**Generic processors** live in `processors/` and apply to any GMCP-capable MUD. They know the standard event/command vocabulary but nothing about any specific game's mechanics. Examples:

- **`Decode`** — parses `connection.GMCPFrame` events using `platform/gmcp` and emits typed message events (`gmcp.CharVitals`, `gmcp.CharName`, `gmcp.RoomInfo`, …).
- **`Render`** — translates the typed GMCP message events into baseline UI commands (`ui.SetHealth`, `ui.SetMana`, `ui.SetCharacter`, `ui.SetRoom`).
- **`Input`** — converts `ui.Input` events into `connection.Send` commands, handling separator splitting and repeat-on-prefix.
- **`Log`** — appends the event/command stream to a file.

**Game-specific processors** live in `worlds/<world>/` and know how a particular MUD works. They decode the game's GMCP extensions, recognise game-specific patterns in text output, maintain rich game state, and emit commands that go beyond the generic baseline (custom vitals, target tracking, learning, tunnel vision, balance timing, …).

**User scripts** are also processors. They live in the user's configuration directory and are loaded at startup in a deterministic order (filename or config).

All three sources implement the same `Processor` signature and slot into the same chain. The source axis is about ownership and lifecycle, not about runtime behaviour.

### Phase

The chain is split into three explicit phases. Within each phase, processors run in the order they appear in their slice — that listing order is the only sequencing primitive, no per-processor priority numbers.

- **Pre** — decoding, baseline rendering, state mutation, input translation, feature processors. Anything that should see clean events from the outside world before scripts get a chance to modify them.
- **Scripts** — user scripts. Read everything Pre produced; their output is what reaches Post and the endpoints.
- **Post** — logging, rate limiting, final dedupe or coalesce. Anything that should see the final decided state after scripts.

The world contributes `Pre()` and `Post()`; the engine inserts user scripts between them at composition time:

```go
// worlds/achaea/achaea.go
type World struct { state *State }

func New() *World { return &World{state: newState()} }

func (w *World) Pre() []app.Processor {
    return []app.Processor{
        processors.Decode(),                  // generic
        processors.Render(),                  // generic
        processors.Input(';'),                // generic
        w.state.GMCPDispatch(),               // game-specific
        features.Learning(w.state),           // game-specific
        features.TunnelVision(w.state),       // game-specific
    }
}

func (w *World) Post() []app.Processor {
    return []app.Processor{
        processors.Log(...),                  // generic
    }
}
```

Composition happens in the engine wiring:

```go
chain := app.Chain(
    append(append(world.Pre(), scripts...), world.Post()...)...,
)
```

Most processors land in Pre. Post starts thin — typically just logging — and grows only when something legitimately needs to act on what survived the script layer.

Scripts default to the `Scripts` phase. The rare script that needs to run earlier or later (a script wanting raw events before generic decoding, or one that finalises Send-command rate limiting) can opt into `Pre` or `Post` explicitly when registered. The 90% case is "this script reacts to processed events and adjusts the resulting commands" — it goes in `Scripts` without anyone having to think about ordering.

## The two ports: Connection and UI

The pipeline has two endpoints. They are asymmetric in their domain — one speaks a wire protocol, the other renders a terminal — but symmetric in their shape: each produces events from its outside world and applies commands directed at it. They are otherwise oblivious to anything else in the project.

### Connection

Represents the network endpoint. Its read path is a **tokeniser** — raw bytes are turned into typed events at the boundary, so no processor sees raw bytes. The concrete event types live in the `connection/` contract package:

- `connection.TextLine` — one paragraph of output text.
- `connection.Prompt` — a GA-terminated prompt line. (Today this logic is buried in `engine.go`.)
- `connection.TelnetCommand` — IAC sequences.
- `connection.GMCPFrame` — the raw payload of a GMCP subnegotiation envelope. The connection does *not* decode message types; that is the generic `Decode` processor's job, which keeps the connection ignorant of game-vocabulary GMCP messages.
- `connection.StateChanged` — connected, disconnected, errored.

Its apply path consumes commands from the same package:

- `connection.Send{Bytes}` — write to the wire.
- `connection.Reconnect`, `connection.Disconnect` — control operations.

Tokenisation is part of `platform/telnet/` — it is the connection's read implementation, not a separate package. The connection package handles only the wire envelope (`IAC SB GMCP ... IAC SE` framing); typed GMCP messages live in the sibling `platform/gmcp/` package.

`Connection` replaces the existing `pkg.Client` interface. The rename clarifies what the port is — "the connection to the server" — and avoids the MUD-vocabulary confusion where "client" usually refers to the whole application.

### UI

Represents the terminal endpoint. Its read path produces events from the `ui/` contract package:

- `ui.Input{Bytes}` — a line the user submitted (after pressing enter; UI buffers keystrokes locally).
- `ui.Resize`, future `ui.WidgetClicked`, etc.

Its apply path consumes commands from the same package:

- `ui.PrintLine{Text}` — append a line to the scrollback.
- `ui.SetHealth{Value, Max}`, `ui.SetMana{Value, Max}` — primary vitals.
- `ui.AddVital{Name, Value, Max}`, `ui.SetVital{Name, Value, Max}`, `ui.RemoveVital{Name}` — auxiliary, named vitals.
- `ui.SetCharacter{Name, Title}`, `ui.SetTarget{*ui.Target}`, `ui.SetRoom{*ui.Room}` — structured state.
- `ui.MaskInput`, `ui.UnmaskInput` — input echo control.

The UI is *declarative*. It knows nothing about game mechanics — what Ki or Karma means, whether a player has died, what a room's exits denote. It applies the commands it receives, and emits events for what the user does. World-specific knowledge is pushed entirely into the processors that produce the commands.

### Named widgets and idempotency

The UI's named-widget commands (`AddVital`, `SetVital`, `RemoveVital`) are **idempotent**. `AddVital{"ki", v, m}` means "ensure the Ki vital exists with these values"; emitting it every pass where the game state changes is fine. State is owned by the emitter (the Achaea processor), not by the UI; the UI is a renderer.

This eliminates the need for handles, callbacks, or lifecycle events for widget management. If a user script wants to suppress a vital permanently, it becomes a processor that drops `AddVital{"ki", …}` commands as they pass. Layered processors mean this composes naturally without requiring the game processor to "know" the widget was suppressed.

## Worlds and rich state

A world is the bundle of game-specific processors plus the rich state they consult. For Achaea, `worlds/achaea/state/` holds `Character` (with all the Achaea fields — vitals, balance, stances, Ki, Karma, etc.), `Target`, `Area`, and the methods that mutate them from typed GMCP events. The Achaea world is the only place that ever sees this rich state; commands going to the UI are the projections.

### Room duplication

`app.Room` (the perceived room: description, exits, items, present npcs) and `lib/navigation.Node` (the graph node: id, coordinates, edges) are intentionally separate types that share the name "room" only in conversation. The translation between them happens inside the world adapter, because the mapping is game-specific. Do not introduce a shared `Room` interface — the duplication carries information about which concern owns the data.

## Data flow

There is one chain, and it handles both directions. The phases (Pre → Scripts → Post) are the same; what differs between directions is which events arrive and which commands the chain produces.

Inbound (something arrived from the server):

```
socket bytes
  → platform/telnet
       (tokenise; emit connection.TextLine, connection.Prompt,
        connection.TelnetCommand, connection.GMCPFrame)
  → app/Engine wraps events into a Batch

  ── Pre phase ──
  → processors/Decode          (connection.GMCPFrame → typed GMCP message events)
  → processors/Render          (typed GMCP events → baseline ui.* commands)
  → worlds/<world>/Pre         (game-specific GMCP dispatch, state mutation,
                                feature processors)

  ── Scripts phase ──
  → user scripts               (may modify or originate any event or command)

  ── Post phase ──
  → worlds/<world>/Post        (typically just logging)

  → app/Engine drains:
       ui.* commands → ui port Apply
       connection.* commands → connection port Apply
```

Outbound (something happened in the UI):

```
user presses enter
  → platform/tui emits ui.Input event
  → app/Engine wraps event into a Batch

  ── Pre phase ──
  → processors/Input           (ui.Input → connection.Send; separator split;
                                repeat prefix)
  → worlds/<world>/Pre         (alias and macro expansion on connection.Send)

  ── Scripts phase ──
  → user scripts

  ── Post phase ──
  → worlds/<world>/Post        (logging; future rate limiting)

  → app/Engine drains:
       connection.* commands → connection port Apply (Send writes the bytes)
       ui.* commands → ui port Apply (e.g., a script asked to print a confirmation)
```

The same processor chain serves both. A processor that only cares about one direction reads only the events it recognises and emits only the commands it produces; processors that span both directions (the world's GMCP dispatch can react to server events and emit server commands) react in the order they appear in the chain.

## Migration order

The full sequence with per-step status is tracked in [`target-migration.md`](target-migration.md). The order is:

1. **Batch, Events, Commands, and processor rewrite.** Land `Batch`, `Event`, `Command`, and `Processor` in a new `app/` package; create the `connection/` and `ui/` contract packages with their concrete event and command types; rewrite every existing processor to the new signature and convert direct UI/Client method calls into appended commands. Worlds stop taking endpoint references in their constructors.
2. **Move main.go.** `main.go` → `cmd/nogfx/main.go`.
3. **Extract platform adapters.** `pkg/telnet/` → `platform/telnet/`; `pkg/gmcp/` → `platform/gmcp/`; `pkg/tui/` → `platform/tui/`. Telnet absorbs tokenisation and emits `connection.*` events; TUI emits `ui.Input` events. Land the `connection.Connection` and `ui.UI` port interfaces in their respective packages. Verify the zero-dependency endpoint property: `platform/telnet` imports only `app/`, `connection/`, `lib/*`; `platform/tui` imports only `app/`, `ui/`, `lib/*`.
4. **Libraries to `lib/`.** `pkg/simpex/` → `lib/simpex/`; `pkg/navigation/` → `lib/navigation/`.
5. **Generic processors to `processors/`.** Pull `Decode`, `Render`, `Input` out of `cmdprocess`; existing generic processors (split, repeat, log) move alongside them.
6. **Worlds.** `pkg/world/` → `worlds/`.
7. **Introduce Pre/Post phases.** Restructure world constructors to return `Pre()` and `Post()` slices; engine inserts user scripts between.

Step 1 is the largest mechanical change and must land before any package moves — otherwise every move triggers a signature conflict mid-flight. Step 3 ties tokenisation to the new event model; once it lands the engine becomes a thin pump. After step 1 worlds are already endpoint-free; the remaining steps move packages into their target locations.
