package achaea

import (
	"strconv"
	"strings"

	"github.com/tobiassjosten/nogfx/pkg"
	"github.com/tobiassjosten/nogfx/platform/gmcp"
	agmcp "github.com/tobiassjosten/nogfx/platform/gmcp/achaea"
	igmcp "github.com/tobiassjosten/nogfx/platform/gmcp/ironrealms"
	"github.com/tobiassjosten/nogfx/lib/navigation"
	"github.com/tobiassjosten/nogfx/ui"
)

// Target represents who or what is being targeted for skills and attacks.
type Target struct {
	*pkg.Target
	isPlayer bool
	area     *navigation.Area

	// pendingSends accumulates byte sequences that should be emitted as
	// connection.Send commands the next time the world's processor drains
	// the target. This is the transitional replacement for the old
	// client.Send call from inside Set; once auto-retarget is reworked to
	// flow through the batch directly the field can go away.
	pendingSends [][]byte
}

// NewTarget creates a new target object.
func NewTarget() *Target {
	target := &Target{}
	target.Target = pkg.NewTarget(target.Set)
	return target
}

// PkgTarget converts our game-specific Target to the general pkg struct.
func (tgt *Target) PkgTarget() *pkg.Target {
	return tgt.Target
}

// SetTargetCommand returns a ui.SetTarget command reflecting the current
// target, suitable for appending to a batch.
func (tgt *Target) SetTargetCommand() ui.SetTarget {
	if tgt.Target == nil || tgt.Name == "" {
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

// Set records a desired target change. The actual settarget command is
// queued via DrainSends; the world's processor flushes it onto the batch.
func (tgt *Target) Set(name string, _ *pkg.Target) {
	if tgt.isPlayer {
		return
	}

	if name == "" {
		tgt.pendingSends = append(tgt.pendingSends, []byte("settarget none"))
		return
	}

	tgt.pendingSends = append(tgt.pendingSends, []byte("settarget "+name))
}

// FromRoomInfo handles targeting when moving between rooms (areas, in effect).
func (tgt *Target) FromRoomInfo(msg *gmcp.RoomInfo) {
	room := navigation.RoomFromGMCP(msg)
	if room == nil || room.Area == nil {
		return
	}

	if tgt.area != nil && room.Area.ID == tgt.area.ID {
		return
	}
	tgt.area = room.Area

	npcs := tgt.npcs()[room.Area.ID]
	tgt.Target.SetCandidates(npcs)
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
	tgt.Target.SetPresent(present)
}

// FromCharItemsAdd adds an NPC to the room list and retargets.
func (tgt *Target) FromCharItemsAdd(msg *gmcp.CharItemsAdd) {
	if msg.Location != "room" || !msg.Item.Attributes.Monster {
		return
	}

	tgt.Target.AddPresent(msg.Item.Name)
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
	// room and thus remain an eligible  target. Fix this.
	if msg.Item.Attributes.Dead {
		name = strings.TrimPrefix(name, "the corpse of ")
	}

	tgt.Target.RemovePresent(name)
}

// FromCharStatus updates the current target.
func (tgt *Target) FromCharStatus(msg *agmcp.CharStatus) {
	if msg.Target != nil {
		tgt.Name = strings.ToLower(*msg.Target)
	}
}

// FromIRETargetSet updates the player status of the current target.
func (tgt *Target) FromIRETargetSet(msg *igmcp.IRETargetSet) {
	// This message works so inconsistenyly that we can only rely
	// on it for knowing that non-numbers equals a player.
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
