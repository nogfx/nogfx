// Package ui holds the contract for the user-facing endpoint: the events the
// UI emits (user input, resize, …) and the commands it accepts (print,
// vitals, target, …). Concrete implementations of the UI (e.g. tcell) live
// in platform/.
package ui

import "github.com/nogfx/nogfx/app"

// Input is one line the user submitted (typically after pressing enter; the
// UI buffers keystrokes locally and emits Input when the line is complete).
type Input struct {
	app.EventMarker
	Bytes []byte
}

// Resize signals that the user's terminal has been resized.
type Resize struct {
	app.EventMarker
	Width, Height int
}

// ReFormatting is the UI's reply to a ReFormat command: one event per
// scrollback line in scope, in scrollback order (oldest first). Processors
// that recognise their own lines emit a replacement PrintLine carrying the
// same Line.ID so the UI overwrites the slot in place.
//
// ReFormatting implements app.GuardedEvent and forbids ReFormat commands
// in its batch — emitting one would re-enter the same code path and loop.
type ReFormatting struct {
	app.EventMarker
	Line Line
}

// Forbids implements app.GuardedEvent.
func (ReFormatting) Forbids(cmd app.Command) bool {
	_, ok := cmd.(ReFormat)
	return ok
}
