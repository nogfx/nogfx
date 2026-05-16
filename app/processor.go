package app

import "fmt"

// Processor reads a Batch, optionally appends events and commands, and
// returns the modified Batch. The same Processor signature is used for
// every kind of work in the chain — generic, world-specific, user scripts.
type Processor func(Batch) (Batch, error)

// Chain composes processors into a single Processor that runs them in order.
// Each processor sees what the previous ones produced.
func Chain(processors ...Processor) Processor {
	return func(b Batch) (Batch, error) {
		for i, proc := range processors {
			if proc == nil {
				continue
			}
			next, err := proc(b)
			if err != nil {
				return b, fmt.Errorf("processor %d: %w", i, err)
			}
			b = next
		}
		return b, nil
	}
}
