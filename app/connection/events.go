// Package connection holds the contract for the network endpoint: the events
// the connection emits and the effects it accepts. Concrete implementations
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

// Sent reports that a Send / SendGMCP effect was successfully written
// to the wire by the Connection. Effect carries the original effect;
// downstream tracking (processors/generic.Recorder) uses this as the
// authoritative "we actually sent this" signal rather than reading
// batch.Effects, which a later processor could mutate before Apply
// dispatches it.
type Sent struct {
	app.EventMarker
	Effect app.Effect
}

// Message is one turn's worth of server output: every TextLine and
// GMCPFrame received since the previous Prompt, plus the Prompt that
// closed the turn. Message is a *derived* event the Aggregator
// processor emits — the raw events still flow through the chain as
// today. Game-logic processors that want the turn as a unit (TunnelVision
// rewrite, illusion detection, misframing splitter) consume Message;
// processors that operate per-event (raw log, output renderer, GMCP
// state updaters) keep consuming the underlying events.
//
// Orphan GMCP frames — those arriving with no closing prompt — buffer
// inside the Aggregator until the next Prompt eventually fires, at
// which point they join that prompt's Message. See
// docs/design/messages.md for the full rationale.
type Message struct {
	app.EventMarker
	Lines  []TextLine
	GMCP   []GMCPFrame
	Prompt Prompt
}
