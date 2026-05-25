package achaea_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/achaea"
)

func TestBashing_KillExpandsToQueue(t *testing.T) {
	bsh := achaea.NewBashing(nil)
	p := bsh.Processor()

	got, err := p(app.Batch{Effects: []app.Effect{send("kill")}})
	require.NoError(t, err)
	require.Len(t, got.Effects, 1)
	assert.Equal(t,
		"queue addclear eqbal combo sdk ucp ucp",
		string(got.Effects[0].(connection.Send).Bytes),
	)
}

func TestBashing_AttackLineQueuesContinuation(t *testing.T) {
	bsh := achaea.NewBashing(nil)
	p := bsh.Processor()

	// First "kill" activates and queues the attack.
	_, err := p(app.Batch{Effects: []app.Effect{send("kill")}})
	require.NoError(t, err)

	// Server reports our side kick connecting → continue attacking on
	// the next equilibrium.
	got, err := p(app.Batch{
		Event: line("You pump out at the orc with a powerful side kick."),
	})
	require.NoError(t, err)
	require.Equal(t,
		[]string{"queue addclear eqbal combo sdk ucp ucp"},
		sendStrings(got),
		"an attack line should requeue the bash combo",
	)
}

func TestBashing_SlainStopsAttackingIfNoCandidates(t *testing.T) {
	bsh := achaea.NewBashing(nil)
	p := bsh.Processor()

	// Activate.
	_, err := p(app.Batch{Effects: []app.Effect{send("kill")}})
	require.NoError(t, err)

	// One of the previous attacks is still queued, then the target dies.
	got, err := p(app.Batch{
		Effects: []app.Effect{send("queue addclear eqbal combo sdk ucp ucp")},
		Event:   line("You have slain the orc, retrieving the corpse."),
	})
	require.NoError(t, err)

	cmds := sendStrings(got)
	assert.NotContains(t, cmds, "queue addclear eqbal combo sdk ucp ucp",
		"the queued attack should be dropped after slay",
	)
	assert.Contains(t, cmds, "clearqueue eqbal",
		"a clearqueue eqbal should be queued to release the unused equilibrium",
	)
}

func TestBashing_GoldTriggersLooting(t *testing.T) {
	bsh := achaea.NewBashing(nil)
	p := bsh.Processor()

	_, err := p(app.Batch{Effects: []app.Effect{send("kill")}})
	require.NoError(t, err)

	got, err := p(app.Batch{Event: line("You have slain the orc, retrieving the corpse.")})
	require.NoError(t, err)
	assert.NotContains(t, sendStrings(got), "get sovereigns",
		"the slain event alone shouldn't trigger looting")

	got, err = p(app.Batch{Event: line("A small pile of sovereigns spills from the corpse.")})
	require.NoError(t, err)

	cmds := sendStrings(got)
	assert.Contains(t, cmds, "get sovereigns")
	assert.Contains(t, cmds, "put sovereigns in pack")
}

func TestBashing_NoKillNoBashing(t *testing.T) {
	bsh := achaea.NewBashing(nil)
	p := bsh.Processor()

	got, err := p(app.Batch{
		Event: line("You pump out at the orc with a powerful side kick."),
	})
	require.NoError(t, err)
	assert.Empty(t, sendStrings(got),
		"attack lines should not queue anything until the user activates bashing",
	)
}
