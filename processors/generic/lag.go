package generic

import (
	"errors"
	"log"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// LagMeasured is a derived event carrying the round-trip time of a
// latency-bearing GMCP heartbeat (Core.Ping today; Core.KeepAlive on
// servers that echo it). LagWatcher is the producer.
type LagMeasured struct {
	app.EventMarker
	Lag time.Duration
	At  time.Time
}

// LagWatcher returns a processor that measures GMCP round-trip
// latency entirely on its own — without going through the Tracker,
// which is reserved for MUD commands (plain text), not protocol
// frames. LagWatcher subscribes to:
//
//   - connection.Sent events whose Effect is a latency-bearing GMCP
//     send (Core.Ping or Core.KeepAlive). It pushes time.Now() onto
//     a per-message-ID FIFO.
//   - connection.GMCPFrame events whose payload parses to a
//     latency-bearing GMCP message. It pops the oldest entry from
//     the matching FIFO, computes the elapsed duration, emits
//     LagMeasured, and dispatches ui.SetLag.
//
// Per-message-ID FIFOs (rather than one shared FIFO) matter because
// Core.Ping and Core.KeepAlive interleave on the wire but only
// Core.Ping reliably gets a reply on Iron Realms — a shared FIFO
// would let a Ping reply pop a stale KeepAlive timestamp and report
// 30-second lag.
func LagWatcher() app.Processor {
	return lagWatcher(time.Now)
}

// lagWatcher is the constructor with an injectable clock. The public
// LagWatcher passes time.Now; tests pass a deterministic clock to
// avoid wall-clock sleeps. Mirrors the pluggable-clock pattern in
// platform/clock.Ticker.
func lagWatcher(now func() time.Time) app.Processor {
	fifos := make(map[string][]time.Time)

	return func(batch app.Batch) (app.Batch, error) {
		switch ev := batch.Event.(type) {
		case connection.Sent:
			id, ok := lagBearingGMCPID(ev.Effect)
			if !ok {
				return batch, nil
			}

			fifos[id] = append(fifos[id], now())

		case connection.GMCPFrame:
			msg, err := gmcp.Parse(ev.Payload)
			if err != nil {
				if !errors.Is(err, gmcp.ErrUnknownMessage) {
					log.Printf("lag watcher: parse reply: %s", err)
				}

				return batch, nil
			}

			id, ok := lagBearingReplyID(msg)
			if !ok {
				return batch, nil
			}

			list := fifos[id]
			if len(list) == 0 {
				return batch, nil
			}

			sentAt := list[0]
			// Reallocate rather than re-slicing list[1:]: a plain reslice
			// pins the original backing array, so over a long session the
			// underlying capacity grows without bound even though len
			// stays small. The trade is one allocation per reply for a
			// stable heap profile.
			fifos[id] = append([]time.Time(nil), list[1:]...)

			at := now()
			lag := at.Sub(sentAt)

			batch = batch.AppendEvent(LagMeasured{Lag: lag, At: at})
			batch = batch.AppendEffect(ui.SetLag{Lag: lag})
		}

		return batch, nil
	}
}

// lagBearingGMCPID returns the GMCP message ID of eff if it's a
// latency-bearing outbound heartbeat (Core.Ping or Core.KeepAlive).
// The ok return distinguishes "tracked, ID is X" from "not tracked".
func lagBearingGMCPID(eff app.Effect) (string, bool) {
	sg, ok := eff.(connection.SendGMCP)
	if !ok {
		return "", false
	}

	msg, err := gmcp.Parse(sg.Payload)
	if err != nil {
		return "", false
	}

	return lagBearingReplyID(msg)
}

// lagBearingReplyID classifies an already-parsed GMCP message against
// the latency-bearing set. Sharing the classifier between the outbound
// (Sent) and inbound (GMCPFrame) paths keeps the set in one place.
func lagBearingReplyID(msg gmcp.Message) (string, bool) {
	switch msg.(type) {
	case *gmcp.CorePing, *gmcp.CoreKeepAlive:
		return msg.ID(), true
	}

	return "", false
}
