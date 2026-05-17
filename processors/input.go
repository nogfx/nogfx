package processors

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/ui"
)

// Input converts each ui.Input event in the batch into a connection.Send
// command. This is the bridge that turns user-submitted lines into bytes the
// game server receives; without it, keystrokes never make it onto the wire.
//
// Input is typically the first processor in the chain, so subsequent
// processors (SplitInputProcessor, RepeatInputProcessor, world-specific
// alias/macro expansions) operate on Send commands rather than on the raw
// ui.Input events.
func Input() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		for _, ev := range batch.Events {
			inp, ok := ev.(ui.Input)
			if !ok {
				continue
			}
			batch = batch.AppendCommand(connection.Send{Bytes: inp.Bytes})
		}
		return batch, nil
	}
}
