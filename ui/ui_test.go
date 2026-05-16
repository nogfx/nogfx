package ui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/lib/navigation"
	"github.com/tobiassjosten/nogfx/ui"
)

// TestEventsSatisfyAppEvent guards against accidentally dropping the
// EventMarker embed.
func TestEventsSatisfyAppEvent(t *testing.T) {
	var _ app.Event = ui.Input{}
	var _ app.Event = ui.Resize{}
}

// TestCommandsSatisfyAppCommand covers every UI command, so a new command
// type added without the marker embed is caught.
func TestCommandsSatisfyAppCommand(t *testing.T) {
	var _ app.Command = ui.PrintLine{}
	var _ app.Command = ui.SetHealth{}
	var _ app.Command = ui.SetMana{}
	var _ app.Command = ui.AddVital{}
	var _ app.Command = ui.SetVital{}
	var _ app.Command = ui.RemoveVital{}
	var _ app.Command = ui.SetCharacter{}
	var _ app.Command = ui.SetTarget{}
	var _ app.Command = ui.SetRoom{}
	var _ app.Command = ui.MaskInput{}
	var _ app.Command = ui.UnmaskInput{}
}

func TestSnapshotShapes(t *testing.T) {
	tgt := ui.Target{Name: "orc", Health: 50, Queue: 2}
	assert.Equal(t, "orc", tgt.Name)
	assert.Equal(t, 50, tgt.Health)
	assert.Equal(t, 2, tgt.Queue)
}

func TestSetRoomCarriesNavigationPointer(t *testing.T) {
	room := &navigation.Room{Name: "Forest"}
	cmd := ui.SetRoom{Room: room}
	assert.Same(t, room, cmd.Room)
}

func TestVitalsCommandsCarryValues(t *testing.T) {
	assert.Equal(t, 30, ui.SetHealth{Value: 30, Max: 100}.Value)
	assert.Equal(t, 100, ui.SetHealth{Value: 30, Max: 100}.Max)

	add := ui.AddVital{Name: "ki", Value: 45, Max: 100}
	assert.Equal(t, "ki", add.Name)
	assert.Equal(t, 45, add.Value)
	assert.Equal(t, 100, add.Max)
}
