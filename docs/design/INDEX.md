# Design

Decisions, rationale, conventions, and "things we tried." Architecture docs answer *where things live*; design docs answer *why things are the way they are*.

- [`simpex.md`](simpex.md) — the homegrown pattern language and when to use it.
- [`reformatting.md`](reformatting.md) — how output gets re-rendered after the policy that produced it changes (ReFormat command, ReFormatting event, GuardedEvent contract).
- [`messages.md`](messages.md) — the `Message` event that aggregates a turn's lines, GMCP frames, and prompt. Dual delivery: per-frame state updates flow through immediately; the bundled `Message` carries the turn-level view for TunnelVision, illusion detection, and misframing splitters.
