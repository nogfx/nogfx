// Package connection holds the contract for the network endpoint: the events
// the connection emits and the commands it accepts. Concrete implementations
// of the connection (e.g. telnet) live in platform/.
package connection

import "github.com/nogfx/nogfx/app"

// TextLine is one paragraph of output text received from the server.
type TextLine struct {
	app.EventMarker
	Bytes []byte
}

// Prompt is a GA-terminated prompt line received from the server.
type Prompt struct {
	app.EventMarker
	Bytes []byte
}

// TelnetCommand is a telnet IAC sequence received from the server.
type TelnetCommand struct {
	app.EventMarker
	Bytes []byte
}

// GMCPFrame is the raw payload of a GMCP subnegotiation envelope received
// from the server. The first whitespace-separated token is the GMCP module
// and message name; the rest is the JSON payload. Typed decoding into
// specific message types is a processor's job, not the connection's.
type GMCPFrame struct {
	app.EventMarker
	Payload []byte
}

// StateChanged signals that the connection's link state has changed.
type StateChanged struct {
	app.EventMarker
	Connected bool
	Err       error
}
