package app

import "context"

// Endpoint is one side of the engine's pipeline. It runs in its own
// goroutine, pushes the events it observes onto the engine's shared
// channel, and applies commands the engine dispatches to it.
//
// Both ends of the pipeline (the network connection and the user-facing
// UI) implement the same shape; what distinguishes them is which concrete
// events they emit and which commands they accept. Apply returns
// ErrCommandNotApplicable for commands targeting the other endpoint, so
// the engine can route by attempting Apply on each in turn.
type Endpoint interface {
	Run(ctx context.Context, events chan<- Event) error
	Apply(cmd Command) error
}
