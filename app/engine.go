package app

import (
	"context"
	"errors"
	"log"
)

// eventBufferCap is the depth of the engine's shared event channel.
// Endpoints (Connection, UI, every Source) push events into it; the
// engine drains them one at a time through the processor chain. The
// buffer is sized for bursty server output (a typical Iron-Realms
// turn is a handful of events) with a generous margin; a runaway
// pile-up is surfaced via warning logs at 50/75/90% (see
// eventBufferWarnThresholds) before it can stall any producer.
const eventBufferCap = 1024

// eventBufferWarnThresholds are the fill levels (in slots, not
// percent) at which Engine.Run logs a warning. A given threshold
// logs once per upward crossing; if the buffer drains back below it
// and rises again, it logs again. Producers don't block until the
// buffer is fully saturated, so seeing 50% is meaningful enough to
// surface — typical steady-state should sit near zero.
var eventBufferWarnThresholds = []int{
	eventBufferCap / 2,      // 50%
	eventBufferCap * 3 / 4,  // 75%
	eventBufferCap * 9 / 10, // 90%
}

// Engine pumps events through the processor chain and routes the
// resulting effects to the endpoints.
//
// Two terms appear in the contract and are easy to conflate, so they
// have distinct meanings here:
//
//   - "Apply-consequence event" — an event an endpoint's Apply returns
//     synchronously after handling an effect (e.g. connection.Sent
//     after a successful wire write). It flows back through the chain
//     from inside the engine loop.
//   - "Endpoint-channel event" — an event an endpoint pushes onto the
//     shared channel from its own Run goroutine (server output, IAC
//     traffic, user input, ticks, and any wire-level server reply that
//     happens to be a downstream consequence of an effect we wrote).
//
// Ordering contract (see Batch for the data shape):
//
//  1. Within a batch, every effect is applied (in order) before any
//     derived event is re-emitted.
//  2. Apply-consequence events and derived events are processed in
//     emission order, each as its own batch, before the engine returns
//     to the endpoint channel.
//  3. Any endpoint-channel event therefore lands after the entire chain
//     reaction triggered by the original event has completed — including
//     wire-level server replies, which arrive via Run, not via Apply.
//
// The mechanism is a local FIFO queue: the engine drains derived events
// (whether processor-emitted or apply-emitted) from that queue before
// reading new events from the endpoint channel.
type Engine struct {
	Connection Endpoint
	UI         Endpoint
	// Sources are emission-only endpoints (e.g. a Ticker) that push events
	// onto the engine's shared channel but receive no effects. The engine
	// runs each in its own goroutine and does not route effects to them.
	Sources   []Endpoint
	Processor Processor
}

// Run starts the engine and blocks until ctx is cancelled or one of the
// endpoints errors out.
func (engine *Engine) Run(pctx context.Context) error {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	endpointEvents := make(chan Event, eventBufferCap)
	errs := make(chan error, 2+len(engine.Sources))

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

	for _, src := range engine.Sources {
		go func() {
			if err := src.Run(ctx, endpointEvents); err != nil {
				errs <- err
			}

			cancel()
		}()
	}

	var (
		derived       []Event
		fillWarnIndex int
	)

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
				fillWarnIndex = checkEventBufferFill(len(endpointEvents), fillWarnIndex)
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

		// Drop any effect the trigger event forbids (see GuardedEvent).
		// This is the contract-level cycle break: e.g. ReFormatting
		// forbids ReFormat so a buggy processor can't loop the chain.
		if guarded, ok := batch.Event.(GuardedEvent); ok {
			filtered := batch.Effects[:0]
			for _, eff := range batch.Effects {
				if guarded.Forbids(eff) {
					log.Printf("dropping forbidden effect %T in batch triggered by %T", eff, batch.Event)

					continue
				}

				filtered = append(filtered, eff)
			}

			batch.Effects = filtered
		}

		// Apply all effects first, in order. Apply-consequence events
		// returned by endpoints (e.g. connection.Sent after a successful
		// wire write) are queued ahead of processor-derived events: they
		// were caused by this batch's effects and should flow through the
		// chain before any independent derived events the processors
		// emitted.
		applyEvents := engine.dispatch(batch.Effects)
		derived = append(derived, applyEvents...)
		derived = append(derived, batch.Events...)
	}
}

func (engine *Engine) dispatch(effects []Effect) []Event {
	var emitted []Event

	for _, eff := range effects {
		evs, err := engine.Connection.Apply(eff)
		if err == nil {
			emitted = append(emitted, evs...)

			continue
		} else if !errors.Is(err, ErrEffectNotApplicable) {
			log.Printf("connection apply (%T): %v", eff, err)

			continue
		}

		evs, err = engine.UI.Apply(eff)
		if err == nil {
			emitted = append(emitted, evs...)

			continue
		} else if !errors.Is(err, ErrEffectNotApplicable) {
			log.Printf("ui apply (%T): %v", eff, err)

			continue
		}

		log.Printf("unhandled effect: %T", eff)
	}

	return emitted
}

// checkEventBufferFill logs a warning when fill crosses into a new
// upward threshold bucket and returns the new bucket index. The
// bucket index is recomputed from the current fill on each call, so
// any drop past a threshold immediately re-arms a future log for that
// band: a buffer that rises, drains, then rises again logs once per
// rise rather than once per session.
func checkEventBufferFill(fill, lastIdx int) int {
	idx := 0

	for i, t := range eventBufferWarnThresholds {
		if fill >= t {
			idx = i + 1
		}
	}

	if idx > lastIdx {
		pct := 100 * fill / eventBufferCap
		log.Printf("engine: event buffer at %d%% (%d/%d slots)", pct, fill, eventBufferCap)
	}

	return idx
}
