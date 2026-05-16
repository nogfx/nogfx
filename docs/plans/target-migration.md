# Target architecture migration

Tracks execution of the package-layout and pipeline-model refactor described in [`target-architecture.md`](target-architecture.md). Each step below corresponds to a phase of that work.

## Update protocol

- Update a step's **Status** when work on it starts and when it lands.
- When a step lands as a PR, link the PR under that step.
- When a step uncovers a decision, surprise, or deviation from the target architecture, capture it under **Notes**. If a deviation should persist, also update `target-architecture.md` so the target document continues to reflect the intended end state.
- Steps should land in order. Step 1 in particular is a prerequisite for every later step — until the new processor signature is in place, every package move triggers a signature conflict.

## Step 1 — Batch, Events, Commands, and processor rewrite

**Status:** done (with Learning/TunnelVision/Bashing reimplementations tracked as separate follow-ups, see below)

Land `Batch`, `Event`, `Command`, and `Processor` in a new `app/` package as abstract pipeline primitives. Land concrete event and command types in sibling contract packages `connection/` and `ui/`, so that `app/` stays endpoint-agnostic and the two endpoints stay decoupled from each other.

Rewrite every existing processor in place to the new signature **and** convert every direct UI/Client method call into an appended command. After this step, worlds no longer take `ui` or `client` references in their constructors — they emit commands that the engine routes to the right endpoint.

This is the largest mechanical change and must land first; later package moves would otherwise have to bridge two processor signatures simultaneously. The signature change and the port-stripping are merged because doing them separately creates an awkward intermediate where processors have new signatures but still hold endpoint references.

**Scope checklist:**
- [x] `app/batch.go` (Batch + AppendEvent/AppendCommand), `app/processor.go` (Processor, Chain), `app/event.go` (Event interface + EventMarker), `app/command.go` (Command interface + CommandMarker) defined and compiling.
- [x] `connection/events.go` (TextLine, Prompt, TelnetCommand, GMCPFrame, StateChanged) and `connection/commands.go` (Send, Reconnect, Disconnect) defined and compiling.
- [x] `ui/events.go` (Input, Resize), `ui/commands.go` (PrintLine, SetHealth, SetMana, AddVital, SetVital, RemoveVital, SetCharacter, SetTarget, SetRoom, MaskInput, UnmaskInput), and `ui/snapshots.go` (Target, Room) defined and compiling.
- [ ] `connection.Connection` and `ui.UI` port interfaces (deferred to step 3 alongside the platform-adapter extraction; port and implementation should land together).
- [x] `pkg/process/process.go` Processor signature replaced — it now re-exports `app.Processor` and `app.Chain`.
- [x] Simple generic processors converted: `process_splitinput`, `process_repeatinput`, `process_logger`, `match`. They operate on `app.Batch`, reading `connection.TextLine` / `ui.Input` events and emitting `connection.Send` / `ui.PrintLine` commands.
- [ ] **Tag-based Achaea processors disabled with stubs** — `Learning`, `TunnelVision`, and `Bashing` are temporarily no-ops. The procs/ files were already commented-out scratch notes. These need proper redesigns against events/commands (not mechanical conversion) before being re-enabled. See "Deferred to follow-up" below.
- [x] `pkg/engine.go` updated to pump batches through the chain. The engine tokenises socket bytes into `connection.TextLine` / `connection.Prompt` / `connection.TelnetCommand` events, wraps each pass into a Batch, runs the chain, and dispatches commands by type-switch to the existing `pkg.Client` / `pkg.UI` implementations.
- [x] `pkg/services.go` Client/UI interfaces adjusted. Client is now just `net.Conn` + `SplitFunc`; the `Send`/`Will`/`Wont`/`Do`/`Dont`/`Subneg` methods that nothing implemented are removed. UI loses the typed setters (`SetCharacter`/`SetRoom`/`SetTarget`); engine routes the corresponding commands by type-switch (currently as no-ops; real wiring lands in step 3).
- [x] `world/achaea/Processor()` constructed without using the passed Client/UI references. The arguments are retained on the signature for source compatibility with `main.go` and will be removed in step 7. The `Target.Set` callback queues `settarget` bytes via `DrainSends` rather than calling `client.Send` directly.
- [x] `main.go` continues to compile against the existing signature — `telnet.Dial` reintroduced (it had gone missing).
- [x] **Production build is green** — `go build ./...` succeeds.
- [x] Test files updated. `pkg/process/process_splitinput_test.go` and `process_repeatinput_test.go` converted to the new signature; `pkg/world/achaea/target_test.go` updated to use the new `NewTarget()` and `DrainSends()` API; `pkg/engine_test.go`, `pkg/world/achaea/achaea_test.go`, and `pkg/world/achaea/learning_test.go` deleted — they tested fictional APIs (`pkg.NewEngine`, `achaea.NewWorld`, `pkg/testing` helpers) that never landed. `go test ./...` now compiles across every package.

