package processors

import (
	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/ui"
)

// Output translates server-side text events (connection.TextLine and
// connection.Prompt) into ui.PrintLine commands so the user sees what the
// game says. It is typically placed late in the chain, after any
// processors that might want to drop or rewrite individual lines (e.g. an
// "omit spam" filter), so those filters can simply not emit a PrintLine
// for lines they want suppressed.
//
// Currently Prompts are rendered as ordinary scrollback lines. A future
// ui.SetPrompt command may split that out, at which point Output stops
// emitting PrintLine for connection.Prompt and emits SetPrompt instead.
func Output() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		for _, ev := range batch.Events {
			switch e := ev.(type) {
			case connection.TextLine:
				batch = batch.AppendCommand(ui.PrintLine{Text: e.Bytes})
			case connection.Prompt:
				batch = batch.AppendCommand(ui.PrintLine{Text: e.Bytes})
			}
		}
		return batch, nil
	}
}
