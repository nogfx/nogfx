package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// NegotiationOption configures one option code's policy. AcceptTheirs is
// true when the client accepts the server's WILL on that option (replies
// DO); AcceptOurs is true when the client accepts the server's DO on that
// option (replies WILL). The defaults — both false — decline by replying
// DONT or WONT respectively.
type NegotiationOption struct {
	AcceptTheirs bool
	AcceptOurs   bool
}

// NegotiationPolicy maps an option code to the client's policy on that
// option. Options not in the map are declined.
type NegotiationPolicy map[byte]NegotiationOption

// DefaultNegotiation is the standard policy for nogfx: accept GMCP and
// SuppressGoAhead from the server (their side), accept Echo if the server
// asks us to take over echoing (our side). Anything else is declined.
//
// This matches the prior inline negotiation defaults — callers building a
// chain composed of the standard generic processors will see the same
// wire behaviour they had before negotiation moved out of the NVT.
func DefaultNegotiation() NegotiationPolicy {
	return NegotiationPolicy{
		connection.SuppressGoAhead: {AcceptTheirs: true},
		connection.GMCP:            {AcceptTheirs: true},
		connection.Echo:            {AcceptOurs: true},
	}
}

// TelnetNegotiation returns a processor that replies to telnet negotiation
// requests according to the given policy. It is stateful within a session
// — once an option has been agreed in a direction, subsequent matching
// requests from the server are silently ignored (avoiding the redundant
// ack-loops RFC 1143 was designed against).
//
// The processor only inspects 3-byte WILL/WONT/DO/DONT TelnetCommand
// events. Subnegotiation frames (notably GMCP) flow through it unchanged
// because they surface as their own typed events (e.g. GMCPFrame).
func TelnetNegotiation(policy NegotiationPolicy) app.Processor {
	their := map[byte]bool{}
	our := map[byte]bool{}

	return func(batch app.Batch) (app.Batch, error) {
		tc, ok := batch.Event.(connection.TelnetCommand)
		if !ok || len(tc.Bytes) != 3 || tc.Bytes[0] != connection.IAC {
			return batch, nil
		}

		verb := tc.Bytes[1]
		opt := tc.Bytes[2]
		cfg := policy[opt]

		var reply []byte

		switch verb {
		case connection.Will:
			if their[opt] {
				return batch, nil
			}

			if cfg.AcceptTheirs {
				their[opt] = true
				reply = []byte{connection.IAC, connection.Do, opt}
			} else {
				reply = []byte{connection.IAC, connection.Dont, opt}
			}
		case connection.Do:
			if our[opt] {
				return batch, nil
			}

			if cfg.AcceptOurs {
				our[opt] = true
				reply = []byte{connection.IAC, connection.Will, opt}
			} else {
				reply = []byte{connection.IAC, connection.Wont, opt}
			}
		case connection.Wont:
			if !their[opt] {
				return batch, nil
			}

			their[opt] = false
			reply = []byte{connection.IAC, connection.Dont, opt}
		case connection.Dont:
			if !our[opt] {
				return batch, nil
			}

			our[opt] = false
			reply = []byte{connection.IAC, connection.Wont, opt}
		default:
			return batch, nil
		}

		return batch.AppendEffect(connection.Send{Bytes: reply}), nil
	}
}
