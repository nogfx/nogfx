package telnet_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/telnet"
)

// collect runs the NVT against the given server bytes for up to a short
// window, then closes the connection and returns the events the Run loop
// emitted in order.
func collect(t *testing.T, serverOutput []byte) []app.Event {
	t.Helper()

	conn := NewMockConn(serverOutput)
	client := telnet.NewNVT(conn)

	events := make(chan app.Event, 32)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- client.Run(ctx, events) }()

	// Wait for Run to finish (mock conn EOFs once bytes are consumed).
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Run did not return in time")
	}

	close(events)

	var got []app.Event
	for ev := range events {
		got = append(got, ev)
	}

	return got
}

func TestRun_GAOnlyBecomesPrompt(t *testing.T) {
	got := collect(t, []byte{'h', ':', '1', telnet.IAC, telnet.GA})

	// Filter to just text/prompt events for clarity.
	var (
		prompts []connection.Prompt
		lines   []connection.TextLine
	)

	for _, ev := range got {
		switch e := ev.(type) {
		case connection.Prompt:
			prompts = append(prompts, e)
		case connection.TextLine:
			lines = append(lines, e)
		}
	}

	require.Len(t, prompts, 1, "got events: %#v", got)
	assert.Equal(t, "h:1", string(prompts[0].Bytes))
	assert.Empty(t, lines, "no TextLines for a single-line prompt")
}

func TestRun_MultiLinePromptSplitsOnCRLF(t *testing.T) {
	// Mirrors how Achaea sends a "look" response: several lines of text
	// followed by the vitals prompt, all terminated by a single IAC GA.
	got := collect(t, []byte(
		"Lakeside highway. (road)\r\nYou see exits leading north and east.\r\nh:550 m:500 ex-"+
			"\xff\xf9", // IAC GA
	))

	var (
		prompts []connection.Prompt
		lines   []connection.TextLine
	)

	for _, ev := range got {
		switch e := ev.(type) {
		case connection.Prompt:
			prompts = append(prompts, e)
		case connection.TextLine:
			lines = append(lines, e)
		}
	}

	require.Len(t, lines, 2, "expected TextLines for each \\r\\n-delimited body line")
	assert.Equal(t, "Lakeside highway. (road)", string(lines[0].Bytes))
	assert.Equal(t, "You see exits leading north and east.", string(lines[1].Bytes))

	require.Len(t, prompts, 1, "the final line is the prompt")
	assert.Equal(t, "h:550 m:500 ex-", string(prompts[0].Bytes))
}

func TestApply_EmitsSentForSend(t *testing.T) {
	conn := NewMockConn(nil)
	nvt := telnet.NewNVT(conn)

	events, err := nvt.Apply(connection.Send{Bytes: []byte("kick rat")})
	require.NoError(t, err)
	assert.Equal(t, "kick rat\r\n", string(conn.Written))

	require.Len(t, events, 1)
	sent, ok := events[0].(connection.Sent)
	require.True(t, ok, "expected connection.Sent, got %T", events[0])

	send, ok := sent.Effect.(connection.Send)
	require.True(t, ok, "expected Sent.Effect to be connection.Send, got %T", sent.Effect)
	assert.Equal(t, []byte("kick rat"), send.Bytes)
}

func TestApply_EmitsSentForSendGMCP(t *testing.T) {
	conn := NewMockConn(nil)
	nvt := telnet.NewNVT(conn)

	events, err := nvt.Apply(connection.SendGMCP{Payload: []byte("Core.Ping")})
	require.NoError(t, err)
	// Wire bytes wrap the payload in IAC SB GMCP ... IAC SE.
	assert.Equal(t,
		[]byte{telnet.IAC, telnet.SB, telnet.GMCP, 'C', 'o', 'r', 'e', '.', 'P', 'i', 'n', 'g', telnet.IAC, telnet.SE},
		conn.Written,
	)

	require.Len(t, events, 1)
	sent, ok := events[0].(connection.Sent)
	require.True(t, ok, "expected connection.Sent, got %T", events[0])

	sg, ok := sent.Effect.(connection.SendGMCP)
	require.True(t, ok, "expected Sent.Effect to be connection.SendGMCP, got %T", sent.Effect)
	assert.Equal(t, []byte("Core.Ping"), sg.Payload)
}

func TestApply_DisconnectClosesUnderlyingConn(t *testing.T) {
	conn := NewMockConn(nil)
	nvt := telnet.NewNVT(conn)

	events, err := nvt.Apply(connection.Disconnect{})
	require.NoError(t, err)
	assert.Empty(t, events, "Disconnect emits no apply-consequence events")
	assert.True(t, conn.Closed, "Disconnect must close the underlying conn")
}

func TestApply_DoesNotEmitSentOnWriteError(t *testing.T) {
	conn := NewMockConn(nil)
	conn.WriteErr = func([]byte) error { return assert.AnError }
	nvt := telnet.NewNVT(conn)

	events, err := nvt.Apply(connection.Send{Bytes: []byte("anything")})
	require.Error(t, err)
	assert.Empty(t, events, "Sent must not fire when the wire write failed")
}

func TestRun_DoubledCRLFAtStart(t *testing.T) {
	// Achaea's welcome banner begins with \r\n and contains many empty
	// lines. The splitter shouldn't drop them — empty lines are part of
	// the visible scrollback.
	got := collect(t, []byte("\r\nfirst\r\n\r\nh:1 -\xff\xf9"))

	var (
		prompts []connection.Prompt
		lines   []connection.TextLine
	)

	for _, ev := range got {
		switch e := ev.(type) {
		case connection.Prompt:
			prompts = append(prompts, e)
		case connection.TextLine:
			lines = append(lines, e)
		}
	}

	require.Len(t, lines, 3)
	assert.Empty(t, lines[0].Bytes)
	assert.Equal(t, "first", string(lines[1].Bytes))
	assert.Empty(t, lines[2].Bytes)

	require.Len(t, prompts, 1)
	assert.Equal(t, "h:1 -", string(prompts[0].Bytes))
}
