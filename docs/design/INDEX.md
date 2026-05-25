# Design

Decisions, rationale, conventions, and "things we tried." Architecture docs answer *where things live*; design docs answer *why things are the way they are*.

- [`simpex.md`](simpex.md) — the homegrown pattern language and when to use it.
- [`reformatting.md`](reformatting.md) — how output gets re-rendered after the policy that produced it changes (ReFormat effect, ReFormatting event, GuardedEvent contract).
- [`messages.md`](messages.md) — the `Message` event that aggregates a turn's lines, GMCP frames, and prompt. Dual delivery: per-frame state updates flow through immediately; the bundled `Message` carries the turn-level view for TunnelVision, illusion detection, and misframing splitters.
- [`tracking.md`](tracking.md) — remembering what MUD commands we've sent so we can match replies against them. Drives illusion detection, swallowed-command detection, and reorder detection. (GMCP lag measurement is intentionally *outside* the Tracker — see `tracking.md`.)
