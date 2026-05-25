package ui

import (
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/internal/navigation"
)

// PrintLine appends (or, when Line.ID is set, overwrites) a line in the
// user-facing scrollback. Emitters set Line.Raw and Line.Formatted; the UI
// assigns Line.ID on first print. Reformatters echo the incoming ID back
// so the UI overwrites the existing slot rather than appending.
type PrintLine struct {
	app.EffectMarker
	Line Line
}

// ReFormat asks the UI to replay every scrollback line back through the
// processor chain as ReFormatting events, giving processors a chance to
// rewrite their formatting under whatever policy is now in effect.
type ReFormat struct {
	app.EffectMarker
}

// SetHealth updates the primary health vital.
type SetHealth struct {
	app.EffectMarker
	Value, Max int
}

// SetMana updates the primary mana vital.
type SetMana struct {
	app.EffectMarker
	Value, Max int
}

// AddVital ensures the named auxiliary vital exists with the given values.
// AddVital is idempotent — emitting it every pass is fine; state is owned by
// the emitter, not the UI.
type AddVital struct {
	app.EffectMarker
	Name       string
	Value, Max int
}

// SetVital updates the values of an existing auxiliary vital.
type SetVital struct {
	app.EffectMarker
	Name       string
	Value, Max int
}

// RemoveVital removes a named auxiliary vital.
type RemoveVital struct {
	app.EffectMarker
	Name string
}

// SetCharacter updates the character's identity fields.
type SetCharacter struct {
	app.EffectMarker
	Name, Title string
}

// SetTarget updates the current target. A nil Target clears the target.
type SetTarget struct {
	app.EffectMarker
	Target *Target
}

// SetRoom updates the current room. A nil Room clears the room. The
// navigation.Room is the rich graph node so the UI can render adjacency
// (e.g. the minimap); world adapters pass it directly rather than
// projecting through a slim snapshot.
type SetRoom struct {
	app.EffectMarker
	Room *navigation.Room
}

// SetLag updates the current measured round-trip latency to the server.
// Producers (today: the LagWatcher processor) emit this whenever a new
// measurement lands. A zero Lag means "no measurement yet" and renders
// blank; the UI is otherwise idempotent — repeated emissions just refresh
// the displayed value.
type SetLag struct {
	app.EffectMarker
	Lag time.Duration
}

// MaskInput hides characters the user types (typical of password entry).
type MaskInput struct {
	app.EffectMarker
}

// UnmaskInput restores normal echoing of typed characters.
type UnmaskInput struct {
	app.EffectMarker
}
