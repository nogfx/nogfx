package connection

import (
	"context"

	"github.com/nogfx/nogfx/app"
)

// Connection is the port for the network endpoint. An implementation runs in
// its own goroutine, tokenises bytes off the wire into typed events that it
// pushes to the events channel, and applies commands the engine sends to it
// via Apply.
//
// Implementations live in platform/ (e.g. platform/telnet). Worlds and
// processors do not depend on implementations — they communicate only
// through this port and the events/commands defined in this package.
type Connection interface {
	// Run reads from the underlying transport and emits events onto the
	// given channel until ctx is cancelled or the transport closes.
	Run(ctx context.Context, events chan<- app.Event) error

	// Apply executes a single command against the connection. Returns
	// app.ErrCommandNotApplicable if the command does not target this
	// endpoint; the engine uses that to route the command to the next
	// candidate.
	Apply(cmd app.Command) error
}
