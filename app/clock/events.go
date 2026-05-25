// Package clock holds the contract for the clock endpoint: a time signal
// that lets processors do periodic work without owning a goroutine. The
// concrete ticker implementation lives in platform/clock.
package clock

import (
	"time"

	"github.com/nogfx/nogfx/app"
)

// Tick is a periodic time signal. Processors that need to do work on a
// cadence (keep-alive, lag visualisation, expiry sweeps) react to Tick
// events; the engine routes them through the chain like any other event.
//
// At carries the moment the tick fired, so processors can compare against
// their own last-action timestamp without calling time.Now themselves —
// useful for both wall-clock-independent tests and consistent ordering.
type Tick struct {
	app.EventMarker
	At time.Time
}
