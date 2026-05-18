package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// Input converts a ui.Input event into a connection.Send command. This is
// the bridge that turns user-submitted lines into bytes the game server
// receives; without it, keystrokes never make it onto the wire.
//
// Input runs early in the chain so subsequent processors
// (SplitInputProcessor, RepeatInputProcessor, world-specific
// alias/macro expansions) operate on Send commands rather than on the
// raw ui.Input event.
func Input() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		inp, ok := batch.Event.(ui.Input)
		if !ok {
			return batch, nil
		}
		return batch.AppendCommand(connection.Send{Bytes: inp.Bytes}), nil
	}
}
