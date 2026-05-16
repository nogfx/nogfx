// Package ui holds the contract for the user-facing endpoint: the events the
// UI emits (user input, resize, …) and the commands it accepts (print,
// vitals, target, …). Concrete implementations of the UI (e.g. tcell) live
// in platform/.
package ui

import "github.com/tobiassjosten/nogfx/app"

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
