package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

// feed drives the aggregator through a sequence of trigger events and
// returns the Messages emitted as derived events (in arrival order). It
// also reports whether every non-Prompt input flowed through with its
// trigger event intact, since the aggregator is purely additive.
func feed(t *testing.T, agg app.Processor, events ...app.Event) []connection.Message {
	t.Helper()

	var out []connection.Message
	for _, ev := range events {
		got, err := agg(app.Batch{Event: ev})
		require.NoError(t, err)

		// Trigger event must always survive the aggregator.
		assert.Equal(t, ev, got.Event, "aggregator must not consume the trigger event")

		for _, derived := range got.Events {
			if msg, ok := derived.(connection.Message); ok {
				out = append(out, msg)
			}
		}
	}
	return out
}

func TestAggregator_EmitsMessageOnPrompt(t *testing.T) {
	agg := generic.Aggregator()

	msgs := feed(t, agg,
		connection.TextLine{Bytes: []byte("you arrive at a small clearing.")},
		connection.TextLine{Bytes: []byte("a bird sings overhead.")},
		connection.Prompt{Bytes: []byte("h:100 m:100 -")},
	)

	require.Len(t, msgs, 1)
	assert.Equal(t, [][]byte{
		[]byte("you arrive at a small clearing."),
		[]byte("a bird sings overhead."),
	}, [][]byte{msgs[0].Lines[0].Bytes, msgs[0].Lines[1].Bytes})
	assert.Equal(t, "h:100 m:100 -", string(msgs[0].Prompt.Bytes))
	assert.Empty(t, msgs[0].GMCP)
}

func TestAggregator_IncludesGMCP(t *testing.T) {
	agg := generic.Aggregator()

	msgs := feed(t, agg,
		connection.GMCPFrame{Payload: []byte(`Char.Vitals {"hp":"100"}`)},
		connection.TextLine{Bytes: []byte("you sip an elixir.")},
		connection.GMCPFrame{Payload: []byte(`Char.Items.Update {}`)},
		connection.Prompt{Bytes: []byte("h:100 m:100 -")},
	)

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].GMCP, 2)
	assert.Equal(t, `Char.Vitals {"hp":"100"}`, string(msgs[0].GMCP[0].Payload))
	assert.Equal(t, `Char.Items.Update {}`, string(msgs[0].GMCP[1].Payload))
	require.Len(t, msgs[0].Lines, 1)
	assert.Equal(t, "you sip an elixir.", string(msgs[0].Lines[0].Bytes))
}

func TestAggregator_ResetsBetweenPrompts(t *testing.T) {
	agg := generic.Aggregator()

	msgs := feed(t, agg,
		connection.TextLine{Bytes: []byte("first")},
		connection.Prompt{Bytes: []byte("h:1 -")},
		connection.TextLine{Bytes: []byte("second")},
		connection.Prompt{Bytes: []byte("h:2 -")},
	)

	require.Len(t, msgs, 2)
	require.Len(t, msgs[0].Lines, 1)
	assert.Equal(t, "first", string(msgs[0].Lines[0].Bytes))
	require.Len(t, msgs[1].Lines, 1)
	assert.Equal(t, "second", string(msgs[1].Lines[0].Bytes))
}

func TestAggregator_EmptyPromptStillEmits(t *testing.T) {
	agg := generic.Aggregator()

	msgs := feed(t, agg,
		connection.Prompt{Bytes: []byte("h:100 -")},
	)

	require.Len(t, msgs, 1)
	assert.Empty(t, msgs[0].Lines)
	assert.Empty(t, msgs[0].GMCP)
	assert.Equal(t, "h:100 -", string(msgs[0].Prompt.Bytes))
}

func TestAggregator_OrphanGMCPJoinsNextMessage(t *testing.T) {
	// This is the empirical case from the probe session: GMCP frames
	// arrive after a Prompt with no following prompt of their own. The
	// aggregator must buffer them until the next prompt eventually fires,
	// and that next message must include them.
	agg := generic.Aggregator()

	msgs := feed(t, agg,
		// First turn — a normal prompt closes a message.
		connection.TextLine{Bytes: []byte("Welcome.")},
		connection.Prompt{Bytes: []byte("h:100 -")},

		// Three orphan GMCP frames between turns.
		connection.GMCPFrame{Payload: []byte("Char.Items.List []")},
		connection.GMCPFrame{Payload: []byte("Comm.Channel.Players []")},
		connection.GMCPFrame{Payload: []byte("IRE.Rift.List []")},

		// Eventually the user does something; a new turn closes.
		connection.TextLine{Bytes: []byte("You see an orc.")},
		connection.Prompt{Bytes: []byte("h:100 -")},
	)

	require.Len(t, msgs, 2)
	assert.Empty(t, msgs[0].GMCP, "first turn had no GMCP of its own")

	require.Len(t, msgs[1].GMCP, 3, "orphan GMCP frames must attach to the next message")
	assert.Equal(t, "Char.Items.List []", string(msgs[1].GMCP[0].Payload))
	assert.Equal(t, "Comm.Channel.Players []", string(msgs[1].GMCP[1].Payload))
	assert.Equal(t, "IRE.Rift.List []", string(msgs[1].GMCP[2].Payload))
}

func TestAggregator_PassesUnrelatedEventsThrough(t *testing.T) {
	agg := generic.Aggregator()

	// TelnetCommand, StateChanged: not relevant to message aggregation;
	// they must pass through with no derived event.
	for _, ev := range []app.Event{
		connection.TelnetCommand{Bytes: []byte{255, 251, 201}},
		connection.StateChanged{Connected: true},
	} {
		got, err := agg(app.Batch{Event: ev})
		require.NoError(t, err)
		assert.Equal(t, ev, got.Event)
		assert.Empty(t, got.Events, "no Message derived from %T", ev)
	}
}

func TestAggregator_PreservesLineAndFrameIdentity(t *testing.T) {
	// The Message must carry the same TextLine and GMCPFrame values
	// (same Bytes/Payload) the chain saw — no copy, no reslice, no
	// trimming. Consumers comparing by reference should still work.
	agg := generic.Aggregator()

	line := connection.TextLine{Bytes: []byte("you see an orc.")}
	frame := connection.GMCPFrame{Payload: []byte("Char.Vitals {}")}

	msgs := feed(t, agg, line, frame, connection.Prompt{Bytes: []byte("h:1 -")})

	require.Len(t, msgs, 1)
	require.Len(t, msgs[0].Lines, 1)
	require.Len(t, msgs[0].GMCP, 1)
	assert.Equal(t, line, msgs[0].Lines[0])
	assert.Equal(t, frame, msgs[0].GMCP[0])
}
