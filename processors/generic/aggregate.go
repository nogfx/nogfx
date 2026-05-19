package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// Aggregator buffers TextLine and GMCPFrame events between prompts and
// emits a connection.Message derived event when a Prompt arrives. The
// underlying events still flow through the chain — Aggregator is
// purely additive, intended for processors that want a turn-level view
// (TunnelVision rewrite, illusion detection, misframing splitter).
//
// Buffer semantics for orphans: GMCP frames that arrive with no
// following prompt accumulate in the buffer until the next Prompt
// eventually fires, at which point they join that prompt's Message.
// This deliberately attaches them to a logically unrelated turn rather
// than stranding them — the state-update path (the immediate
// GMCPFrame events) has already applied the state changes; the
// Message bundle is a turn-level view, and a tolerant consumer is
// preferable to a stranded one. See docs/design/messages.md.
func Aggregator() app.Processor {
	var (
		lines []connection.TextLine
		gmcp  []connection.GMCPFrame
	)

	return func(batch app.Batch) (app.Batch, error) {
		switch ev := batch.Event.(type) {
		case connection.TextLine:
			lines = append(lines, ev)
		case connection.GMCPFrame:
			gmcp = append(gmcp, ev)
		case connection.Prompt:
			msg := connection.Message{
				Lines:  lines,
				GMCP:   gmcp,
				Prompt: ev,
			}
			lines = nil
			gmcp = nil
			batch = batch.AppendEvent(msg)
		}

		return batch, nil
	}
}
