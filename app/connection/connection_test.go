package connection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// TestEventsSatisfyAppEvent guards against accidentally dropping the
// EventMarker embed on a concrete event type.
func TestEventsSatisfyAppEvent(t *testing.T) {
	var (
		_ app.Event = connection.TextLine{}
		_ app.Event = connection.Prompt{}
		_ app.Event = connection.TelnetCommand{}
		_ app.Event = connection.GMCPFrame{}
		_ app.Event = connection.StateChanged{}
	)
}

// TestCommandsSatisfyAppCommand guards against accidentally dropping the
// CommandMarker embed on a concrete command type.
func TestCommandsSatisfyAppCommand(t *testing.T) {
	var (
		_ app.Command = connection.Send{}
		_ app.Command = connection.Reconnect{}
		_ app.Command = connection.Disconnect{}
	)
}

func TestEventPayloads(t *testing.T) {
	assert.Equal(t, []byte("hello"), connection.TextLine{Bytes: []byte("hello")}.Bytes)
	assert.Equal(t, []byte("hi"), connection.Prompt{Bytes: []byte("hi")}.Bytes)
	assert.True(t, connection.StateChanged{Connected: true}.Connected)
}

func TestSendCommand(t *testing.T) {
	send := connection.Send{Bytes: []byte("kick orc")}
	assert.Equal(t, []byte("kick orc"), send.Bytes)
}
