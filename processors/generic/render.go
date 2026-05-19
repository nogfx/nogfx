package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/ui"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// Render translates a DecodedGMCP trigger into the corresponding UI
// commands. It is the minimum translation that applies to any GMCP-capable
// MUD; world-specific processors layer on top with richer behaviour (e.g.
// Achaea's vitals, custom auxiliary vitals, target tracking).
//
// What Render handles today:
//
//   - gmcp.CharName  → ui.SetCharacter
//   - gmcp.RoomInfo  → ui.SetRoom (constructed via msg.AsNavigation)
//
// The generic GMCP vocabulary does not specify a vitals message, so
// health and mana commands are emitted by the world layer (Achaea uses
// its richer agmcp.CharVitals).
func Render() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		d, ok := batch.Event.(DecodedGMCP)
		if !ok {
			return batch, nil
		}

		switch msg := d.Message.(type) {
		case *gmcp.CharName:
			return batch.AppendCommand(ui.SetCharacter{
				Name:  msg.Name,
				Title: msg.Fullname,
			}), nil

		case *gmcp.RoomInfo:
			room := msg.AsNavigation()
			if room != nil {
				room.HasPlayer = true
			}

			return batch.AppendCommand(ui.SetRoom{Room: room}), nil
		}

		return batch, nil
	}
}
