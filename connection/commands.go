package connection

import "github.com/tobiassjosten/nogfx/app"

// Send writes bytes to the wire.
type Send struct {
	app.CommandMarker
	Bytes []byte
}

// Reconnect asks the connection to drop and re-establish the link.
type Reconnect struct {
	app.CommandMarker
}

// Disconnect closes the connection.
type Disconnect struct {
	app.CommandMarker
}
