package generic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// fakeClock returns a now() closure backed by a pointer the test can
// advance between feeds — `*at = at.Add(...)` walks wall-clock time
// without touching the real clock.
func fakeClock(at *time.Time) func() time.Time {
	return func() time.Time { return *at }
}

func TestLagWatcher_EmitsLagAndSetLag(t *testing.T) {
	// Deterministic version of "Core.Ping send, Core.Ping reply lands
	// 250ms later → LagMeasured event + ui.SetLag effect". The injected
	// clock removes wall-clock jitter; bumping `at` between the send and
	// the reply is the only way real time enters the assertion.
	at := time.Unix(1_000_000, 0)
	proc := lagWatcher(fakeClock(&at))

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.SendGMCP{Payload: []byte("Core.Ping")},
	}})
	require.NoError(t, err)

	at = at.Add(250 * time.Millisecond)

	got, err := proc(app.Batch{Event: connection.GMCPFrame{Payload: []byte("Core.Ping")}})
	require.NoError(t, err)

	require.Len(t, got.Events, 1)
	measured, ok := got.Events[0].(LagMeasured)
	require.True(t, ok, "expected LagMeasured, got %T", got.Events[0])
	assert.Equal(t, 250*time.Millisecond, measured.Lag)
	assert.Equal(t, at, measured.At)

	require.Len(t, got.Effects, 1)
	set, ok := got.Effects[0].(ui.SetLag)
	require.True(t, ok, "expected ui.SetLag, got %T", got.Effects[0])
	assert.Equal(t, measured.Lag, set.Lag)
}

func TestLagWatcher_PerIDFIFOAvoidsKeepAlivePopByPing(t *testing.T) {
	// If LagWatcher kept a single shared FIFO, a Core.Ping reply would
	// pop the oldest entry — likely a stale Core.KeepAlive timestamp —
	// and report a nonsense lag. With per-ID FIFOs, each reply pops only
	// its own bucket. Injected clock so the bounds are exact instead of
	// jittery.
	at := time.Unix(2_000_000, 0)
	proc := lagWatcher(fakeClock(&at))

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.SendGMCP{Payload: []byte("Core.KeepAlive")},
	}})
	require.NoError(t, err)

	at = at.Add(40 * time.Millisecond)

	_, err = proc(app.Batch{Event: connection.Sent{
		Effect: connection.SendGMCP{Payload: []byte("Core.Ping")},
	}})
	require.NoError(t, err)

	at = at.Add(15 * time.Millisecond)

	got, err := proc(app.Batch{Event: connection.GMCPFrame{Payload: []byte("Core.Ping")}})
	require.NoError(t, err)

	require.Len(t, got.Events, 1)
	measured := got.Events[0].(LagMeasured)
	// Lag must be against the Ping send (15ms), not the older KeepAlive
	// (which would be 55ms if a shared FIFO had popped the wrong entry).
	assert.Equal(t, 15*time.Millisecond, measured.Lag)
}
