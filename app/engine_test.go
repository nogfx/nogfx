package app_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// fakeConn is a Connection that emits a configurable script of events and
// records every command Apply receives.
type fakeConn struct {
	emit []app.Event

	mu       sync.Mutex
	applied  []app.Command
	sendBack chan struct{}
}

func newFakeConn(emit ...app.Event) *fakeConn {
	return &fakeConn{emit: emit, sendBack: make(chan struct{}, 1)}
}

func (c *fakeConn) Run(ctx context.Context, events chan<- app.Event) error {
	for _, ev := range c.emit {
		select {
		case <-ctx.Done():
			return nil
		case events <- ev:
		}
	}

	<-ctx.Done()

	return nil
}

func (c *fakeConn) Apply(cmd app.Command) error {
	switch cmd.(type) {
	case connection.Send, connection.Reconnect, connection.Disconnect:
		c.mu.Lock()
		c.applied = append(c.applied, cmd)
		c.mu.Unlock()

		select {
		case c.sendBack <- struct{}{}:
		default:
		}

		return nil
	}

	return app.ErrCommandNotApplicable
}

func (c *fakeConn) Applied() []app.Command {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]app.Command{}, c.applied...)
}

// fakeUI mirrors fakeConn for the UI side.
type fakeUI struct {
	emit []app.Event

	mu       sync.Mutex
	applied  []app.Command
	sendBack chan struct{}
}

func newFakeUI(emit ...app.Event) *fakeUI {
	return &fakeUI{emit: emit, sendBack: make(chan struct{}, 1)}
}

func (u *fakeUI) Run(ctx context.Context, events chan<- app.Event) error {
	for _, ev := range u.emit {
		select {
		case <-ctx.Done():
			return nil
		case events <- ev:
		}
	}

	<-ctx.Done()

	return nil
}

func (u *fakeUI) Apply(cmd app.Command) error {
	switch cmd.(type) {
	case ui.PrintLine, ui.SetHealth, ui.SetMana, ui.AddVital, ui.SetVital,
		ui.RemoveVital, ui.SetCharacter, ui.SetTarget, ui.SetRoom,
		ui.MaskInput, ui.UnmaskInput:
		u.mu.Lock()
		u.applied = append(u.applied, cmd)
		u.mu.Unlock()

		select {
		case u.sendBack <- struct{}{}:
		default:
		}

		return nil
	}

	return app.ErrCommandNotApplicable
}

func (u *fakeUI) Applied() []app.Command {
	u.mu.Lock()
	defer u.mu.Unlock()

	return append([]app.Command{}, u.applied...)
}

// runEngine starts an engine, lets it process some events, and stops it.
func runEngine(t *testing.T, conn *fakeConn, gui *fakeUI, proc app.Processor, expectN int) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		e := &app.Engine{
			Connection: conn,
			UI:         gui,
			Processor:  proc,
		}
		done <- e.Run(ctx)
	}()

	// Wait for the expected number of Apply calls, with a short timeout.
	deadline := time.After(time.Second)

	got := 0
	for got < expectN {
		select {
		case <-conn.sendBack:
			got++
		case <-gui.sendBack:
			got++
		case <-deadline:
			t.Fatalf("timed out waiting for Apply calls: got %d/%d", got, expectN)
		}
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("engine did not shut down")
	}
}

func TestEngine_RoutesConnectionCommandsToConnection(t *testing.T) {
	conn := newFakeConn()
	gui := newFakeUI(ui.Input{Bytes: []byte("hello")})

	// A processor that turns ui.Input into a connection.Send.
	proc := func(b app.Batch) (app.Batch, error) {
		if in, ok := b.Event.(ui.Input); ok {
			b = b.AppendCommand(connection.Send{Bytes: in.Bytes})
		}

		return b, nil
	}

	runEngine(t, conn, gui, proc, 1)

	require.Len(t, conn.Applied(), 1)
	assert.Equal(t, []byte("hello"), conn.Applied()[0].(connection.Send).Bytes)
	assert.Empty(t, gui.Applied(), "connection commands must not reach the UI")
}

func TestEngine_RoutesUICommandsToUI(t *testing.T) {
	conn := newFakeConn(connection.TextLine{Bytes: []byte("server says hi")})
	gui := newFakeUI()

	proc := func(b app.Batch) (app.Batch, error) {
		if tl, ok := b.Event.(connection.TextLine); ok {
			b = b.AppendCommand(ui.PrintLine{Line: ui.Line{Raw: tl.Bytes, Formatted: tl.Bytes}})
		}

		return b, nil
	}

	runEngine(t, conn, gui, proc, 1)

	require.Len(t, gui.Applied(), 1)
	assert.Equal(t, []byte("server says hi"), gui.Applied()[0].(ui.PrintLine).Line.Formatted)
	assert.Empty(t, conn.Applied(), "UI commands must not reach the connection")
}

func TestEngine_UnknownCommandIsLoggedNotPanic(t *testing.T) {
	// An unknown command shouldn't crash the engine.
	type orphan struct {
		app.CommandMarker
	}

	conn := newFakeConn(connection.TextLine{Bytes: []byte("trigger")})
	gui := newFakeUI()

	emitOrphan := false
	proc := func(b app.Batch) (app.Batch, error) {
		if !emitOrphan {
			emitOrphan = true

			return b.AppendCommand(orphan{}), nil
		}

		return b, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		e := &app.Engine{Connection: conn, UI: gui, Processor: proc}
		done <- e.Run(ctx)
	}()

	// Give the engine time to process the event.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("engine did not shut down")
	}

	assert.Empty(t, conn.Applied())
	assert.Empty(t, gui.Applied())
}

// fakeFailingConn returns an error from Run; the engine should propagate it.
type fakeFailingConn struct {
	err error
}

func (f *fakeFailingConn) Run(ctx context.Context, events chan<- app.Event) error {
	return f.err
}
func (f *fakeFailingConn) Apply(cmd app.Command) error { return app.ErrCommandNotApplicable }

func TestEngine_PropagatesConnectionError(t *testing.T) {
	boom := errors.New("transport gone")
	conn := &fakeFailingConn{err: boom}
	gui := newFakeUI()

	e := &app.Engine{Connection: conn, UI: gui, Processor: nil}
	err := e.Run(context.Background())
	assert.ErrorIs(t, err, boom)
}
