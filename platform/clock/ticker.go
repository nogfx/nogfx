// Package clock implements an Endpoint that pushes periodic clock.Tick
// events onto the engine's event channel. It exists so that processors
// can do timer-driven work (keep-alive, lag visualisation, expiry sweeps)
// without owning a goroutine or reaching outside the chain for time.
package clock

import (
	"context"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/clock"
)

// Ticker is an Endpoint that emits clock.Tick events at a fixed interval.
// It applies no effects — Apply always returns ErrEffectNotApplicable —
// so it sits in the engine's Sources list rather than as Connection or UI.
type Ticker struct {
	interval time.Duration
	now      func() time.Time
}

// NewTicker returns a Ticker emitting one Tick every interval. The first
// Tick fires after the first interval elapses; there is no immediate tick
// at startup.
func NewTicker(interval time.Duration) *Ticker {
	return &Ticker{interval: interval, now: time.Now}
}

// Run pushes Tick events onto events until ctx is cancelled. The cadence
// is driven by time.Ticker so a slow consumer cannot cause unbounded
// catch-up — missed ticks are dropped, which matches keep-alive's
// "fire-and-forget" semantics.
func (t *Ticker) Run(ctx context.Context, events chan<- app.Event) error {
	if t.interval <= 0 {
		<-ctx.Done()

		return nil
	}

	tk := time.NewTicker(t.interval)
	defer tk.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-tk.C:
			select {
			case <-ctx.Done():
				return nil
			case events <- clock.Tick{At: t.now()}:
			}
		}
	}
}

// Apply is a no-op for the Ticker; it never receives effects.
func (t *Ticker) Apply(_ app.Effect) ([]app.Event, error) {
	return nil, app.ErrEffectNotApplicable
}
