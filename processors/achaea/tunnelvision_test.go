package achaea_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/achaea"
)

// runTV feeds each TextLine through the world's Pre chain as its own batch
// and collects the TextLines that survive (i.e. were not dropped by setting
// batch.Event to nil). Attack consolidation is deliberately not exercised
// here; that behaviour was dropped during the per-event refactor and is
// tracked separately for a future redesign.
func runTV(t *testing.T, events ...connection.TextLine) []string {
	t.Helper()

	w := achaea.New()

	chain := app.Chain(w.Processors()...)

	var out []string
	for _, ev := range events {
		got, err := chain(app.Batch{Event: ev})
		require.NoError(t, err)
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
