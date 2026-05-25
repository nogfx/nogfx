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
// records every effect Apply receives.
type fakeConn struct {
	emit []app.Event

	// applyEmit, when non-empty, is returned from Apply on every accepted
	// effect. Tests exercising the engine's apply-consequence flow set
	// this to observe how those events thread through the chain.
	applyEmit []app.Event

	mu       sync.Mutex
	applied  []app.Effect
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

func (c *fakeConn) Apply(eff app.Effect) ([]app.Event, error) {
	switch eff.(type) {
	case connection.Send, connection.Disconnect:
		c.mu.Lock()
		c.applied = append(c.applied, eff)
		c.mu.Unlock()

		select {
		case c.sendBack <- struct{}{}:
		default:
		}

		return c.applyEmit, nil
	}

	return nil, app.ErrEffectNotApplicable
}

func (c *fakeConn) Applied() []app.Effect {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]app.Effect{}, c.applied...)
}

// fakeUI mirrors fakeConn for the UI side.
type fakeUI struct {
	emit []app.Event

	mu       sync.Mutex
	applied  []app.Effect
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

func (u *fakeUI) Apply(eff app.Effect) ([]app.Event, error) {
	switch eff.(type) {
	case ui.PrintLine, ui.ReFormat,
		ui.SetHealth, ui.SetMana, ui.AddVital, ui.SetVital,
		ui.RemoveVital, ui.SetCharacter, ui.SetTarget, ui.SetRoom,
		ui.SetLag, ui.MaskInput, ui.UnmaskInput:
		u.mu.Lock()
		u.applied = append(u.applied, eff)
		u.mu.Unlock()

		select {
		case u.sendBack <- struct{}{}:
		default:
		}

		return nil, nil
	}

	return nil, app.ErrEffectNotApplicable
}

func (u *fakeUI) Applied() []app.Effect {
	u.mu.Lock()
	defer u.mu.Unlock()

	return append([]app.Effect{}, u.applied...)
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

func TestEngine_RoutesConnectionEffectsToConnection(t *testing.T) {
	conn := newFakeConn()
	gui := newFakeUI(ui.Input{Bytes: []byte("hello")})

	// A processor that turns ui.Input into a connection.Send.
	proc := func(b app.Batch) (app.Batch, error) {
		if in, ok := b.Event.(ui.Input); ok {
			b = b.AppendEffect(connection.Send{Bytes: in.Bytes})
		}

		return b, nil
	}

	runEngine(t, conn, gui, proc, 1)

	require.Len(t, conn.Applied(), 1)
	assert.Equal(t, []byte("hello"), conn.Applied()[0].(connection.Send).Bytes)
	assert.Empty(t, gui.Applied(), "connection effects must not reach the UI")
}

func TestEngine_RoutesUIEffectsToUI(t *testing.T) {
	conn := newFakeConn(connection.TextLine{Bytes: []byte("server says hi")})
	gui := newFakeUI()

	proc := func(b app.Batch) (app.Batch, error) {
		if tl, ok := b.Event.(connection.TextLine); ok {
			b = b.AppendEffect(ui.PrintLine{Line: ui.Line{Raw: tl.Bytes, Formatted: tl.Bytes}})
		}

		return b, nil
	}

	runEngine(t, conn, gui, proc, 1)

	require.Len(t, gui.Applied(), 1)
	assert.Equal(t, []byte("server says hi"), gui.Applied()[0].(ui.PrintLine).Line.Formatted)
	assert.Empty(t, conn.Applied(), "UI effects must not reach the connection")
}

func TestEngine_UnknownEffectIsLoggedNotPanic(t *testing.T) {
	// An unknown effect shouldn't crash the engine.
	type orphan struct {
		app.EffectMarker
	}

	conn := newFakeConn(connection.TextLine{Bytes: []byte("trigger")})
	gui := newFakeUI()

	emitOrphan := false
	proc := func(b app.Batch) (app.Batch, error) {
		if !emitOrphan {
			emitOrphan = true

			return b.AppendEffect(orphan{}), nil
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
func (f *fakeFailingConn) Apply(_ app.Effect) ([]app.Event, error) {
	return nil, app.ErrEffectNotApplicable
}

// fakeSource is an emission-only Endpoint used to exercise Engine.Sources.
// It pushes the configured events into the engine's channel and then
// blocks on ctx.Done, mirroring the contract a Ticker satisfies. Apply
// always returns ErrEffectNotApplicable.
type fakeSource struct {
	emit []app.Event
}

func (s *fakeSource) Run(ctx context.Context, events chan<- app.Event) error {
	for _, ev := range s.emit {
		select {
		case <-ctx.Done():
			return nil
		case events <- ev:
		}
	}

	<-ctx.Done()

	return nil
}

func (*fakeSource) Apply(_ app.Effect) ([]app.Event, error) {
	return nil, app.ErrEffectNotApplicable
}

// sourceMarker is a private event the test uses to confirm a Source's
// emissions actually flow through the processor chain.
type sourceMarker struct{ app.EventMarker }

// orderProbe is an event the ordering test uses to observe the
// sequence in which the engine re-emits apply-consequence events and
// processor-derived events into the chain.
type orderProbe struct {
	app.EventMarker
	tag string
}

func TestEngine_ApplyConsequenceEventsPrecedeProcessorDerived(t *testing.T) {
	// The engine queues apply-consequence events (returned by an
	// endpoint's Apply) ahead of processor-derived events from the
	// same batch. Without this contract, Recorder would see a derived
	// event referencing a queue entry before the Sent that adds it.
	conn := newFakeConn(connection.TextLine{Bytes: []byte("trigger")})
	conn.applyEmit = []app.Event{orderProbe{tag: "apply"}}
	gui := newFakeUI()

	var (
		mu        sync.Mutex
		seen      []string
		triggered bool
	)

	proc := func(b app.Batch) (app.Batch, error) {
		if probe, ok := b.Event.(orderProbe); ok {
			mu.Lock()
			defer mu.Unlock()

			seen = append(seen, probe.tag)

			return b, nil
		}

		if tl, ok := b.Event.(connection.TextLine); ok &&
			string(tl.Bytes) == "trigger" && !triggered {
			triggered = true
			b = b.AppendEffect(connection.Send{Bytes: []byte("dummy")})
			b = b.AppendEvent(orderProbe{tag: "derived"})
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

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()

		return len(seen) >= 2
	}, time.Second, 10*time.Millisecond, "both probes should flow through the chain")

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("engine did not shut down")
	}

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, []string{"apply", "derived"}, seen,
		"apply-consequence event must reach the chain before processor-derived event")
}

func TestEngine_SourcesDeliverEventsIntoChain(t *testing.T) {
	conn := newFakeConn()
	gui := newFakeUI()
	src := &fakeSource{emit: []app.Event{sourceMarker{}}}

	// A processor that converts the source's marker into a UI effect we
	// can observe via the fake UI's Applied().
	proc := func(b app.Batch) (app.Batch, error) {
		if _, ok := b.Event.(sourceMarker); ok {
			b = b.AppendEffect(ui.PrintLine{Line: ui.Line{Formatted: []byte("source-fired")}})
		}

		return b, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)

	go func() {
		e := &app.Engine{
			Connection: conn,
			UI:         gui,
			Sources:    []app.Endpoint{src},
			Processor:  proc,
		}
		done <- e.Run(ctx)
	}()

	select {
	case <-gui.sendBack:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the Source's event to reach the chain")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("engine did not shut down")
	}

	require.Len(t, gui.Applied(), 1)
	line, ok := gui.Applied()[0].(ui.PrintLine)
	require.True(t, ok, "expected ui.PrintLine, got %T", gui.Applied()[0])
	assert.Equal(t, []byte("source-fired"), line.Line.Formatted)
}

func TestEngine_PropagatesConnectionError(t *testing.T) {
	boom := errors.New("transport gone")
	conn := &fakeFailingConn{err: boom}
	gui := newFakeUI()

	e := &app.Engine{Connection: conn, UI: gui, Processor: nil}
	err := e.Run(context.Background())
	assert.ErrorIs(t, err, boom)
}
