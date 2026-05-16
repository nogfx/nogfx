package achaea

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/pkg"
	"github.com/tobiassjosten/nogfx/platform/gmcp"
	agmcp "github.com/tobiassjosten/nogfx/platform/gmcp/achaea"
	igmcp "github.com/tobiassjosten/nogfx/platform/gmcp/ironrealms"
	"github.com/tobiassjosten/nogfx/lib/navigation"
	"github.com/tobiassjosten/nogfx/processors"
	"github.com/tobiassjosten/nogfx/platform/telnet"
	"github.com/tobiassjosten/nogfx/ui"
)

type world struct {
	Character *Character
	Room      *navigation.Room
	Target    *Target
}

// World holds the Achaea-specific state and exposes Pre/Post slices of
// processors. User scripts are inserted between Pre and Post by the engine
// wiring (currently `app.Chain(append(append(w.Pre(), scripts...),
// w.Post()...)...)`); when no scripts are loaded the Pre+Post chain is
// equivalent to the previous single Processor.
type World struct {
	state *world
	tv    TunnelVision

	rawLog app.Processor
	out    app.Processor
}

// New constructs an Achaea World, opening per-session raw and processed log
// files. New takes no UI or Connection references — everything the world
// does flows through the batch's events and commands.
func New() (*World, error) {
	state := &world{
		Character: &Character{},
		Target:    NewTarget(),
	}

	now := time.Now().Format("20060102-150405")

	rawLog, err := processors.LogProcessor(
		filepath.Join(pkg.Directory, "logs"),
		fmt.Sprintf("achaea.com-%s.raw.log", now),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create log processor: %w", err)
	}

	out, err := processors.LogProcessor(
		filepath.Join(pkg.Directory, "logs"),
		fmt.Sprintf("achaea.com-%s.log", now),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create log processor: %w", err)
	}

	return &World{
		state:  state,
		tv:     TunnelVision{character: state.Character},
		rawLog: rawLog,
		out:    out,
	}, nil
}

// Pre returns the processors that run before user scripts. These do raw
// logging, input translation, output rewriting, game state mutation, and
// feature processors that should see clean events.
func (w *World) Pre() []app.Processor {
	// @todo Read the CommandSeparator configuration and use that instead.
	sep := []byte{';'}

	return []app.Processor{
		w.rawLog,
		processors.Input(),
		processors.SplitInputProcessor(sep),
		processors.RepeatInputProcessor(),
		RewriteOutput,

		w.state.cmdprocess,

		(&Learning{}).Processor(),
		NewBashing(w.state).Processor(),
		w.tv.rewriteProcessor(),
	}
}

// Post returns the processors that run after user scripts. Server text
// becomes UI PrintLine commands here (after any script that might want to
// drop or rewrite individual lines); then the processed log captures the
// final batch.
func (w *World) Post() []app.Processor {
	return []app.Processor{
		processors.Output(),
		w.out,
	}
}

// Chain composes the Pre and Post slices around the given user-script
// processors. With no scripts, the result is just Pre + Post.
func (w *World) Chain(scripts ...app.Processor) app.Processor {
	return app.Chain(append(append(w.Pre(), scripts...), w.Post()...)...)
}

// Processor is a back-compatible shim that returns the full Pre+Post chain
// with no scripts inserted. Callers that want script support should call
// New and Chain directly.
func Processor() (processors.Processor, error) {
	w, err := New()
	if err != nil {
		return nil, err
	}
	return w.Chain(), nil
}

// RewriteOutput drops leading blank lines that the server sends to compensate
// for echoed player input. With output control we don't need them.
func RewriteOutput(batch app.Batch) (app.Batch, error) {
	if len(batch.Events) < 3 {
		return batch, nil
	}
	first, ok := batch.Events[0].(connection.TextLine)
	if !ok || len(stripANSI(first.Bytes)) != 0 {
		return batch, nil
	}
	// Carry any ANSI prefix forward onto the next line.
	if len(first.Bytes) > 0 {
		if second, ok := batch.Events[1].(connection.TextLine); ok {
			batch.Events[1] = connection.TextLine{
				Bytes: append(append([]byte{}, first.Bytes...), second.Bytes...),
			}
		}
	}
	batch.Events = batch.Events[1:]
	return batch, nil
}

func (world *world) cmdprocess(batch app.Batch) (app.Batch, error) {
	for _, ev := range batch.Events {
		switch ev := ev.(type) {
		case connection.TelnetCommand:
			if bytes.Equal(ev.Bytes, telnet.IAC_WILL_GMCP) {
				batch = batch.AppendCommand(connection.Send{
					Bytes: []byte((&gmcp.CoreSupportsSet{
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
			batch = batch.AppendCommand(connection.Send{Bytes: m})
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
		world.Room = navigation.RoomFromGMCP(msg)
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
