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

func TestOutput_TextLineToPrintLine(t *testing.T) {
	batch := app.Batch{
		Event: connection.TextLine{Bytes: []byte("you see an orc")},
	}

	got, err := generic.Output()(batch)
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)
	line := got.Effects[0].(ui.PrintLine).Line
	assert.Equal(t, []byte("you see an orc"), line.Raw)
	assert.Equal(t, []byte("you see an orc"), line.Formatted)
}

func TestOutput_PromptAlsoPrints(t *testing.T) {
	batch := app.Batch{
		Event: connection.Prompt{Bytes: []byte("hp:50 >")},
	}

	got, err := generic.Output()(batch)
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)
	line := got.Effects[0].(ui.PrintLine).Line
	assert.Equal(t, []byte("hp:50 >"), line.Raw)
	assert.Equal(t, []byte("hp:50 >"), line.Formatted)
}

func TestOutput_IgnoresUIEvents(t *testing.T) {
	batch := app.Batch{
		Event: ui.Input{Bytes: []byte("typed")},
	}

	got, err := generic.Output()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
}
