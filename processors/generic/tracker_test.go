package generic_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

// matchExact returns a predicate that matches a pending command equal
// to the given string. Used in many of these tests.
func matchExact(want string) func(string) bool {
	return func(cmd string) bool { return cmd == want }
}

func TestTracker_RecordAndResolve(t *testing.T) {
	tracker := generic.NewTracker()

	tracker.Record("kick rat")

	reply := connection.TextLine{Bytes: []byte("You kick a rat.")}
	events := tracker.Resolve(matchExact("kick rat"), reply)

	require.Len(t, events, 1)
	resolved, ok := events[0].(generic.Resolved)
	require.True(t, ok, "expected Resolved, got %T", events[0])

	assert.Equal(t, "kick rat", resolved.Command)
	assert.Equal(t, reply, resolved.Reply)
	assert.Empty(t, tracker.List(), "resolved entry must be dropped")
}

func TestTracker_ResolveEmitsTimeoutsForPreceding(t *testing.T) {
	tracker := generic.NewTracker()

	tracker.Record("look")
	tracker.Record("smile")
	tracker.Record("kick rat")

	reply := connection.TextLine{Bytes: []byte("You kick a rat.")}
	events := tracker.Resolve(matchExact("kick rat"), reply)

	require.Len(t, events, 3, "two Timeouts + one Resolved")

	t0, ok := events[0].(generic.Timeout)
	require.True(t, ok, "events[0] should be Timeout, got %T", events[0])
	assert.Equal(t, "look", t0.Command)
	assert.Equal(t, reply, t0.Reason)

	t1, ok := events[1].(generic.Timeout)
	require.True(t, ok, "events[1] should be Timeout, got %T", events[1])
	assert.Equal(t, "smile", t1.Command)

	r, ok := events[2].(generic.Resolved)
	require.True(t, ok, "events[2] should be Resolved, got %T", events[2])
	assert.Equal(t, "kick rat", r.Command)
}

func TestTracker_ResolveNoMatchLeavesQueueIntact(t *testing.T) {
	tracker := generic.NewTracker()

	tracker.Record("kick rat")

	events := tracker.Resolve(func(cmd string) bool { return false },
		connection.TextLine{Bytes: []byte("anything")})

	assert.Empty(t, events)
	assert.Len(t, tracker.List(), 1, "queue must be untouched on no-match")
}

func TestTracker_ListIsACopy(t *testing.T) {
	tracker := generic.NewTracker()

	tracker.Record("first")

	snap := tracker.List()
	require.Len(t, snap, 1)

	snap[0] = "mutated"

	again := tracker.List()
	assert.Equal(t, "first", again[0], "List mutation must not leak back into Tracker")
}

func TestTracker_FindReturnsMatchWithoutMutating(t *testing.T) {
	tracker := generic.NewTracker()

	tracker.Record("look")
	tracker.Record("kick rat")

	got, ok := tracker.Find(func(cmd string) bool {
		return strings.HasPrefix(cmd, "kick ")
	})
	require.True(t, ok)
	assert.Equal(t, "kick rat", got)
	assert.Len(t, tracker.List(), 2, "Find must not mutate the queue")
}

func TestTracker_RecordEmptyStringIsNoop(t *testing.T) {
	// Empty strings never enter the queue. The Recorder also strips
	// them at its layer; this is the inner guard so a world-specific
	// caller that does Record("") doesn't accidentally pollute the
	// queue either.
	tracker := generic.NewTracker()

	tracker.Record("")

	assert.Empty(t, tracker.List())
}

func TestRecorder_RecordsConnectionSendText(t *testing.T) {
	tracker := generic.NewTracker()
	proc := generic.Recorder(tracker)

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.Send{Bytes: []byte("look")},
	}})
	require.NoError(t, err)

	_, err = proc(app.Batch{Event: connection.Sent{
		Effect: connection.Send{Bytes: []byte("kick rat")},
	}})
	require.NoError(t, err)

	assert.Equal(t, []string{"look", "kick rat"}, tracker.List())
}

func TestRecorder_SkipsEmptySend(t *testing.T) {
	// A player pressing Enter on a blank input emits
	// connection.Send{Bytes: nil}. That's a routine interaction
	// (refresh prompt / page through output), not a MUD command, and
	// must not become a queue entry.
	tracker := generic.NewTracker()
	proc := generic.Recorder(tracker)

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.Send{Bytes: nil},
	}})
	require.NoError(t, err)

	_, err = proc(app.Batch{Event: connection.Sent{
		Effect: connection.Send{Bytes: []byte{}},
	}})
	require.NoError(t, err)

	assert.Empty(t, tracker.List())
}

func TestRecorder_SkipsSendGMCP(t *testing.T) {
	// GMCP traffic isn't a MUD command in the domain sense — it's
	// protocol. Recorder must not put GMCP sends into the queue.
	tracker := generic.NewTracker()
	proc := generic.Recorder(tracker)

	_, err := proc(app.Batch{Event: connection.Sent{
		Effect: connection.SendGMCP{Payload: []byte("Core.Ping")},
	}})
	require.NoError(t, err)

	assert.Empty(t, tracker.List())
}

func TestRecorder_IgnoresOtherEvents(t *testing.T) {
	tracker := generic.NewTracker()
	proc := generic.Recorder(tracker)

	_, err := proc(app.Batch{Event: connection.TextLine{Bytes: []byte("anything")}})
	require.NoError(t, err)

	_, err = proc(app.Batch{Event: connection.GMCPFrame{Payload: []byte("Core.Ping")}})
	require.NoError(t, err)

	assert.Empty(t, tracker.List())
}

func TestRecorder_IgnoresEffectsInBatch(t *testing.T) {
	// Recorder must NOT read batch.Effects. A later processor (or the
	// engine's GuardedEvent filter) can mutate the slice before Apply
	// dispatches; the connection.Sent event is the authoritative signal.
	tracker := generic.NewTracker()
	proc := generic.Recorder(tracker)

	_, err := proc(app.Batch{
		Event:   connection.TextLine{Bytes: []byte("noop")},
		Effects: []app.Effect{connection.Send{Bytes: []byte("look")}},
	})
	require.NoError(t, err)

	assert.Empty(t, tracker.List(), "Recorder must rely on Sent events, not batch.Effects")
}
