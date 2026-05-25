package connection

import "github.com/nogfx/nogfx/app"

// Payload immutability: Send.Bytes and SendGMCP.Payload are owned by the
// processor that constructed the effect. Once appended to a batch, neither
// the producer nor any downstream processor may mutate the underlying
// array. The engine and the Connection endpoint rely on this — they hand
// the same slice to writers and to derived Sent events without copying.
// A processor that needs to transform an effect appends a *new* effect
// (with a new slice) rather than editing in place.

// Send writes bytes to the wire. Bytes is read-only after Send is
// appended to a batch (see "Payload immutability" above).
type Send struct {
	app.EffectMarker
	Bytes []byte
}

// SendGMCP writes a GMCP message to the wire wrapped in the telnet
// subnegotiation envelope (IAC SB GMCP … IAC SE). Payload is the message
// body — e.g. `Char.Login { "name": "…", "password": "…" }` — without
// any IAC framing. Telnet adapters handle the envelope so callers don't
// have to know the wire bytes. Payload is read-only after SendGMCP is
// appended to a batch (see "Payload immutability" above).
type SendGMCP struct {
	app.EffectMarker
	Payload []byte
}

// Disconnect closes the connection. The endpoint stops emitting events
// and the engine shuts down once Run returns.
type Disconnect struct {
	app.EffectMarker
}
