package achaea_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/processors/achaea"
)

// runTV feeds each TextLine through the world's chain as its own batch and
// returns the visible scrollback in order: ui.PrintLine commands the
// processor appended (e.g. the consolidated attack summary) followed by
// the surviving TextLine (anything generic.Output would later turn into a
// PrintLine downstream).
func runTV(t *testing.T, events ...connection.TextLine) []string {
	t.Helper()

	w := achaea.New()

	chain := app.Chain(w.Processors()...)

	var out []string
	for _, ev := range events {
		got, err := chain(app.Batch{Event: ev})
		require.NoError(t, err)
		for _, c := range got.Commands {
			if pl, ok := c.(ui.PrintLine); ok {
				out = append(out, string(pl.Line.Formatted))
			}
		}
		if tl, ok := got.Event.(connection.TextLine); ok {
			out = append(out, string(tl.Bytes))
		}
	}
	return out
}

func TestTunnelVision_OmitsBalanceConfirmations(t *testing.T) {
	got := runTV(t,
		line("You may drink another health or mana elixir."),
		line("You see an orc here."),
	)
	assert.Equal(t, []string{"You see an orc here."}, got,
		"balance-recovery lines should be dropped")
}

func TestTunnelVision_OmitsWeather(t *testing.T) {
	got := runTV(t,
		line("Occasional drops of rain fall to the ground from a sky grey with pregnant clouds."),
		line("You see an orc here."),
	)
	assert.Equal(t, []string{"You see an orc here."}, got)
}

func TestTunnelVision_SuppressesPairedCuring(t *testing.T) {
	got := runTV(t,
		line("You take a drink from a vial."),
		line("The elixir heals and soothes you."),
		line("You see an orc here."),
	)
	assert.Equal(t, []string{"You see an orc here."}, got,
		"a paired curing/cured sequence should be suppressed entirely")
}

func TestTunnelVision_DropsUnpairedCuring(t *testing.T) {
	got := runTV(t,
		line("You take a drink from a vial."),
		line("You see an orc here."),
	)
	// The curing line is optimistically dropped; with no matching cured
	// line, it stays dropped (a small behavioural concession for the
	// stateless-across-batches refactor).
	assert.Equal(t, []string{"You see an orc here."}, got)
}

func TestTunnelVision_PassesUnknownThrough(t *testing.T) {
	got := runTV(t,
		line("You arrive at a small clearing."),
		line("A bird sings overhead."),
	)
	assert.Equal(t, []string{
		"You arrive at a small clearing.",
		"A bird sings overhead.",
	}, got)
}

func TestTunnelVision_ConsolidatesAttackSummary(t *testing.T) {
	got := runTV(t,
		line("You pump out at the orc with a powerful side kick."),
		line("You connect!"),
		line("You see an orc here."),
	)
	require.Len(t, got, 2)
	assert.Contains(t, got[0], "attack")
	assert.Contains(t, got[0], "the orc")
	assert.Contains(t, got[0], "Sidekick")
	assert.Contains(t, got[0], "hit")
	assert.Equal(t, "You see an orc here.", got[1])
}

func TestTunnelVision_ConsolidatesMultipleAttacks(t *testing.T) {
	got := runTV(t,
		line("You pump out at the orc with a powerful side kick."),
		line("You connect!"),
		line("You launch a powerful uppercut at the orc."),
		line("You miss."),
		line("You see an orc here."),
	)
	require.Len(t, got, 2)
	assert.Contains(t, got[0], "Sidekick")
	assert.Contains(t, got[0], "hit")
	assert.Contains(t, got[0], "Uppercut")
	assert.Contains(t, got[0], "miss")
	assert.Equal(t, "You see an orc here.", got[1])
}

func TestTunnelVision_FlushesAttackOnPrompt(t *testing.T) {
	t.Helper()

	w := achaea.New()
	chain := app.Chain(w.Processors()...)

	// Attack, then a Prompt — the prompt ends the response and must flush.
	events := []app.Event{
		line("You pump out at the orc with a powerful side kick."),
		line("You connect!"),
		connection.Prompt{Bytes: []byte("h:100 m:100 e:100 w:100 -")},
	}

	var summaries []string
	for _, ev := range events {
		got, err := chain(app.Batch{Event: ev})
		require.NoError(t, err)
		for _, c := range got.Commands {
			if pl, ok := c.(ui.PrintLine); ok {
				summaries = append(summaries, string(pl.Line.Formatted))
			}
		}
	}

	require.Len(t, summaries, 1)
	assert.Contains(t, summaries[0], "Sidekick")
	assert.Contains(t, summaries[0], "hit")
}

func TestTunnelVision_DoesNotFlushOnInterleavedGMCP(t *testing.T) {
	t.Helper()

	w := achaea.New()
	chain := app.Chain(w.Processors()...)

	// A GMCP frame between an attack and its modifier should not split the
	// run — they consolidate as a single summary on the trailing flush.
	events := []app.Event{
		line("You pump out at the orc with a powerful side kick."),
		connection.GMCPFrame{Payload: []byte("Char.Vitals {}")},
		line("You connect!"),
		line("You see an orc here."),
	}

	var out []string
	for _, ev := range events {
		got, err := chain(app.Batch{Event: ev})
		require.NoError(t, err)
		for _, c := range got.Commands {
			if pl, ok := c.(ui.PrintLine); ok {
				out = append(out, string(pl.Line.Formatted))
			}
		}
		if tl, ok := got.Event.(connection.TextLine); ok {
			out = append(out, string(tl.Bytes))
		}
	}

	require.Len(t, out, 2)
	assert.Contains(t, out[0], "Sidekick")
	assert.Contains(t, out[0], "hit")
	assert.Equal(t, "You see an orc here.", out[1])
}
