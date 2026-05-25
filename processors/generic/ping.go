package generic

import (
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// Ping returns a processor that sends Core.Ping on cadence. Core.Ping
// is the bidirectional GMCP probe explicitly defined for latency
// measurement; LagWatcher correlates the reply directly against the
// connection.Sent / connection.GMCPFrame stream (per-message-ID FIFO),
// emits LagMeasured, and dispatches ui.SetLag. The Tracker is not
// involved — GMCP heartbeats are protocol traffic, not MUD commands.
// See docs/design/tracking.md for the full picture.
//
// The processor latches off the Sent event for our IAC DO GMCP reply
// so no SendGMCP envelope hits the wire before GMCP has actually been
// agreed to in both directions.
//
// An interval of zero disables sending; the returned processor is a
// pass-through.
func Ping(interval time.Duration) app.Processor {
	return gmcpHeartbeat(interval, &gmcp.CorePing{})
}
