package gmcp

import (
	"github.com/icza/gox/gox"

	"github.com/nogfx/nogfx/internal/navigation"
)

// AsNavigation converts a Room.Info message into a navigation.Room, looking
// up or creating canonical Room and Area entries in the navigation cache.
// The bridge from the wire-data RoomInfo into the in-memory map graph lives
// here so that lib/navigation stays free of platform/gmcp dependencies.
func (msg *RoomInfo) AsNavigation() *navigation.Room {
	room, existed := navigation.LookupOrCreateRoom(msg.Number)
	if existed && room.Known {
		return room
	}

	room.Name = msg.Name
	room.X = gox.NewInt(msg.X)
	room.Y = gox.NewInt(msg.Y)
	room.Known = true

	if msg.Exits != nil {
		room.Exits = map[string]*navigation.Room{}
		for direction, number := range msg.Exits {
			adjacent, _ := navigation.LookupOrCreateRoom(number)
			room.Exits[direction] = adjacent
		}
	}

	room.Area = navigation.LookupOrCreateArea(msg.AreaNumber, msg.AreaName)

	return room
}
