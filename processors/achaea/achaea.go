package achaea

import (
	"bytes"
	"errors"
	"log"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/internal/navigation"
	"github.com/nogfx/nogfx/platform/gmcp"
	agmcp "github.com/nogfx/nogfx/platform/gmcp/achaea"
	igmcp "github.com/nogfx/nogfx/platform/gmcp/ironrealms"
)

type world struct {
	Character *Character
	Room      *navigation.Room
	Target    *Target
}

// World holds the Achaea-specific state and exposes a Processors slice the
// composition root (cmd/nogfx/main.go) splices into the engine chain. The
// world is unaware of logging, user scripts, and any other infrastructure
// around it — only its own state and feature generic.
type World struct {
	state *world
	tv    TunnelVision
}

// New constructs an Achaea World. New takes no UI, Connection, or logging
// references — everything the world does flows through the batch's events
// and commands.
func New() *World {
	state := &world{
		Character: &Character{},
		Target:    NewTarget(),
	}

	return &World{
		state: state,
		tv:    TunnelVision{character: state.Character},
	}
}

// newRewriteOutput returns a stateful processor that drops blank TextLine
// events (the server sends them to compensate for echoed player input) and
// carries any leading ANSI escape bytes forward onto the next TextLine, so
// colour state is preserved.
func newRewriteOutput() app.Processor {
	var pendingANSI []byte
	return func(batch app.Batch) (app.Batch, error) {
		line, ok := batch.Event.(connection.TextLine)
		if !ok {
			return batch, nil
		}
		if len(stripANSI(line.Bytes)) == 0 {
			if len(line.Bytes) > 0 {
				pendingANSI = append(pendingANSI[:0], line.Bytes...)
			}
			batch.Event = nil
			return batch, nil
		}
		if len(pendingANSI) > 0 {
			batch.Event = connection.TextLine{
				Bytes: append(append([]byte{}, pendingANSI...), line.Bytes...),
			}
			pendingANSI = pendingANSI[:0]
		}
		return batch, nil
	}
}

// Processors returns the Achaea-specific processors in their intended
// chain order: output rewrite (drop server-echo blanks), GMCP / telnet
// dispatch and state mutation, feature processors (Learning, Bashing,
// TunnelVision).
//
// The composition root is responsible for ordering these around generic
// infrastructure (logging, input translation, output rendering) and user
// scripts.
func (w *World) Processors() []app.Processor {
	return []app.Processor{
		newRewriteOutput(),

		w.state.cmdprocess,

		(&Learning{}).Processor(),
		NewBashing(w.state).Processor(),
		w.tv.rewriteProcessor(),
	}
}

func (world *world) cmdprocess(batch app.Batch) (app.Batch, error) {
	switch ev := batch.Event.(type) {
	case connection.TelnetCommand:
		if bytes.Equal(ev.Bytes, connection.IACWillGMCP) {
			batch = batch.AppendCommand(connection.SendGMCP{
				Payload: []byte((&gmcp.CoreSupportsSet{
					"Char":         1,
					"Char.Items":   1,
					"Char.Skills":  1,
					"Comm.Channel": 1,
					"Room":         1,
					"IRE.Rift":     1,
					"IRE.Target":   1,
				}).Marshal()),
			})
		}

	case connection.GMCPFrame:
		batch = world.dispatchGMCP(batch, ev.Payload)
	}
	return batch, nil
}

func (world *world) dispatchGMCP(batch app.Batch, data []byte) app.Batch {
	message, err := agmcp.Parse(data)
	if err != nil {
		if !errors.Is(err, gmcp.ErrUnknownMessage) {
			log.Printf("failed parsing GMCP: %s", err)
		}
		return batch
	}

	switch msg := message.(type) {
	case *gmcp.CharItemsList:
		world.Target.FromCharItemsList(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

	case *gmcp.CharItemsAdd:
		world.Target.FromCharItemsAdd(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

	case *gmcp.CharItemsRemove:
		world.Target.FromCharItemsRemove(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

	case *gmcp.CharName:
		world.Character.FromCharName(msg)
		for _, m := range [][]byte{
			[]byte((&gmcp.CharItemsInv{}).Marshal()),
			[]byte((&gmcp.CommChannelPlayers{}).Marshal()),
			[]byte((&igmcp.IRERiftRequest{}).Marshal()),
		} {
			batch = batch.AppendCommand(connection.SendGMCP{Payload: m})
		}
		batch = batch.AppendCommand(ui.SetCharacter{
			Name:  world.Character.Name,
			Title: world.Character.Title,
		})

	case *agmcp.CharStatus:
		world.Character.FromCharStatus(msg)
		world.Target.FromCharStatus(msg)
		batch = batch.AppendCommand(ui.SetCharacter{
			Name:  world.Character.Name,
			Title: world.Character.Title,
		})
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

	case *agmcp.CharVitals:
		world.Character.FromCharVitals(msg)
		batch = appendVitalsCommands(batch, world.Character)

	case *gmcp.RoomInfo:
		world.Target.FromRoomInfo(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

		if world.Room != nil {
			world.Room.HasPlayer = false
		}
		world.Room = msg.AsNavigation()
		world.Room.HasPlayer = true
		batch = batch.AppendCommand(ui.SetRoom{Room: world.Room})

	case *igmcp.IRETargetSet:
		world.Target.FromIRETargetSet(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())

	case *igmcp.IRETargetInfo:
		world.Target.FromIRETargetInfo(msg)
		batch = batch.AppendCommand(world.Target.SetTargetCommand())
	}

	// Drain any Send commands the Target accumulated during this dispatch
	// (e.g. settarget on auto-retarget).
	for _, b := range world.Target.DrainSends() {
		batch = batch.AppendCommand(connection.Send{Bytes: b})
	}
	return batch
}

func appendVitalsCommands(batch app.Batch, c *Character) app.Batch {
	batch = batch.AppendCommand(ui.SetHealth{Value: c.Health, Max: c.MaxHealth})
	batch = batch.AppendCommand(ui.SetMana{Value: c.Mana, Max: c.MaxMana})
	batch = batch.AppendCommand(ui.AddVital{Name: "endurance", Value: c.Endurance, Max: c.MaxEndurance})
	batch = batch.AppendCommand(ui.AddVital{Name: "willpower", Value: c.Willpower, Max: c.MaxWillpower})
	if c.Ferocity > 0 {
		batch = batch.AppendCommand(ui.AddVital{Name: "ferocity", Value: c.Ferocity, Max: 100})
	}
	if c.Kai > 0 {
		batch = batch.AppendCommand(ui.AddVital{Name: "kai", Value: c.Kai, Max: 100})
	}
	if c.Karma > 0 {
		batch = batch.AppendCommand(ui.AddVital{Name: "karma", Value: c.Karma, Max: 100})
	}
	return batch
}

// @todo Move this somewhere else. It's too general to belong with Achaea
// specifics but we currently don't have a good other place to put it.
func stripANSI(text []byte) (clean []byte) {
	var sequence bool
	for _, c := range text {
		if c == 0x1b {
			sequence = true
			continue
		}

		if sequence {
			if c == 'm' {
				sequence = false
			}
			continue
		}

		clean = append(clean, c)
	}

	return clean
}
