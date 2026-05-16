package processors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/processors"
	"github.com/tobiassjosten/nogfx/ui"
)

func TestInput_TranslatesUIInputToSend(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			ui.Input{Bytes: []byte("kick orc")},
			ui.Input{Bytes: []byte("kick troll")},
		},
	}

	got, err := processors.Input()(batch)
	require.NoError(t, err)
	require.Len(t, got.Commands, 2)
	assert.Equal(t, []byte("kick orc"), got.Commands[0].(connection.Send).Bytes)
	assert.Equal(t, []byte("kick troll"), got.Commands[1].(connection.Send).Bytes)
}

func TestInput_IgnoresOtherEvents(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.TextLine{Bytes: []byte("server says hi")},
			ui.Resize{Width: 80, Height: 24},
		},
	}

	got, err := processors.Input()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}

func TestInput_NoEvents(t *testing.T) {
	got, err := processors.Input()(app.Batch{})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
	assert.Empty(t, got.Events)
}
