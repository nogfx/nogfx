package generic_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/clock"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestPing_WaitsForGMCPNegotiation(t *testing.T) {
	proc := generic.Ping(10 * time.Second)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Unix(0, 0)}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects, "no Send before negotiation")

	primeGMCP(t, proc)

	got, err = proc(app.Batch{Event: clock.Tick{At: time.Unix(1, 0)}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)

	send, ok := got.Effects[0].(connection.SendGMCP)
	require.True(t, ok, "expected SendGMCP, got %T", got.Effects[0])
	assert.Equal(t, "Core.Ping", string(send.Payload))
}

func TestPing_SendsOnFirstTick(t *testing.T) {
	proc := generic.Ping(10 * time.Second)
	primeGMCP(t, proc)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Unix(0, 0)}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)

	send, ok := got.Effects[0].(connection.SendGMCP)
	require.True(t, ok, "expected SendGMCP, got %T", got.Effects[0])
	assert.Equal(t, "Core.Ping", string(send.Payload))
}

func TestPing_RespectsInterval(t *testing.T) {
	proc := generic.Ping(10 * time.Second)
	primeGMCP(t, proc)

	start := time.Unix(1_000_000, 0)

	got, err := proc(app.Batch{Event: clock.Tick{At: start}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)

	got, err = proc(app.Batch{Event: clock.Tick{At: start.Add(5 * time.Second)}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)

	got, err = proc(app.Batch{Event: clock.Tick{At: start.Add(10 * time.Second)}})
	require.NoError(t, err)
	assert.Len(t, got.Effects, 1)
}

func TestPing_ZeroIntervalDisablesSending(t *testing.T) {
	proc := generic.Ping(0)
	primeGMCP(t, proc)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Now()}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
}
