package ui

import (
	"context"

	"github.com/tobiassjosten/nogfx/app"
)

// UI is the port for the user-facing endpoint. An implementation runs in its
// own goroutine, emits events (user input, resize, …) onto the events
// channel, and applies commands the engine sends to it via Apply.
//
// Implementations live in platform/ (e.g. platform/tui). Worlds and
// processors do not depend on implementations — they communicate only
// through this port and the events/commands defined in this package.
type UI interface {
	// Run starts the user interface, blocking until ctx is cancelled or the
	// UI exits. Events the user generates (input lines, resizes) are
	// pushed onto the given channel.
	Run(ctx context.Context, events chan<- app.Event) error

	// Apply executes a single command against the UI. Returns
	// app.ErrCommandNotApplicable if the command does not target this
	// endpoint; the engine uses that to route the command to the next
	// candidate.
	Apply(cmd app.Command) error
}
