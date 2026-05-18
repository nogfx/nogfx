package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

// runNeg returns the bytes of the single connection.Send command the
// processor appended, or nil if no command was emitted.
func runNeg(t *testing.T, proc app.Processor, ev app.Event) []byte {
	t.Helper()
	got, err := proc(app.Batch{Event: ev})
	require.NoError(t, err)
	if len(got.Commands) == 0 {
		return nil
	}
	require.Len(t, got.Commands, 1)
	send, ok := got.Commands[0].(connection.Send)
	require.True(t, ok)
	return send.Bytes
}

func TestNegotiation_AcceptsTheirsWillWithDo(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Will, connection.GMCP},
	})
	assert.Equal(t,
		[]byte{connection.IAC, connection.Do, connection.GMCP},
		reply,
	)
}

func TestNegotiation_DeclinesUnconfiguredWill(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Will, 99 /* unknown opt */},
	})
	assert.Equal(t,
		[]byte{connection.IAC, connection.Dont, 99},
		reply,
	)
}

func TestNegotiation_AcceptsOursDoWithWill(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Do, connection.Echo},
	})
	assert.Equal(t,
		[]byte{connection.IAC, connection.Will, connection.Echo},
		reply,
	)
}

func TestNegotiation_DeclinesUnconfiguredDo(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Do, connection.SuppressGoAhead},
	})
	assert.Equal(t,
		[]byte{connection.IAC, connection.Wont, connection.SuppressGoAhead},
		reply,
	)
}

func TestNegotiation_SuppressesRepeatWill(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	// First WILL — accepted, reply is sent.
	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Will, connection.GMCP},
	})
	require.NotNil(t, reply)

	// Second WILL — already enabled on their side, no reply.
	reply = runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Will, connection.GMCP},
	})
	assert.Nil(t, reply, "duplicate WILL should not re-ack")
}

func TestNegotiation_WontAfterWillRepliesDont(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	// Enable first.
	_ = runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Will, connection.GMCP},
	})

	// Server WONT GMCP — we reply DONT.
	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Wont, connection.GMCP},
	})
	assert.Equal(t,
		[]byte{connection.IAC, connection.Dont, connection.GMCP},
		reply,
	)
}

func TestNegotiation_DontWithoutPriorWillIgnored(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	reply := runNeg(t, proc, connection.TelnetCommand{
		Bytes: []byte{connection.IAC, connection.Dont, connection.Echo},
	})
	assert.Nil(t, reply, "DONT for an option we never enabled is a no-op")
}

func TestNegotiation_IgnoresGMCPFrame(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	got, err := proc(app.Batch{Event: connection.GMCPFrame{
		Payload: []byte("Char.Vitals {}"),
	}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}

func TestNegotiation_IgnoresNonTelnetEvents(t *testing.T) {
	proc := generic.TelnetNegotiation(generic.DefaultNegotiation())

	got, err := proc(app.Batch{Event: connection.TextLine{Bytes: []byte("ignored")}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}
