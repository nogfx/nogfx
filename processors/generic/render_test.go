package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/platform/gmcp"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestRender_CharName(t *testing.T) {
	batch := app.Batch{
		Event: generic.DecodedGMCP{Message: &gmcp.CharName{
			Name:     "asdf",
			Fullname: "AsDf the Mighty",
		}},
	}

	got, err := generic.Render()(batch)
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)

	set, ok := got.Commands[0].(ui.SetCharacter)
	require.True(t, ok)
	assert.Equal(t, "asdf", set.Name)
	assert.Equal(t, "AsDf the Mighty", set.Title)
}

func TestRender_RoomInfo(t *testing.T) {
	batch := app.Batch{
		Event: generic.DecodedGMCP{Message: &gmcp.RoomInfo{
			Number: 42,
			Name:   "A Glade",
		}},
	}

	got, err := generic.Render()(batch)
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)

	set, ok := got.Commands[0].(ui.SetRoom)
	require.True(t, ok)
	require.NotNil(t, set.Room)
	assert.Equal(t, "A Glade", set.Room.Name)
	assert.True(t, set.Room.HasPlayer)
}

func TestRender_IgnoresUnknownMessages(t *testing.T) {
	batch := app.Batch{
		Event: generic.DecodedGMCP{Message: &gmcp.CharLogin{Name: "x"}},
	}

	got, err := generic.Render()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}
