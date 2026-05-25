package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

// feedSent and feedFrame are tiny helpers so the lag tests read at
// the level of "send fired" / "reply landed", not channel plumbing.
func feedSent(t *testing.T, proc app.Processor, eff app.Effect) {
	t.Helper()

	_, err := proc(app.Batch{Event: connection.Sent{Effect: eff}})
	require.NoError(t, err)
}

func feedFrame(t *testing.T, proc app.Processor, payload []byte) app.Batch {
	t.Helper()

	got, err := proc(app.Batch{Event: connection.GMCPFrame{Payload: payload}})
	require.NoError(t, err)

	return got
}

func TestLagWatcher_AcceptsCorePingWithLatencyArgument(t *testing.T) {
	// Some servers echo Core.Ping with a measured latency, e.g.
	// "Core.Ping 1234". LagWatcher must accept it (the parser handles
	// the argument; we match on message ID).
	proc := generic.LagWatcher()

	feedSent(t, proc, connection.SendGMCP{Payload: []byte("Core.Ping")})

	got := feedFrame(t, proc, []byte("Core.Ping 1234"))
	require.Len(t, got.Events, 1)
	assert.IsType(t, generic.LagMeasured{}, got.Events[0])
}

func TestLagWatcher_IgnoresNonLatencyBearingSends(t *testing.T) {
	proc := generic.LagWatcher()

	feedSent(t, proc, connection.Send{Bytes: []byte("look")})

	got := feedFrame(t, proc, []byte("Core.Ping"))
	assert.Empty(t, got.Events)
	assert.Empty(t, got.Effects)
}

func TestLagWatcher_FrameWithoutPriorSentIsNoop(t *testing.T) {
	// A Core.Ping reply with no preceding Sent event (LagWatcher
	// missed a startup tick) must not emit a lag measurement against a
	// zero time.
	proc := generic.LagWatcher()

	got := feedFrame(t, proc, []byte("Core.Ping"))
	assert.Empty(t, got.Events)
	assert.Empty(t, got.Effects)
}

func TestLagWatcher_IgnoresUnrelatedGMCPFrames(t *testing.T) {
	proc := generic.LagWatcher()

	feedSent(t, proc, connection.SendGMCP{Payload: []byte("Core.Ping")})

	got := feedFrame(t, proc, []byte(`Char.Vitals {"hp":"100"}`))
	assert.Empty(t, got.Events)
	assert.Empty(t, got.Effects)
}

func TestLagWatcher_IgnoresOtherEvents(t *testing.T) {
	proc := generic.LagWatcher()

	_, err := proc(app.Batch{Event: connection.TextLine{Bytes: []byte("anything")}})
	require.NoError(t, err)
}
