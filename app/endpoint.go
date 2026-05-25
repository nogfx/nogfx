package app

import "context"

// Endpoint is one side of the engine's pipeline. It runs in its own
// goroutine, pushes the events it observes onto the engine's shared
// channel, and applies effects the engine dispatches to it.
//
// Both ends of the pipeline (the network connection and the user-facing
// UI) implement the same shape; what distinguishes them is which concrete
// events they emit and which effects they accept. Apply returns
// ErrEffectNotApplicable for effects targeting the other endpoint, so
// the engine can route by attempting Apply on each in turn.
//
// Apply may return apply-consequence events — events synthesised
// in direct response to the effect just applied (e.g. connection.Sent
// after a successful wire write). The engine processes them through
// the chain before any new event from the endpoint channel, mirroring
// the ordering contract for processor-derived events. Returning nil is
// the common case.
type Endpoint interface {
	Run(ctx context.Context, events chan<- Event) error
	Apply(eff Effect) ([]Event, error)
}
