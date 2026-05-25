package ui

import (
	"context"

	"github.com/nogfx/nogfx/app"
)

// UI is the port for the user-facing endpoint. An implementation runs in its
// own goroutine, emits events (user input, resize, …) onto the events
// channel, and applies effects the engine sends to it via Apply.
//
// Implementations live in platform/ (e.g. platform/tui). Worlds and
// processors do not depend on implementations — they communicate only
// through this port and the events/effects defined in this package.
type UI interface {
	// Run starts the user interface, blocking until ctx is cancelled or the
	// UI exits. Events the user generates (input lines, resizes) are
	// pushed onto the given channel.
	Run(ctx context.Context, events chan<- app.Event) error

	// Apply executes a single effect against the UI. Returns
	// app.ErrEffectNotApplicable if the effect does not target this
	// endpoint; the engine uses that to route the effect to the next
	// candidate. May return apply-consequence events the engine flows
	// through the chain like processor-derived events.
	Apply(eff app.Effect) ([]app.Event, error)
}
