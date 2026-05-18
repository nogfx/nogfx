package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// Output translates server-side text events (connection.TextLine and
// connection.Prompt) into ui.PrintLine commands so the user sees what
// the game says. It is typically placed late in the chain, after any
// processors that might want to drop or rewrite individual lines (e.g.
// an "omit spam" filter), so those filters can either replace the
// trigger event with something else or nil it out.
func Output() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		switch e := batch.Event.(type) {
		case connection.TextLine:
			return batch.AppendCommand(ui.PrintLine{
				Line: ui.Line{Raw: e.Bytes, Formatted: e.Bytes},
			}), nil
		case connection.Prompt:
			return batch.AppendCommand(ui.PrintLine{
				Line: ui.Line{Raw: e.Bytes, Formatted: e.Bytes},
			}), nil
		}
		return batch, nil
	}
}
