package connection

// Telnet wire-protocol constants. The connection package hosts these
// because the byte vocabulary is part of the contract between the
// telnet adapter and any processor reacting to a TelnetCommand event
// (negotiation policy, GMCP framing, MCCP2, …). Processors should not
// depend on the platform/telnet package.
//
// Values come from RFC 854 (IAC framing), RFC 855 (option-code framework)
// and per-option RFCs: Echo (RFC 857, opt 1), SGA (RFC 858, opt 3),
// GMCP (Mudlet spec, opt 201).
const (
	IAC  byte = 255
	SE   byte = 240
	GA   byte = 249
	SB   byte = 250
	Will byte = 251
	Wont byte = 252
	Do   byte = 253
	Dont byte = 254

	Echo            byte = 1
	SuppressGoAhead byte = 3
	GMCP            byte = 201
)

// Pre-built sequences for processors that want a byte comparison rather
// than a manual three-byte assembly.
var (
	IACWillEcho = []byte{IAC, Will, Echo}
	IACWontEcho = []byte{IAC, Wont, Echo}
	IACWillGMCP = []byte{IAC, Will, GMCP}
)
