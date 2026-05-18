package connection

import "github.com/nogfx/nogfx/app"

// Send writes bytes to the wire.
type Send struct {
	app.CommandMarker
	Bytes []byte
}

// SendGMCP writes a GMCP message to the wire wrapped in the telnet
// subnegotiation envelope (IAC SB GMCP … IAC SE). Payload is the message
// body — e.g. `Char.Login { "name": "…", "password": "…" }` — without
// any IAC framing. Telnet adapters handle the envelope so callers don't
// have to know the wire bytes.
type SendGMCP struct {
	app.CommandMarker
	Payload []byte
}

// Reconnect asks the connection to drop and re-establish the link.
type Reconnect struct {
	app.CommandMarker
}

// Disconnect closes the connection.
type Disconnect struct {
	app.CommandMarker
}
