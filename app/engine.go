package app

import (
	"context"
	"errors"
	"log"
)

// Engine pumps events through the processor chain and routes the
// resulting commands to the endpoints.
//
// Ordering contract (see Batch for the data shape):
//
//  1. Within a batch, every command is applied (in order) before any
//     derived event is re-emitted.
//  2. Derived events are processed in their emission order, each as its
//     own batch, before the engine returns to the endpoint channel.
//  3. Apply-consequence events from endpoints (e.g. server responses to
//     a write) therefore land after the entire chain reaction triggered
//     by the original event has completed.
//
// The mechanism is a local FIFO queue: the engine drains derived events
// from that queue before reading new events from the endpoint channel.
type Engine struct {
	Connection Endpoint
	UI         Endpoint
	Processor  Processor
}

// Run starts the engine and blocks until ctx is cancelled or one of the
// endpoints errors out.
func (engine *Engine) Run(pctx context.Context) error {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	endpointEvents := make(chan Event, 256)
	errs := make(chan error, 2)

	go func() {
		if err := engine.Connection.Run(ctx, endpointEvents); err != nil {
			errs <- err
		}
		cancel()
	}()

	go func() {
		if err := engine.UI.Run(ctx, endpointEvents); err != nil {
			errs <- err
		}
		cancel()
	}()

	var derived []Event

	for {
		var ev Event

		// The local derived queue takes priority over the endpoint
		// channel — that's what enforces "derived events come before
		// apply-consequence events".
		if len(derived) > 0 {
			ev = derived[0]
			derived = derived[1:]
		} else {
			select {
			case <-ctx.Done():
				select {
				case err := <-errs:
					return err
				default:
					return nil
				}
			case ev = <-endpointEvents:
			}
		}

		batch := Batch{Event: ev}

		if engine.Processor != nil {
			next, err := engine.Processor(batch)
			if err != nil {
				log.Printf("processor chain: %v", err)
				continue
			}
			batch = next
		}

		// Drop any command the trigger event forbids (see GuardedEvent).
		// This is the contract-level cycle break: e.g. ReFormatting
		// forbids ReFormat so a buggy processor can't loop the chain.
		if guarded, ok := batch.Event.(GuardedEvent); ok {
			filtered := batch.Commands[:0]
			for _, cmd := range batch.Commands {
				if guarded.Forbids(cmd) {
					log.Printf("dropping forbidden command %T in batch triggered by %T", cmd, batch.Event)
					continue
				}
				filtered = append(filtered, cmd)
			}
			batch.Commands = filtered
		}

		// Apply all commands first, in order. Synchronous Apply means
		// any async consequence from an endpoint arrives later via the
		// endpoint channel, not as an interleaved command effect.
		engine.dispatch(batch.Commands)

		// Then queue derived events for processing before any new
		// endpoint event is read.
		derived = append(derived, batch.Events...)
	}
}

func (engine *Engine) dispatch(commands []Command) {
	for _, cmd := range commands {
		if err := engine.Connection.Apply(cmd); err == nil {
			continue
		} else if !errors.Is(err, ErrCommandNotApplicable) {
			log.Printf("connection apply (%T): %v", cmd, err)
			continue
		}
		if err := engine.UI.Apply(cmd); err == nil {
			continue
		} else if !errors.Is(err, ErrCommandNotApplicable) {
			log.Printf("ui apply (%T): %v", cmd, err)
			continue
		}
		log.Printf("unhandled command: %T", cmd)
	}
}
