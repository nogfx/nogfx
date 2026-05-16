package ui

import (
	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/lib/navigation"
)

// PrintLine appends a line to the user-facing scrollback.
type PrintLine struct {
	app.CommandMarker
	Text []byte
}

// SetHealth updates the primary health vital.
type SetHealth struct {
	app.CommandMarker
	Value, Max int
}

// SetMana updates the primary mana vital.
type SetMana struct {
	app.CommandMarker
	Value, Max int
}

// AddVital ensures the named auxiliary vital exists with the given values.
// AddVital is idempotent — emitting it every pass is fine; state is owned by
// the emitter, not the UI.
type AddVital struct {
	app.CommandMarker
	Name       string
	Value, Max int
}

// SetVital updates the values of an existing auxiliary vital.
type SetVital struct {
	app.CommandMarker
	Name       string
	Value, Max int
}

// RemoveVital removes a named auxiliary vital.
type RemoveVital struct {
	app.CommandMarker
	Name string
}

// SetCharacter updates the character's identity fields.
type SetCharacter struct {
	app.CommandMarker
	Name, Title string
}

// SetTarget updates the current target. A nil Target clears the target.
type SetTarget struct {
	app.CommandMarker
	Target *Target
}

// SetRoom updates the current room. A nil Room clears the room. The
// navigation.Room is the rich graph node so the UI can render adjacency
// (e.g. the minimap); world adapters pass it directly rather than
// projecting through a slim snapshot.
type SetRoom struct {
	app.CommandMarker
	Room *navigation.Room
}

// MaskInput hides characters the user types (typical of password entry).
type MaskInput struct {
	app.CommandMarker
}

// UnmaskInput restores normal echoing of typed characters.
type UnmaskInput struct {
	app.CommandMarker
}
