package achaea

import (
	"bytes"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/internal/navigation"
	"github.com/nogfx/nogfx/platform/gmcp"
	agmcp "github.com/nogfx/nogfx/platform/gmcp/achaea"
	igmcp "github.com/nogfx/nogfx/platform/gmcp/ironrealms"
)

// Target represents who or what is being targeted for skills and attacks.
// It owns both the displayable state (Name, Health) and the auto-retarget
// machinery (candidates / present lists), so a change in the room can
// pick a new target without external coordination.
type Target struct {
	Name   string
	Health int

	// candidates lists the names of potential auto-target NPCs in
	// descending priority (most dangerous first). Updated when the
	// player enters a new area.
	candidates []string

	// present lists the NPC entries currently in the room. The overlap
	// between this and candidates is what drives auto-target selection.
	present []string

	isPlayer bool
	area     *navigation.Area

	// pendingSends accumulates "settarget …" byte sequences produced by
	// auto-retargeting. They are drained by the world's processor into
	// connection.Send commands so the engine routes them to the wire.
	pendingSends [][]byte
}

// NewTarget creates a new target with Health initialised to -1 (unknown).
func NewTarget() *Target {
	return &Target{Health: -1}
}

// SetTargetCommand returns a ui.SetTarget command reflecting the current
// target, suitable for appending to a batch. A nil Target clears the UI's
// current target.
func (tgt *Target) SetTargetCommand() ui.SetTarget {
	if tgt.Name == "" {
		return ui.SetTarget{Target: nil}
	}

	queue := tgt.Queue() - 1
	if queue < 0 {
		queue = 0
	}

	return ui.SetTarget{Target: &ui.Target{
		Name:   tgt.Name,
		Health: tgt.Health,
		Queue:  queue,
	}}
}

// DrainSends returns and clears the accumulated send-byte sequences. The
// caller wraps each in a connection.Send command and appends to the batch.
func (tgt *Target) DrainSends() [][]byte {
	sends := tgt.pendingSends
	tgt.pendingSends = nil

	return sends
}

// queueSet queues a "settarget X" command (or "settarget none" for an
// empty name) so the engine can write it to the wire on the next pass.
// Duplicate consecutive sends for the same target are coalesced — a real
// concern, since Name itself is only updated when the server confirms via
// IRETargetSet, so without dedupe two retarget calls in the same batch
// would each queue the same command.
//
// Player targets are left alone — they are set manually.
func (tgt *Target) queueSet(name string) {
	if name == tgt.Name {
		return
	}

	if tgt.isPlayer {
		return
	}

	var cmd []byte
	if name == "" {
		cmd = []byte("settarget none")
	} else {
		cmd = []byte("settarget " + name)
	}

	if n := len(tgt.pendingSends); n > 0 && bytes.Equal(tgt.pendingSends[n-1], cmd) {
		return
	}

	tgt.pendingSends = append(tgt.pendingSends, cmd)
}

// SetCandidates replaces the list of auto-targetable NPCs. If the current
// target is no longer a candidate, it is cleared; otherwise the auto-target
// logic re-runs against the new list.
func (tgt *Target) SetCandidates(names []string) {
	oldCandidates := tgt.candidates
	tgt.candidates = names

	wasCandidate := slices.Contains(oldCandidates, tgt.Name)
	stillCandidate := slices.Contains(tgt.candidates, tgt.Name)

	if wasCandidate && !stillCandidate {
		tgt.queueSet("")

		return
	}

	tgt.retarget()
}

// SetPresent replaces the list of entities in the room. Auto-targeting
// re-runs after the update.
func (tgt *Target) SetPresent(names []string) {
	tgt.present = names
	tgt.retarget()
}

// AddPresent records that an entity has entered the room. Auto-targeting
// re-runs after the update.
func (tgt *Target) AddPresent(name string) {
	tgt.present = append(tgt.present, name)
	tgt.retarget()
}

// RemovePresent records that an entity has left the room. Auto-targeting
// re-runs after the update.
func (tgt *Target) RemovePresent(name string) {
	if i := slices.Index(tgt.present, name); i >= 0 {
		tgt.present = append(tgt.present[:i], tgt.present[i+1:]...)
		tgt.retarget()
	}
}

