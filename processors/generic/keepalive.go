package generic

import (
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// KeepAlive returns a processor that sends Core.KeepAlive on cadence
// to reset the server's idle timer. Per the GMCP spec Core.KeepAlive
// is one-way: Iron Realms servers reset their timer but do not echo.
// On servers that do echo, LagWatcher (not the Tracker — GMCP traffic
// stays out of the MUD-command queue) correlates the reply via its
// per-message-ID FIFO and emits a lag measurement.
//
// The processor latches off the Sent event for our IAC DO GMCP reply
// so no SendGMCP envelope hits the wire before GMCP has actually been
// agreed to in both directions.
//
// An interval of zero disables sending; the returned processor is a
// pass-through.
func KeepAlive(interval time.Duration) app.Processor {
	return gmcpHeartbeat(interval, &gmcp.CoreKeepAlive{})
}
