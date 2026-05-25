package connection

import (
	"context"

	"github.com/nogfx/nogfx/app"
)

// Connection is the port for the network endpoint. An implementation runs in
// its own goroutine, tokenises bytes off the wire into typed events that it
// pushes to the events channel, and applies effects the engine sends to it
// via Apply.
//
// Implementations live in platform/ (e.g. platform/telnet). Worlds and
// processors do not depend on implementations — they communicate only
// through this port and the events/effects defined in this package.
//
// The MUD-domain "command" (a line of text the player would type) is the
// payload of connection.Send.Bytes — an Effect carrying a Command on the
// wire. See docs/design/tracking.md for how the two terms relate.
type Connection interface {
	// Run reads from the underlying transport and emits events onto the
	// given channel until ctx is cancelled or the transport closes.
	Run(ctx context.Context, events chan<- app.Event) error

	// Apply executes a single effect against the connection. Returns
	// app.ErrEffectNotApplicable if the effect does not target this
	// endpoint; the engine uses that to route the effect to the next
	// candidate. May return apply-consequence events (e.g.
	// connection.Sent after a successful Send / SendGMCP) that the
	// engine flows through the chain like processor-derived events.
	Apply(eff app.Effect) ([]app.Event, error)
}
