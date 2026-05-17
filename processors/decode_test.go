package processors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
	"github.com/nogfx/nogfx/processors"
)

func TestDecode_KnownMessage(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.GMCPFrame{Payload: []byte(`Char.Name {"name":"asdf","fullname":"AsDf"}`)},
		},
	}

	got, err := processors.Decode()(batch)
	require.NoError(t, err)
	require.Len(t, got.Events, 2, "Decode appends a DecodedGMCP after the original GMCPFrame")

	decoded, ok := got.Events[1].(processors.DecodedGMCP)
	require.True(t, ok)

	name, ok := decoded.Message.(*gmcp.CharName)
	require.True(t, ok)
	assert.Equal(t, "asdf", name.Name)
	assert.Equal(t, "AsDf", name.Fullname)
}

func TestDecode_UnknownMessage_SilentlySkipped(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.GMCPFrame{Payload: []byte("Non.Existent {}")},
		},
	}

	got, err := processors.Decode()(batch)
	require.NoError(t, err)
	require.Len(t, got.Events, 1, "unknown messages are not decoded into events")
	_, isDecoded := got.Events[0].(processors.DecodedGMCP)
	assert.False(t, isDecoded)
}

func TestDecode_IgnoresNonFrameEvents(t *testing.T) {
	batch := app.Batch{
		Events: []app.Event{
			connection.TextLine{Bytes: []byte("not gmcp")},
		},
	}

	got, err := processors.Decode()(batch)
	require.NoError(t, err)
	require.Len(t, got.Events, 1)
}
