package achaea_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/worlds/achaea"
)

// runTV exposes the TunnelVision rewrite processor for testing. We construct
// the world via achaea.New so character state is initialised, then exercise
// the processor in isolation.
func runTV(t *testing.T, events ...app.Event) []string {
	t.Helper()

	w, err := achaea.New(t.TempDir())
	require.NoError(t, err)

	// The rewrite processor is exposed indirectly through Pre(); the
	// rewrite is the second-to-last processor in the chain. Rather than
	// reaching into private fields, we run the full Pre + Post chain and
	// observe the resulting TextLine events (no GMCP / Send activity
	// means everything else is a passthrough for these inputs).
	chain := app.Chain(w.Pre()...)
	got, err := chain(app.Batch{Events: events})
	require.NoError(t, err)

	var out []string
	for _, ev := range got.Events {
		if tl, ok := ev.(connection.TextLine); ok {
			out = append(out, string(tl.Bytes))
		}
	}
	return out
}

func stripStyleStr(s string) string {
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			return s
		}
		j := strings.Index(s[i:], "m")
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
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

func TestTunnelVision_LeavesUnpairedCuringAlone(t *testing.T) {
	got := runTV(t,
		line("You take a drink from a vial."),
		line("You see an orc here."),
	)
	assert.Equal(t, []string{
		"You take a drink from a vial.",
		"You see an orc here.",
	}, got, "without a matching 'cured' follow-up, the curing line passes through")
}

func TestTunnelVision_ConsolidatesAttackSummary(t *testing.T) {
	got := runTV(t,
		line("You pump out at the orc with a powerful side kick."),
		line("You connect!"),
	)

	require.Len(t, got, 1)
	assert.Contains(t, stripStyleStr(got[0]), "attack the orc",
		"the summary should name the target")
	assert.Contains(t, stripStyleStr(got[0]), "Sidekick",
		"the summary should name the attack")
	assert.Contains(t, stripStyleStr(got[0]), "hit",
		"the summary should include the hit modifier")
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

func TestTunnelVision_FlushesOnNonAttackLine(t *testing.T) {
	got := runTV(t,
		line("You pump out at the orc with a powerful side kick."),
		line("You connect!"),
		line("The sun rises over the horizon."),
	)
	require.Len(t, got, 2)
	assert.Contains(t, stripStyleStr(got[0]), "Sidekick")
	assert.Equal(t, "The sun rises over the horizon.", got[1])
}
