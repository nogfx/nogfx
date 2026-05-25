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

// primeGMCP latches a heartbeat processor's gmcpReady flag by feeding
// it the connection.Sent event TelnetNegotiation emits when we accept
// the server's WILL GMCP (Send{Bytes: IAC DO GMCP}). Without this, the
// negotiation gate suppresses every send and most tests can't observe
// the behaviour they're after. Shared between KeepAlive and Ping tests.
func primeGMCP(t *testing.T, proc app.Processor) {
	t.Helper()

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.Send{Bytes: connection.IACDoGMCP},
	}})
	require.NoError(t, err)
}

func TestKeepAlive_WaitsForGMCPNegotiation(t *testing.T) {
	proc := generic.KeepAlive(30 * time.Second)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Unix(0, 0)}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects, "no Send before negotiation")

	primeGMCP(t, proc)

	got, err = proc(app.Batch{Event: clock.Tick{At: time.Unix(1, 0)}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)
}

func TestKeepAlive_SendsOnFirstTick(t *testing.T) {
	proc := generic.KeepAlive(30 * time.Second)
	primeGMCP(t, proc)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Unix(0, 0)}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)

	send, ok := got.Effects[0].(connection.SendGMCP)
	require.True(t, ok, "expected SendGMCP, got %T", got.Effects[0])
	assert.Equal(t, "Core.KeepAlive", string(send.Payload))
}

func TestKeepAlive_RespectsInterval(t *testing.T) {
	proc := generic.KeepAlive(30 * time.Second)
	primeGMCP(t, proc)

	start := time.Unix(1_000_000, 0)

	got, err := proc(app.Batch{Event: clock.Tick{At: start}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)

	got, err = proc(app.Batch{Event: clock.Tick{At: start.Add(15 * time.Second)}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)

	got, err = proc(app.Batch{Event: clock.Tick{At: start.Add(30 * time.Second)}})
	require.NoError(t, err)
	assert.Len(t, got.Effects, 1)
}

func TestKeepAlive_ZeroIntervalDisablesSending(t *testing.T) {
	proc := generic.KeepAlive(0)
	primeGMCP(t, proc)

	got, err := proc(app.Batch{Event: clock.Tick{At: time.Now()}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
}

func TestKeepAlive_IgnoresUnrelatedEvents(t *testing.T) {
	proc := generic.KeepAlive(30 * time.Second)
	primeGMCP(t, proc)

	got, err := proc(app.Batch{Event: connection.TextLine{Bytes: []byte("anything")}})
	require.NoError(t, err)
	assert.Empty(t, got.Effects)
	assert.Empty(t, got.Events)
}

// TestKeepAlive_SharedPayloadStaysStable pins the read-only contract on
// connection.SendGMCP.Payload: the heartbeat hands the same backing array
// to every emission, so a downstream mutation would leak into the next
// send. If a future change reintroduces an in-place mutation (or breaks
// the no-mutation contract elsewhere), the second send's payload would
// diverge from "Core.KeepAlive" and this test fails.
func TestKeepAlive_SharedPayloadStaysStable(t *testing.T) {
	proc := generic.KeepAlive(30 * time.Second)
	primeGMCP(t, proc)

	start := time.Unix(2_000_000, 0)

	first, err := proc(app.Batch{Event: clock.Tick{At: start}})
	require.NoError(t, err)
	require.Len(t, first.Effects, 1)

	p1 := first.Effects[0].(connection.SendGMCP).Payload
	assert.Equal(t, "Core.KeepAlive", string(p1))

	second, err := proc(app.Batch{Event: clock.Tick{At: start.Add(30 * time.Second)}})
	require.NoError(t, err)
	require.Len(t, second.Effects, 1)

	p2 := second.Effects[0].(connection.SendGMCP).Payload
	assert.Equal(t, "Core.KeepAlive", string(p2),
		"shared payload must be byte-stable across emissions; "+
			"a divergence here means someone mutated the slice in flight")
}
