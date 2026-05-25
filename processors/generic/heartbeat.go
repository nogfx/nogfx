package generic

import (
	"bytes"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/clock"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// gmcpHeartbeat returns a processor that appends a SendGMCP carrying
// msg every time interval has elapsed since the previous send. It
// latches off the connection.Sent event TelnetNegotiation emits when
// we *accept* the server's WILL GMCP (Send{Bytes: IAC DO GMCP}) — so
// no SendGMCP envelope hits the wire before we've actually agreed to
// GMCP. A NegotiationPolicy that declines GMCP produces an IAC DONT
// GMCP instead, no IAC DO GMCP Sent event fires, and the heartbeat
// stays dormant. An interval of zero disables sending; the returned
// processor is a pass-through.
//
// The processor is correlation-agnostic: tracking, matching, and lag
// emission live downstream in Tracker and LagWatcher.
func gmcpHeartbeat(interval time.Duration, msg gmcp.Message) app.Processor {
	payload := []byte(msg.Marshal())

	var (
		lastSent  time.Time
		gmcpReady bool
	)

	return func(batch app.Batch) (app.Batch, error) {
		if !gmcpReady {
			if sent, ok := batch.Event.(connection.Sent); ok {
				if send, ok := sent.Effect.(connection.Send); ok &&
					bytes.Equal(send.Bytes, connection.IACDoGMCP) {
					gmcpReady = true
				}
			}
		}

		tick, ok := batch.Event.(clock.Tick)
		if !ok {
			return batch, nil
		}

		if interval <= 0 || !gmcpReady {
			return batch, nil
		}

		if !lastSent.IsZero() && tick.At.Sub(lastSent) < interval {
			return batch, nil
		}

		lastSent = tick.At

		// payload is shared across every emission of this heartbeat.
		// connection.SendGMCP.Payload is read-only post-append (see the
		// "Payload immutability" doc on the type), so sharing is safe.
		return batch.AppendEffect(connection.SendGMCP{Payload: payload}), nil
	}
}
