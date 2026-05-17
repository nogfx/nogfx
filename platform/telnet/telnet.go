package telnet

// Convenience constants to make telnet commands more readable.
const (
	Echo byte = 1

	// SuppressGoAhead disables GO AHEAD termination, for full duplex
	// capabilities.
	// https://datatracker.ietf.org/doc/html/rfc858
	SuppressGoAhead byte = 3

	TType byte = 24
	MCCP2 byte = 86
	ATCP  byte = 200
	GMCP  byte = 201

	SE byte = 240
	GA byte = 249
	SB byte = 250

	Will byte = 251
	Wont byte = 252
	Do   byte = 253
	Dont byte = 254

	IAC byte = 255
)

// Common IAC sequences expressed as byte slices for convenient byte.Equal
// comparisons in the engine.
var (
	IACWillEcho = []byte{IAC, Will, Echo}
	IACWontEcho = []byte{IAC, Wont, Echo}
	IACWillGMCP = []byte{IAC, Will, GMCP}
)
