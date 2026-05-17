package achaea

import (
	"math"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/lib/simpex"
)

var (
	// "ucp ucp" is intentional — the bash combo queues two uppercuts.
	bashAttack     = []byte("queue addclear eqbal combo sdk ucp ucp") //nolint:dupword
	bashClearEqbal = []byte("clearqueue eqbal")
)

var (
	bashKillPattern      = []byte("kill")
	bashSlainPattern     = []byte("You have slain *, retrieving the corpse.")
	bashAttackedPatterns = [][]byte{
		// @todo Reuse the TunnelVision attack patterns rather than
		// duplicating a subset here.
		[]byte("You pump out at * with a powerful side kick."),
		[]byte("You launch a powerful uppercut at *."),
		[]byte("A dizzying beam of energy strikes you as your attack rebounds off of *'s shield."),
	}
	bashGoldPattern = []byte("A ^ pile of sovereigns spills from the corpse.")
)

// Bashing enhances bashing-related tasks: when the user types "kill", it
// expands into a queued attack sequence and keeps attacking until the
// current target is slain and no further candidates remain in the room.
//
// On a slain message it clears the attack queue (so the user stops
// punching air after the last mob dies); when a corpse drops gold, it
// queues "get sovereigns" / "put sovereigns in pack" follow-ups.
type Bashing struct {
	world *world

	active    bool
	attacking bool
	killed    int
}

// NewBashing constructs a Bashing processor bound to the world's target
// tracker.
func NewBashing(w *world) *Bashing {
	return &Bashing{world: w}
}

// Processor returns the Bashing processor.
func (bsh *Bashing) Processor() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		// 1. Expand "kill" inputs into the bash queue, unless the
		// current target is another player.
		for i, cmd := range batch.Commands {
			send, ok := cmd.(connection.Send)
			if !ok {
				continue
			}
			if simpex.Match(bashKillPattern, send.Bytes) == nil {
				continue
			}
			if bsh.world != nil && bsh.world.Target != nil && bsh.world.Target.isPlayer {
				continue
			}
			batch.Commands[i] = connection.Send{Bytes: bashAttack}
			bsh.active = true
		}

		// 2. Walk TextLine events to drive the state machine.
		bsh.killed = math.MinInt
		for i, ev := range batch.Events {
			line, ok := ev.(connection.TextLine)
			if !ok {
				continue
			}

			switch {
			case simpex.Match(bashSlainPattern, line.Bytes) != nil:
				batch = bsh.onSlain(batch, i)

			case matchesAny(bashAttackedPatterns, line.Bytes):
				batch = bsh.onAttack(batch, i)

			case simpex.Match(bashGoldPattern, line.Bytes) != nil:
				batch = bsh.onGold(batch)
			}
		}

		bsh.attacking = false
		bsh.killed = 0
		return batch, nil
	}
}

func (bsh *Bashing) onSlain(batch app.Batch, atIndex int) app.Batch {
	if !bsh.active {
		return batch
	}
	if atIndex > bsh.killed {
		bsh.killed = atIndex
	}
	if bsh.world != nil && bsh.world.Target != nil && bsh.world.Target.Queue() > 0 {
		return batch
	}

	// No further candidates: stop attacking and clear the equilibrium queue.
	bsh.active = false
	bsh.attacking = false
	batch = dropMatching(batch, bashAttack)
	batch = batch.AppendCommand(connection.Send{Bytes: bashClearEqbal})
	return batch
}

func (bsh *Bashing) onAttack(batch app.Batch, atIndex int) app.Batch {
	// If a kill happened earlier in the same paragraph but a new attack
	// follows (the next target stepped up), keep attacking.
	if bsh.killed != math.MinInt && bsh.killed > 0 && !bsh.active {
		if atIndex > bsh.killed {
			bsh.active = true
			batch = dropMatching(batch, bashClearEqbal)
		}
	}

	if !bsh.active {
		return batch
	}

	if bsh.attacking {
		batch = dropMatching(batch, bashAttack)
	}

	batch = batch.AppendCommand(connection.Send{Bytes: bashAttack})
	bsh.attacking = true
	return batch
}

func (bsh *Bashing) onGold(batch app.Batch) app.Batch {
	if bsh.killed == math.MinInt {
		return batch
	}
	batch = batch.AppendCommand(connection.Send{Bytes: []byte("get sovereigns")})
	batch = batch.AppendCommand(connection.Send{Bytes: []byte("put sovereigns in pack")})
	return batch
}

// dropMatching removes any connection.Send commands whose bytes equal data.
func dropMatching(batch app.Batch, data []byte) app.Batch {
	out := batch.Commands[:0]
	for _, c := range batch.Commands {
		if s, ok := c.(connection.Send); ok && bytesEqual(s.Bytes, data) {
			continue
		}
		out = append(out, c)
	}
	batch.Commands = out
	return batch
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