// Queue counts how many valid targets are present in the room, including
// the current one.
func (tgt *Target) Queue() int {
	queue := 0

	for _, present := range tgt.present {
		if tgt.Name != "" && strings.Contains(present, tgt.Name) {
			queue++

			continue
		}

		for _, candidate := range tgt.candidates {
			if strings.Contains(present, candidate) {
				queue++

				break
			}
		}
	}

	return queue
}

// retarget picks the most-prioritised candidate present in the room and
// switches to it if it differs from the current target.
func (tgt *Target) retarget() {
	if tgt.Name != "" && !slices.Contains(tgt.candidates, tgt.Name) {
		return
	}

	var newTarget string

outer:
	for _, candidate := range tgt.candidates {
		for _, present := range tgt.present {
			if strings.Contains(present, candidate) {
				newTarget = candidate

				break outer
			}
		}
	}

	if newTarget != "" && newTarget != tgt.Name {
		tgt.queueSet(newTarget)
		// Name itself is not updated here — the server confirms the
		// change through IRETargetSet and FromIRETargetSet writes
		// the new name into state. That keeps the displayed target
		// consistent with what the server believes.
	}
}

// FromRoomInfo handles targeting when moving between rooms (areas, in effect).
func (tgt *Target) FromRoomInfo(msg *gmcp.RoomInfo) {
	room := msg.AsNavigation()
	if room == nil || room.Area == nil {
		return
	}

	if tgt.area != nil && room.Area.ID == tgt.area.ID {
		return
	}

	tgt.area = room.Area

	npcs := tgt.npcs()[room.Area.ID]
	tgt.SetCandidates(npcs)
}

// FromCharItemsList builds the list of NPCs in the room and retargets.
func (tgt *Target) FromCharItemsList(msg *gmcp.CharItemsList) {
	if msg.Location != "room" {
		return
	}

	present := []string{}
	for _, item := range msg.Items {
		present = append(present, item.Name)
	}

	tgt.SetPresent(present)
}

// FromCharItemsAdd adds an NPC to the room list and retargets.
func (tgt *Target) FromCharItemsAdd(msg *gmcp.CharItemsAdd) {
	if msg.Location != "room" || !msg.Item.Attributes.Monster {
		return
	}

	tgt.AddPresent(msg.Item.Name)
}

// FromCharItemsRemove removes an NPC to the room list and retargets.
func (tgt *Target) FromCharItemsRemove(msg *gmcp.CharItemsRemove) {
	if msg.Location != "room" || !msg.Item.Attributes.Monster {
		return
	}

	name := msg.Item.Name

	// When a NPC dies its name goes from "x" to "the corpse of x" without
	// triggering a Char.Items.Update, so we handle that here.
	// @todo When we don't kill and autograb the corpse, it won't leave the
	// room and thus remain an eligible target. Fix this.
	if msg.Item.Attributes.Dead {
		name = strings.TrimPrefix(name, "the corpse of ")
	}

	tgt.RemovePresent(name)
}

// FromCharStatus updates the current target from a Char.Status GMCP message.
func (tgt *Target) FromCharStatus(msg *agmcp.CharStatus) {
	if msg.Target != nil {
		tgt.Name = strings.ToLower(*msg.Target)
	}
}

// FromIRETargetSet updates the player status of the current target.
func (tgt *Target) FromIRETargetSet(msg *igmcp.IRETargetSet) {
	// This message works so inconsistently that we can only rely on it
	// for knowing that non-numbers equals a player.
	if msg.Target != "" {
		_, err := strconv.Atoi(msg.Target)
		tgt.isPlayer = err != nil
	}

	if msg.Target == "" || tgt.isPlayer {
		tgt.Health = -1
	}
}

// FromIRETargetInfo updates the current NPC-target's health.
func (tgt *Target) FromIRETargetInfo(msg *igmcp.IRETargetInfo) {
	tgt.Health = msg.Health
}

func (tgt *Target) npcs() map[int][]string {
	// An important property of these lists is their order of importance,
	// where the most dangerous NPC is first and the rest in falling order.
	return map[int][]string{
		// The Keep of Belladona.
		134: {
			// Aggressive:
			"grothgar", "crocodile", "guardian", "hound",
			"minotaur",
			// Passive:
			"courtier", "imp", "leech", "toad",
		},

		// The Village of Genji.
		137: {"atavian", "manticore"},
	}
}
