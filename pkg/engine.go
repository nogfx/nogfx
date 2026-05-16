package pkg

import (
	"context"
	"errors"
	"log"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/ui"
)

// Engine is the orchestrator of all the cogs of this machinery. It pumps
// events from the connection and the UI through the processor chain as
// batches, and routes the resulting commands back to the endpoint that
// handles them.
type Engine struct {
	Connection connection.Connection
	UI         ui.UI
	Processor  app.Processor
}

// Run starts the engine and blocks until ctx is cancelled or one of the
// endpoints errors out.
func (engine *Engine) Run(pctx context.Context) error {
	ctx, cancel := context.WithCancel(pctx)
	defer cancel()

	events := make(chan app.Event, 64)
	errs := make(chan error, 2)

	go func() {
		if err := engine.Connection.Run(ctx, events); err != nil {
			errs <- err
		}
		cancel()
	}()

	go func() {
		if err := engine.UI.Run(ctx, events); err != nil {
			errs <- err
		}
		cancel()
	}()

	for {
		select {
		case <-ctx.Done():
			select {
			case err := <-errs:
				return err
			default:
				return nil
			}

		case ev := <-events:
			engine.processEvent(ev)
		}
	}
}

func (engine *Engine) processEvent(ev app.Event) {
	batch := app.Batch{Events: []app.Event{ev}}

	if engine.Processor != nil {
		next, err := engine.Processor(batch)
		if err != nil {
			log.Printf("processor chain: %v", err)
			return
		}
		batch = next
	}

	engine.dispatch(batch)
}

func (engine *Engine) dispatch(batch app.Batch) {
	for _, cmd := range batch.Commands {
		if err := engine.Connection.Apply(cmd); err == nil {
			continue
		} else if !errors.Is(err, app.ErrCommandNotApplicable) {
			log.Printf("connection apply (%T): %v", cmd, err)
			continue
		}
		if err := engine.UI.Apply(cmd); err == nil {
			continue
		} else if !errors.Is(err, app.ErrCommandNotApplicable) {
			log.Printf("ui apply (%T): %v", cmd, err)
			continue
		}
		log.Printf("unhandled command: %T", cmd)
	}
}
