package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	appclock "github.com/nogfx/nogfx/app/clock"
	"github.com/nogfx/nogfx/platform/clock"
)

func TestTicker_EmitsAtInterval(t *testing.T) {
	ticker := clock.NewTicker(20 * time.Millisecond)
	events := make(chan app.Event, 4)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- ticker.Run(ctx, events) }()

	// Three ticks at 20ms cadence ought to land comfortably within 200ms;
	// the buffer absorbs jitter.
	for i := 0; i < 3; i++ {
		select {
		case ev := <-events:
			tick, ok := ev.(appclock.Tick)
			require.True(t, ok, "expected clock.Tick, got %T", ev)
			assert.False(t, tick.At.IsZero(), "Tick.At must carry a timestamp")
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timed out waiting for tick %d", i)
		}
	}

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Ticker.Run did not return after cancel")
	}
}

func TestTicker_ZeroIntervalIsIdle(t *testing.T) {
	ticker := clock.NewTicker(0)
	events := make(chan app.Event, 1)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)

	go func() { done <- ticker.Run(ctx, events) }()

	select {
	case ev := <-events:
		t.Fatalf("unexpected emission from zero-interval ticker: %T", ev)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("Ticker.Run did not return after cancel")
	}
}

func TestTicker_ApplyIsNoop(t *testing.T) {
	ticker := clock.NewTicker(time.Second)
	events, err := ticker.Apply(nil)
	require.ErrorIs(t, err, app.ErrEffectNotApplicable)
	assert.Empty(t, events)
}