**Notes:**
- **Production build is green.** `go build ./...` succeeds; the codebase has emerged from the half-finished Inoutput rescue with a coherent foundation.
- **Step 1 foundation landed.** `app/`, `connection/`, and `ui/` packages defined and compiling. Concrete events and commands live with their endpoints; `app/` only carries the abstract pipeline core (Batch + Event/Command interfaces + Processor/Chain).
- **Pipeline renamed to Batch.** The unit-of-work nature is now explicit — a batch starts with one or a few triggering events, accumulates events and commands as processors run, and carries that full context to scripts and to the endpoints.
- **`Tags` dropped.** Most uses become typed event types in the new model (e.g. "this is a prompt" is `connection.Prompt`, not a tagged `TextLine`). PrintLine carries pre-styled bytes (ANSI codes inline) rather than separate styling Tags.

**Test status:**
- All packages pass `go test ./...`. Pre-existing failures have been fixed: `cmd/nogfx::TestParseAddress` (parser now defaults empty input to `example.com:23`, returns `"invalid address 'X'"` for malformed host:port, and `"invalid port 'X'"` for non-numeric ports) and `platform/gmcp*::TestParse/non-existant` (`gmcp.Parse` now returns `ErrUnknownMessage` wrapped with the message ID; `processors.Decode` and Achaea's `dispatchGMCP` use `errors.Is` to silently skip unknown messages rather than log them).
- Coverage of the new packages: `app/` has 6 tests for `Batch`, `Chain`, marker types, and `ErrCommandNotApplicable`; `connection/` and `ui/` have interface-satisfaction tests guarding the marker embeds; `processors/` has tests for the new `Input`, `Output`, `Decode`, and `Render` processors (alongside the converted `Split`/`Repeat`/`Match` ones); `pkg/` has engine tests covering both endpoints' command routing, unknown-command tolerance, and connection-error propagation; `worlds/achaea/` covers `Learning`, `Bashing`, and `TunnelVision` headline behaviours.

**Deferred follow-ups now resolved:**
- **Learning, TunnelVision, and Bashing have been re-implemented** against the event/command model. Each processor walks `batch.Events` for `connection.TextLine` patterns and `batch.Commands` for `connection.Send` patterns, classifies them via `lib/simpex`, and rewrites or appends accordingly. Learning chains "learn N X from Y" sessions into 15-lesson chunks with a progress-line replacement; Bashing expands "kill" into the queued attack sequence and clears the equilibrium on slay; TunnelVision omits balance/weather spam, suppresses paired curing/cured sequences, and consolidates multi-line attack flurries into a single coloured summary. Each has its own `*_test.go` covering the headline state transitions.
- **UI command dispatch wiring landed in step 3** — `platform/tui.TUI.Apply` handles the full 11-command surface (`PrintLine`, `SetHealth`, `SetMana`, `AddVital`, `SetVital`, `RemoveVital`, `SetCharacter`, `SetTarget`, `SetRoom`, `MaskInput`, `UnmaskInput`).

**Historical context (rescue scope):**
- The codebase was mid-refactor when step 1 began and did not build. A prior in-flight refactor introduced an `Inoutput` type in `pkg/inoutput.go` and converted some processors (notably `learning.go` and `cmdprocess` in `achaea.go`) to use `pkg.Inoutput`, `pkg.Match`, `pkg.MatchInput`, `pkg.Callback`, etc. Those `pkg`-level types were never landed — `pkg/inoutput.go` was entirely commented out, and `pkg/mock/pkg_client.go` was empty. The new `app.Batch`/`Event`/`Command` design supersedes the abandoned `Inoutput` direction; bringing the build green required converting both the original `func(ins, outs [][]byte)`-style processors and the half-migrated `Inoutput`-style processors to the new `app.Processor` signature.

## Step 2 — Move main.go

**Status:** done

`main.go` → `cmd/nogfx/main.go`. Update Makefile and goreleaser.

**Notes:**
- `main.go` and `main_test.go` moved into `cmd/nogfx/`.
- `.goreleaser.yml` updated: `builds[0].main: ./cmd/nogfx` and explicit `binary: nogfx`.
- `Makefile` simplified: previous targets regenerated mocks for types that have been retired (`pkg.Client`, `pkg.UI`, `pkg.Processor`) and referenced a non-existent `pkg/procs.go`. New Makefile has `build`/`test` targets plus a `mocks` target that only handles `tcell.Screen` (the one mock that survived step 1). The Client/UI mocks come back when the proper `connection.Connection` / `ui.UI` ports land in step 3.
- `go build ./...` and `go build ./cmd/nogfx` both succeed; the binary runs and prints its usage line as expected.

## Step 3 — Extract platform adapters

**Status:** done

`pkg/telnet/` → `platform/telnet/`; `pkg/gmcp/` → `platform/gmcp/`; `pkg/tui/` → `platform/tui/`. Telnet absorbs the tokenisation logic currently in `engine.go` and starts emitting `connection.*` events. The TUI starts emitting `ui.Input` events. Land the `connection.Connection` and `ui.UI` port interfaces in their respective packages alongside their implementations. Rename `Client` → `Connection` throughout.

Verify the zero-dependency endpoint property: `platform/telnet` imports only `app/`, `connection/`, `lib/*`; `platform/tui` imports only `app/`, `ui/`, `lib/*`. Neither imports the other or `processors/`/`worlds/`.

**Scope checklist:**
- [x] `pkg/telnet/` → `platform/telnet/`, `pkg/gmcp/` → `platform/gmcp/`, `pkg/tui/` → `platform/tui/`. All imports updated; `go build ./...` green.
- [x] `connection.Connection` port interface defined in `connection/connection.go`.
- [x] `ui.UI` port interface defined in `ui/ui.go`.
- [x] `app.ErrCommandNotApplicable` defined for endpoints to signal "not my command" without it being treated as a real error.
- [x] `platform/telnet.NVT` implements `connection.Connection` — `Run(ctx, events)` tokenises socket bytes into `connection.TextLine`/`Prompt`/`TelnetCommand` events; `Apply(cmd)` handles `connection.Send` (with `Reconnect`/`Disconnect` stubbed). Verified by a `var _ connection.Connection = (*telnet.NVT)(nil)` check.
- [x] `platform/tui.TUI` implements `ui.UI` — `Run(ctx, events)` emits `ui.Input`/`ui.Resize` events; `Apply(cmd)` dispatches `PrintLine`/`SetHealth`/`SetMana`/`AddVital`/`SetVital`/`RemoveVital`/`SetCharacter`/`SetTarget`/`SetRoom`/`MaskInput`/`UnmaskInput` against TUI state. The TUI's internal state was rewritten: no more `pkg.Character`/`pkg.Target` indirection; vitals are tracked individually (`health`, `mana`, named map) and `SetRoom` carries a `*navigation.Room` directly so the minimap stays functional.
- [x] Engine rewired to use `connection.Connection` and `ui.UI` directly. `pkg/engine.go` is now ~90 lines: it runs both endpoints in their own goroutines, reads from a shared events channel, processes each event through the chain, and dispatches each resulting command by attempting `Connection.Apply` then `UI.Apply` (`ErrCommandNotApplicable` signals "try the next").
- [x] Worlds drop endpoint references in their constructor signatures: `achaea.Processor()` now takes no arguments.
- [x] Zero-dependency property verified: production code in `platform/telnet` imports only `app`, `connection`, `lib/*` (test files import `pkg/mock` which is local to step-1 cleanup); production code in `platform/tui` imports only `app`, `ui`, `lib/navigation`.
- [x] `pkg/services.go` removed — `pkg.Client` and `pkg.UI` were the transitional indirection and have no remaining callers.

**Notes:**
- The TUI refactor was the largest piece of step 3. The new TUI maintains its own vitals state (instead of receiving a whole `pkg.Character` via `SetCharacter`), uses a `drawCh` to request redraws from any goroutine (since Apply runs on the engine's goroutine but the screen must be drawn from Run's goroutine), and emits `ui.Input` events via a small `emitEvent` helper.
- `ui.Target` gained a `Queue` field so the target pane can still render the " (+N)" suffix without needing the rich `pkg.Target.Queue()` method.
- `ui.SetRoom` carries `*navigation.Room` directly. `ui/` imports `lib/navigation` — this is a tolerated downstream dependency since the minimap renders the navigation graph; the alternative (defining a UI-specific MapNode type that mirrors the graph) is more elaborate without buying real decoupling.
- Two test files were deleted or trimmed during this step: `platform/tui/vitals_test.go` (tested `RenderVitals` via the deleted `SetCharacter(pkg.Character)` setter); `platform/tui/event_test.go` and `minimap_test.go` were updated to use the new event/command surface.

## Step 4 — Libraries to lib/

**Status:** done

`pkg/simpex/` → `lib/simpex/`; `pkg/navigation/` → `lib/navigation/`.

**Notes:**
- Both packages relocated; all imports updated via `sed`. Production build green; affected tests (`lib/simpex`, callers in `pkg/process`) still pass.
- Done out of order with step 3 because the TUI rendering needs `lib/navigation` for the minimap; doing this move first means the TUI refactor can import the final path.

## Step 5 — Generic processors to processors/

**Status:** done

Pull the generic GMCP dispatch out of the world's `cmdprocess` into `processors/Decode` and `processors/Render`. Pull input handling into `processors/Input`. Existing generic processors (split, repeat, log) move alongside them.

**Notes:**
- `pkg/process/` → top-level `processors/` (package renamed `process` → `processors`). All callers updated.
- New `processors.Input()` converts `ui.Input` events into `connection.Send` commands — this was a real bug: nothing had been doing the translation, so user keystrokes never reached the wire under the new pipeline.
- New `processors.Decode()` parses `connection.GMCPFrame` events using `platform/gmcp.Parse` and appends a `DecodedGMCP{Message: ...}` event carrying the typed message.
- New `processors.Render()` translates baseline `DecodedGMCP` events into UI commands. Today it handles `gmcp.CharName` → `ui.SetCharacter` and `gmcp.RoomInfo` → `ui.SetRoom` (constructed via `lib/navigation.RoomFromGMCP`). The generic GMCP vocabulary doesn't define a vitals message, so health/mana commands are world-emitted (Achaea uses its richer `agmcp.CharVitals`).
- Achaea continues to use its own GMCP dispatch (via `agmcp.Parse`) for the rich Achaea-specific message types. `processors.Render` is preparation for future "generic-world" use and as a baseline reference; Achaea could adopt it for the parts that align, but the integration is left as a future cleanup.
- `processors.ChainProcessor` removed (callers use `app.Chain` directly). The `processors.Processor` alias for `app.Processor` kept as a convenience.

## Step 6 — Worlds

**Status:** done

`pkg/world/` → `worlds/`. Achaea-specific dispatch stays in `worlds/achaea/`.

**Notes:**
- Single-command move (`mv pkg/world worlds`) plus `sed` import-path update. Production code green, Achaea tests pass.

## Step 7 — Introduce Pre/Post phases

**Status:** done

Restructure each world's constructor to return `Pre()` and `Post()` slices. The engine composes the final chain by inserting the user-scripts slice between them. Once this lands, adding user scripting is a matter of populating the middle slot.

**Notes:**
- Achaea's `World` type now exposes `Pre()` (raw-log, input translation, command splitting, repeat-on-prefix, output rewrite, GMCP dispatch, Learning, TunnelVision) and `Post()` (the processed log). User scripts will slot into a `World.Chain(scripts...)` call between them.
- `achaea.Processor()` is retained as a back-compat shim that returns the full Pre+Post chain with no scripts inserted; `cmd/nogfx/main.go` still uses it. Loading user scripts becomes a separate concern with a clear insertion point.
- The phase structure means adding user scripts is a single composition change (call `World.Chain(scripts...)` instead of `Processor()`), with no impact on the world's internal layering.
