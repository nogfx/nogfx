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

func TestOutput_TextLineToPrintLine(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.TextLine{Bytes: []byte("you see an orc")},
		},
	}

	got, err := processors.Output()(batch)
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)
	assert.Equal(t, []byte("you see an orc"), got.Commands[0].(ui.PrintLine).Text)
}

func TestOutput_PromptAlsoPrints(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.Prompt{Bytes: []byte("hp:50 >")},
		},
	}

	got, err := processors.Output()(batch)
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)
	assert.Equal(t, []byte("hp:50 >"), got.Commands[0].(ui.PrintLine).Text)
}

func TestOutput_IgnoresUIEvents(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			ui.Input{Bytes: []byte("typed")},
		},
	}

	got, err := processors.Output()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}
