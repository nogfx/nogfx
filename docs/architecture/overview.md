# Architecture overview

The system is organised around an **Engine** (`pkg/engine.go`) that orchestrates three collaborators wired together in `main.go`:

1. **Client** (`pkg.Client`, implemented by `pkg/telnet`) — the network connection. It handles telnet NVT and negotiation, exposes `io.ReadWriter` plus helpers (`Will/Wont/Do/Dont/Subneg`), and provides a `bufio.SplitFunc` for the engine's scanner. GMCP (`pkg/gmcp`) is layered on top of telnet subnegotiation.
2. **UI** (`pkg.UI`, implemented by `pkg/tui`) — the tcell-based TUI. It owns the input/output channels (`Inputs() <-chan []byte`, `Outputs() chan<- []byte`) and exposes setters for character/room/target state. The TUI is composed of panes via `Layout` (`pkg/tui/layout.go`), with per-pane caching keyed by name (`paneOutput`, `paneVitals`, `paneTarget`, `paneMap`).
3. **Processor** (`pkg/process/process.go`) — a chain of functions composed with `ChainProcessor`. Each layer can rewrite, drop, or inject lines in either direction.

Data flows bidirectionally through the Processor chain: keystrokes from the TUI become `ins`, bytes from the server become `outs`, and one tick processes a batch of each before dispatching back out to the Client (writes) and the UI (`Outputs()` channel).

## Runtime layout

User-facing files live under `~/nogfx/` (the value of `pkg.Directory`, established in `pkg/variables.go`). New persistent state should land there, not in cwd. Per-session game logs end up in `~/nogfx/logs/` via `process.LogProcessor`.

## World-specific logic

World logic lives under `pkg/world/<world>/`. `pkg/world/achaea/achaea.go::Processor` is the canonical entry point and shows the pattern:

1. Construct a `world` struct holding the `Client`, `UI`, and game-state objects (`Character`, `Target`, `Room`).
2. Return a `ChainProcessor` composed of generic processors (raw log, split input, repeat input, output rewrites), the world's own `cmdprocess` (telnet negotiation + GMCP dispatch), feature-specific processors (`Learning`, `TunnelVision`, future bashing), and a final log.

`cmdprocess` is the single place where GMCP messages are dispatched onto the world's state objects and where the UI is notified via `ui.SetCharacter` / `SetTarget` / `SetRoom` to trigger repaints. To add a GMCP-driven feature, add a case to the type switch in `cmdprocess` and update the relevant world struct.

## Processor signature — note

There are two processor shapes in the codebase right now:

- The current one in `pkg/process/process.go`: `func(ins, outs [][]byte) (postins, postouts [][]byte, err error)`.
- The shape the world processors expect: `process.ProcessorFunc(world.cmdprocess)` where `cmdprocess` takes and returns `pkg.Inoutput`.

The `Inoutput` type is the direction the refactor is heading. See [`../design/refactor-status.md`](../design/refactor-status.md) for the details.

## Pattern matching

`pkg/simpex` is the homegrown pattern language used for triggers and line parsers throughout the world layer. Processors compose it through `process.MatchInput` / `process.MatchOutput` in `pkg/process/match.go`. See [`../design/simpex.md`](../design/simpex.md) for syntax and when to prefer it over `regexp`.
