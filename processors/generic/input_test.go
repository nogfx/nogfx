package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestInput_TranslatesUIInputToSend(t *testing.T) {
	batch := app.Batch{
		Event: ui.Input{Bytes: []byte("kick orc")},
	}

	got, err := generic.Input()(batch)
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)
	assert.Equal(t, []byte("kick orc"), got.Effects[0].(connection.Send).Bytes)
}

func TestInput_IgnoresOtherEvents(t *testing.T) {
	batch := app.Batch{
		Event: ui.Resize{Width: 80, Height: 24},
	}

	got, err := generic.Input()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
}

func TestInput_NoEvent(t *testing.T) {
	got, err := generic.Input()(app.Batch{})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
	assert.Empty(t, got.Events)
}
