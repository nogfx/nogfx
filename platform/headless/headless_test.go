package headless_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/platform/headless"
)

func TestHeadless_RunEmitsInputPerLine(t *testing.T) {
	in := strings.NewReader("look\nscore\nquit\n")
	h := headless.NewWithIO(in, &bytes.Buffer{})

	events := make(chan app.Event, 8)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() { done <- h.Run(ctx, events) }()

	var got []string

	for i := 0; i < 3; i++ {
		select {
		case ev := <-events:
			input, ok := ev.(ui.Input)
			require.True(t, ok, "expected ui.Input, got %T", ev)

			got = append(got, string(input.Bytes))
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for input %d", i)
		}
	}

	cancel()

	assert.Equal(t, []string{"look", "score", "quit"}, got)

	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestHeadless_RunReturnsNilOnEOF(t *testing.T) {
	h := headless.NewWithIO(strings.NewReader(""), &bytes.Buffer{})
	events := make(chan app.Event, 1)
	err := h.Run(context.Background(), events)
	assert.NoError(t, err)
}

func TestHeadless_ApplyPrintLineWritesToOutput(t *testing.T) {
	out := &bytes.Buffer{}
	h := headless.NewWithIO(strings.NewReader(""), out)

	_, err := h.Apply(ui.PrintLine{Line: ui.Line{Formatted: []byte("you see an orc")}})
	require.NoError(t, err)

	assert.Equal(t, "you see an orc\n", out.String())
}

func TestHeadless_ApplyAcceptsUIEffectsAsNoOps(t *testing.T) {
	h := headless.New()

	cases := []app.Effect{
		ui.ReFormat{},
		ui.SetHealth{Value: 100, Max: 100},
		ui.SetMana{Value: 100, Max: 100},
		ui.AddVital{Name: "endurance", Value: 100, Max: 100},
		ui.SetVital{Name: "endurance", Value: 50, Max: 100},
		ui.RemoveVital{Name: "endurance"},
		ui.SetCharacter{Name: "x", Title: "y"},
		ui.SetTarget{Target: nil},
		ui.SetRoom{Room: nil},
		ui.MaskInput{},
		ui.UnmaskInput{},
	}
	for _, c := range cases {
		_, err := h.Apply(c)
		assert.NoError(t, err, "expected %T to be accepted", c)
	}
}

type notAnEffect struct{ app.EffectMarker }

func TestHeadless_ApplyReturnsErrEffectNotApplicableForOthers(t *testing.T) {
	h := headless.New()
	_, err := h.Apply(notAnEffect{})
	assert.ErrorIs(t, err, app.ErrEffectNotApplicable)
}
