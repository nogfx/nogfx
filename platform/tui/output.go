package tui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/nogfx/nogfx/app/ui"
)

// outputPadding is the cell used to pad short rows up to the rendered
// width. It is a non-breaking space (U+00A0) rather than a regular space
// so the wrap algorithm can distinguish padding (non-breakable filler)
// from genuine inter-word spacing (a wrap point).
var outputPadding = NewCell(' ')

// Print shows a message to the user.
func (tui *TUI) Print(output []byte) {
	// @todo Make it set its own color (or ^[37m) before resetting back to
	// the previous last seen style.
	tui.output.Append(output)
	tui.setCache(paneOutput, nil)
	tui.Draw()
}

// scrollbackLimit caps how many lines the Output buffer keeps.
const scrollbackLimit = 5000

// Output is the widget where game output is shown. It keeps a parallel
// slice of ui.Line values (`lines`) alongside the rendered `buffer` so the
// raw bytes and per-line ID can be replayed later (see ReFormat).
type Output struct {
	buffer Rows
	lines  []ui.Line // lines[i] corresponds to buffer[i]; both most-recent-first
	byID   map[uint64]int
	nextID uint64
	offset int
	pwidth int
	style  tcell.Style
}

// Append adds a new paragraph to the Output. It preserves the original
// signature for code that has no Line value to forward; AppendLine is the
// richer entry point that the engine's PrintLine handler uses.
func (output *Output) Append(data []byte) {
	output.appendLine(ui.Line{Raw: data, Formatted: data})
}

// AppendLine adds (or, when line.ID is set, overwrites) a paragraph in
// the Output. Returns the resulting ID, which the caller can echo back
// to the source if needed.
func (output *Output) AppendLine(line ui.Line) uint64 {
	return output.appendLine(line)
}

func (output *Output) appendLine(line ui.Line) uint64 {
	if line.Formatted == nil {
		line.Formatted = line.Raw
	}

	if line.ID != 0 {
		if idx, ok := output.byID[line.ID]; ok {
			row, _ := NewRowFromBytes(line.Formatted, output.style)
			output.buffer[idx] = row
			output.lines[idx] = line
			return line.ID
		}
	}

	if line.ID == 0 {
		output.nextID++
		line.ID = output.nextID
	} else if line.ID > output.nextID {
		output.nextID = line.ID
	}

	row, style := NewRowFromBytes(line.Formatted, output.style)
	output.style = style

	output.buffer = output.buffer.prepend(row)
	output.lines = append([]ui.Line{line}, output.lines...)

	if output.byID == nil {
		output.byID = map[uint64]int{}
	}
	// All existing indices shift by one (prepend).
	for id := range output.byID {
		output.byID[id]++
	}
	output.byID[line.ID] = 0

	if output.offset > 0 && output.pwidth > 0 {
		output.offset += len(row.Wrap(output.pwidth))
	}

	if len(output.buffer) > scrollbackLimit {
		dropped := output.lines[scrollbackLimit:]
		for _, l := range dropped {
			delete(output.byID, l.ID)
		}
		output.buffer = output.buffer[0:scrollbackLimit]
		output.lines = output.lines[0:scrollbackLimit]
	}

	return line.ID
}

// Lines returns every scrollback line in chronological (oldest-first)
// order. Used by ReFormat to replay the buffer as ReFormatting events.
func (output *Output) Lines() []ui.Line {
	if len(output.lines) == 0 {
		return nil
	}
	// buffer/lines are most-recent-first; return oldest-first.
	out := make([]ui.Line, len(output.lines))
	for i := range output.lines {
		out[len(output.lines)-1-i] = output.lines[i]
	}
	return out
}

// RenderOutput renders the current Output.
func (tui *TUI) RenderOutput(width, height int) Rows {
	if rows, ok := tui.getCache(paneOutput); ok {
		return rows
	}

	rows := RenderOutput(tui.output, width, height)

	tui.setCache(paneOutput, rows)

	return rows
}

// RenderOutput renders the given Output.
func RenderOutput(output *Output, width, height int) Rows {
	rows := Rows{}

	padding := outputPadding

	if width == 0 || height == 0 {
		return rows
	}

	// @todo Make resizing maintain history scrollback. Resetting it is a
	// temporary workaround because calculating and maintaining scrollback
	// state through resizing is a pain in the ass.
	if output.pwidth > 0 && output.pwidth != width {
		output.offset = 0
	}
	output.pwidth = width

	// Make sure to render enough for a history scrollback split.
	height += output.offset

	for _, row := range output.buffer {
		paragraph := row.Wrap(width, padding)

		// Rows are ordered with the most recent one first, so we
		// prepend older paragraphs to the rows.
		for i := len(paragraph) - 1; i >= 0; i-- {
			rows = rows.prepend(paragraph[i])
		}

		if len(rows) >= height {
			break
		}
	}

	// Reset back to actual height, for finalization.
	height -= output.offset
	length := len(rows)

	// For simpler cases we just return the full buffer.
	if height <= 2 || length <= height || output.offset == 0 {
		rows = rows[max(0, length-height):]
		rows = append(NewRows(width, height-length, padding), rows...)
		return rows
	}

	// Cap offset to the last row in the buffer.
	output.offset = min(length-height, output.offset)

	hheight := length - height - output.offset
	history := rows[hheight : hheight+height/2]

	// @todo Mark this divider better, with colors and flair.
	divider := NewRow(width, NewCell(tcell.RuneHLine))

	rows = rows[length-(height-height/2)+1:]

	return append(history, append(Rows{divider}, rows...)...)
}
