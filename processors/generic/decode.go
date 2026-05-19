package generic

import (
	"errors"
	"log"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// DecodedGMCP is an event carrying a typed GMCP message that the Decode
// processor has parsed from a connection.GMCPFrame. Downstream processors
// type-switch on this event to react to specific messages (e.g. SetHealth
// on gmcp.CharVitals).
type DecodedGMCP struct {
	app.EventMarker
	Message gmcp.Message
}

// Decode parses a connection.GMCPFrame trigger into a typed message and
// appends a DecodedGMCP derived event. The derived event becomes its own
// batch downstream, where Render (or world-specific processors) can match
// on the message type.
//
// Unknown messages (gmcp.ErrUnknownMessage) are silently skipped.
//
// Worlds with their own enriched GMCP parsing (e.g. Achaea's agmcp.Parse)
// can either skip Decode and emit their own decoded events, or run their
// dispatch downstream of Decode using the generic message types.
func Decode() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		frame, ok := batch.Event.(connection.GMCPFrame)
		if !ok {
			return batch, nil
		}

		msg, err := gmcp.Parse(frame.Payload)
		if err != nil {
			if !errors.Is(err, gmcp.ErrUnknownMessage) {
				log.Printf("decode GMCP: %s", err)
			}

			return batch, nil
		}

		return batch.AppendEvent(DecodedGMCP{Message: msg}), nil
	}
}
