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

func TestAutoLogin_SendsCharLoginOnGMCPWill(t *testing.T) {
	proc := generic.AutoLogin(map[string]string{
		generic.CredentialsUser: "testuser",
		generic.CredentialsPass: "testpass",
	})

	got, err := proc(app.Batch{Event: connection.TelnetCommand{
		Bytes: connection.IACWillGMCP,
	}})
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)

	send, ok := got.Commands[0].(connection.SendGMCP)
	require.True(t, ok, "expected SendGMCP, got %T", got.Commands[0])

	payload := string(send.Payload)
	assert.True(t, strings.HasPrefix(payload, "Char.Login "), "payload: %q", payload)
	assert.Contains(t, payload, `"name":"testuser"`)
	assert.Contains(t, payload, `"password":"testpass"`)
}

func TestAutoLogin_IsSingleUse(t *testing.T) {
	proc := generic.AutoLogin(map[string]string{
		generic.CredentialsUser: "testuser",
		generic.CredentialsPass: "testpass",
	})

	got, err := proc(app.Batch{Event: connection.TelnetCommand{Bytes: connection.IACWillGMCP}})
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)

	got, err = proc(app.Batch{Event: connection.TelnetCommand{Bytes: connection.IACWillGMCP}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands, "credentials must not be re-sent")
}

func TestAutoLogin_IgnoresUnrelatedTelnetCommands(t *testing.T) {
	proc := generic.AutoLogin(map[string]string{
		generic.CredentialsUser: "testuser",
		generic.CredentialsPass: "testpass",
	})

	got, err := proc(app.Batch{Event: connection.TelnetCommand{Bytes: connection.IACWillEcho}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}

func TestAutoLogin_IgnoresUnrelatedEventTypes(t *testing.T) {
	proc := generic.AutoLogin(map[string]string{
		generic.CredentialsUser: "testuser",
		generic.CredentialsPass: "testpass",
	})

	got, err := proc(app.Batch{Event: connection.TextLine{Bytes: []byte("anything")}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}

func TestAutoLogin_EmptyCredentialsIsPassthrough(t *testing.T) {
	proc := generic.AutoLogin(nil)
	got, err := proc(app.Batch{Event: connection.TelnetCommand{Bytes: connection.IACWillGMCP}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}

func TestAutoLogin_MissingPasswordIsPassthrough(t *testing.T) {
	proc := generic.AutoLogin(map[string]string{generic.CredentialsUser: "testuser"})
	got, err := proc(app.Batch{Event: connection.TelnetCommand{Bytes: connection.IACWillGMCP}})
	require.NoError(t, err)
	assert.Empty(t, got.Commands)
}
