package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/internal/navigation"
)

// TestEventsSatisfyAppEvent guards against accidentally dropping the
// EventMarker embed.
func TestEventsSatisfyAppEvent(t *testing.T) {
	var (
		_ app.Event = ui.Input{}
		_ app.Event = ui.Resize{}
		_ app.Event = ui.ReFormatting{}
	)
}

// TestEffectsSatisfyAppEffect covers every UI effect, so a new effect
// type added without the marker embed is caught.
func TestEffectsSatisfyAppEffect(t *testing.T) {
	var (
		_ app.Effect = ui.PrintLine{}
		_ app.Effect = ui.ReFormat{}
		_ app.Effect = ui.SetHealth{}
		_ app.Effect = ui.SetMana{}
		_ app.Effect = ui.AddVital{}
		_ app.Effect = ui.SetVital{}
		_ app.Effect = ui.RemoveVital{}
		_ app.Effect = ui.SetCharacter{}
		_ app.Effect = ui.SetTarget{}
		_ app.Effect = ui.SetRoom{}
		_ app.Effect = ui.MaskInput{}
		_ app.Effect = ui.UnmaskInput{}
	)
}

func TestSnapshotShapes(t *testing.T) {
	tgt := ui.Target{Name: "orc", Health: 50, Queue: 2}
	assert.Equal(t, "orc", tgt.Name)
	assert.Equal(t, 50, tgt.Health)
	assert.Equal(t, 2, tgt.Queue)
}

func TestSetRoomCarriesNavigationPointer(t *testing.T) {
	room := &navigation.Room{Name: "Forest"}
	eff := ui.SetRoom{Room: room}
	assert.Same(t, room, eff.Room)
}

func TestVitalsEffectsCarryValues(t *testing.T) {
	assert.Equal(t, 30, ui.SetHealth{Value: 30, Max: 100}.Value)
	assert.Equal(t, 100, ui.SetHealth{Value: 30, Max: 100}.Max)

	add := ui.AddVital{Name: "ki", Value: 45, Max: 100}
	assert.Equal(t, "ki", add.Name)
	assert.Equal(t, 45, add.Value)
	assert.Equal(t, 100, add.Max)
}
