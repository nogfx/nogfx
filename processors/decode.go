package processors

import (
	"errors"
	"log"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// DecodedGMCP is an event carrying a typed GMCP message that the Decode
// processor has parsed from a connection.GMCPFrame. Downstream processors
// type-switch on Message to react to specific messages (e.g. SetHealth on
// gmcp.CharVitals).
type DecodedGMCP struct {
	app.EventMarker
	Message gmcp.Message
}

// Decode reads connection.GMCPFrame events from the batch and appends
// DecodedGMCP events carrying the parsed typed message. Unknown messages
// are skipped (gmcp.Parse currently returns nil/nil for unknown payloads).
//
// Worlds with their own enriched GMCP parsing (e.g. Achaea's agmcp.Parse)
// can either skip Decode and emit their own decoded events, or run their
// dispatch downstream of Decode using the generic message types.
func Decode() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		for _, ev := range batch.Events {
			frame, ok := ev.(connection.GMCPFrame)
			if !ok {
				continue
			}
			msg, err := gmcp.Parse(frame.Payload)
			if err != nil {
				if !errors.Is(err, gmcp.ErrUnknownMessage) {
					log.Printf("decode GMCP: %s", err)
				}
				continue
			}
			batch = batch.AppendEvent(DecodedGMCP{Message: msg})
		}
		return batch, nil
	}
}
