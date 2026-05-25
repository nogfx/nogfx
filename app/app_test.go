package app_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
)

type fakeEvent struct {
	app.EventMarker
	Name string
}

type fakeCommand struct {
	app.EffectMarker
	Name string
}

func TestBatchAppend(t *testing.T) {
	var b app.Batch

	b = b.AppendEvent(fakeEvent{Name: "first"})
	b = b.AppendEvent(fakeEvent{Name: "second"})
	b = b.AppendEffect(fakeCommand{Name: "do"})

	require.Len(t, b.Events, 2)
	require.Len(t, b.Effects, 1)
	assert.Equal(t, "first", b.Events[0].(fakeEvent).Name)
	assert.Equal(t, "second", b.Events[1].(fakeEvent).Name)
	assert.Equal(t, "do", b.Effects[0].(fakeCommand).Name)
}

func TestBatchAppendIsCopy(t *testing.T) {
	// AppendEvent / AppendEffect return new Batch values; the caller's
	// original Batch is unmodified after Append on a copy (because the
	// internal slice header lives on the value receiver).
	a := app.Batch{}
	b := a.AppendEvent(fakeEvent{Name: "x"})
	assert.Empty(t, a.Events)
	assert.Len(t, b.Events, 1)
}

func TestChain_RunsInOrder(t *testing.T) {
	tag := func(name string) app.Processor {
		return func(b app.Batch) (app.Batch, error) {
			return b.AppendEvent(fakeEvent{Name: name}), nil
		}
	}

	chain := app.Chain(tag("a"), tag("b"), tag("c"))

	got, err := chain(app.Batch{})
	require.NoError(t, err)
	require.Len(t, got.Events, 3)
	assert.Equal(t, "a", got.Events[0].(fakeEvent).Name)
	assert.Equal(t, "b", got.Events[1].(fakeEvent).Name)
	assert.Equal(t, "c", got.Events[2].(fakeEvent).Name)
}

func TestChain_SkipsNilProcessors(t *testing.T) {
	called := false
	mark := func(b app.Batch) (app.Batch, error) {
		called = true

		return b, nil
	}

	chain := app.Chain(nil, mark, nil)
	_, err := chain(app.Batch{})
	require.NoError(t, err)
	assert.True(t, called)
}

func TestChain_PropagatesError(t *testing.T) {
	boom := errors.New("boom")
	fail := func(b app.Batch) (app.Batch, error) {
		return b, boom
	}

	chain := app.Chain(fail)
	_, err := chain(app.Batch{})
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestChain_StopsOnError(t *testing.T) {
	boom := errors.New("boom")
	calls := 0

	count := func(b app.Batch) (app.Batch, error) {
		calls++

		return b, nil
	}
	fail := func(b app.Batch) (app.Batch, error) {
		calls++

		return b, boom
	}

	chain := app.Chain(count, fail, count)
	_, err := chain(app.Batch{})
	require.Error(t, err)
	assert.Equal(t, 2, calls, "processors after the failing one should not run")
}

func TestErrEffectNotApplicableSentinel(t *testing.T) {
	// The wrapping pattern endpoints use should round-trip through
	// errors.Is so the engine's routing can identify "not my effect".
	wrapped := errors.Join(app.ErrEffectNotApplicable, errors.New("ignored"))
	assert.ErrorIs(t, wrapped, app.ErrEffectNotApplicable)
}
