package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestDecode_KnownMessage(t *testing.T) {
	batch := app.Batch{
		Event: connection.GMCPFrame{Payload: []byte(`Char.Name {"name":"asdf","fullname":"AsDf"}`)},
	}

	got, err := generic.Decode()(batch)
	require.NoError(t, err)
	require.Len(t, got.Events, 1, "Decode appends a DecodedGMCP derived event")

	decoded, ok := got.Events[0].(generic.DecodedGMCP)
	require.True(t, ok)

	name, ok := decoded.Message.(*gmcp.CharName)
	require.True(t, ok)
	assert.Equal(t, "asdf", name.Name)
	assert.Equal(t, "AsDf", name.Fullname)
}

func TestDecode_UnknownMessage_SilentlySkipped(t *testing.T) {
	batch := app.Batch{
		Event: connection.GMCPFrame{Payload: []byte("Non.Existent {}")},
	}

	got, err := generic.Decode()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Events, "unknown messages are not decoded into events")
}

func TestDecode_IgnoresNonFrameEvents(t *testing.T) {
	batch := app.Batch{
		Event: connection.TextLine{Bytes: []byte("not gmcp")},
	}

	got, err := generic.Decode()(batch)
	require.NoError(t, err)
	assert.Empty(t, got.Events)
	assert.Equal(t, batch.Event, got.Event)
}
