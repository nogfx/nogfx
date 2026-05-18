package connection

// IAC byte sequences that processors may match against TelnetCommand events.
// The connection package hosts these because the wire-protocol vocabulary
// is part of the connection contract — anyone reacting to a "WILL X" needs
// the byte pattern but shouldn't depend on the telnet implementation.
//
// The full meaning of the bytes is RFC 854: IAC=255, WILL=251, WONT=252,
// DO=253, DONT=254. Option codes follow (Echo=1, GMCP=201).
var (
	IACWillEcho = []byte{255, 251, 1}
	IACWontEcho = []byte{255, 252, 1}
	IACWillGMCP = []byte{255, 251, 201}
)
