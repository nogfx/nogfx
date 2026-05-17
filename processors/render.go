package processors

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/lib/navigation"
	"github.com/nogfx/nogfx/platform/gmcp"
	"github.com/nogfx/nogfx/ui"
)

// Render translates baseline typed GMCP message events (those emitted by
// Decode) into the corresponding UI commands. It is the minimum
// translation that applies to any GMCP-capable MUD; world-specific
// processors layer on top with richer behaviour (e.g. Achaea's vitals,
// custom auxiliary vitals, target tracking).
//
// What Render handles today:
//
//   - gmcp.CharName  → ui.SetCharacter
//   - gmcp.RoomInfo  → ui.SetRoom (constructed via navigation.RoomFromGMCP)
//
// The generic GMCP vocabulary does not specify a vitals message, so health
// and mana commands are emitted by the world layer (Achaea uses its richer
// agmcp.CharVitals).
func Render() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		for _, ev := range batch.Events {
			d, ok := ev.(DecodedGMCP)
			if !ok {
				continue
			}
			switch msg := d.Message.(type) {
			case *gmcp.CharName:
				batch = batch.AppendCommand(ui.SetCharacter{
					Name:  msg.Name,
					Title: msg.Fullname,
				})

			case *gmcp.RoomInfo:
				room := navigation.RoomFromGMCP(msg)
				if room != nil {
					room.HasPlayer = true
				}
				batch = batch.AppendCommand(ui.SetRoom{Room: room})
			}
		}
		return batch, nil
	}
}
