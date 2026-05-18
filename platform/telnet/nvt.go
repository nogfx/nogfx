package telnet

import (
	"bufio"
	"net"

	"github.com/nogfx/nogfx/app"
)

// Telnet is a symmetric protocol, with no distinct server and client. Options
// can be negotiated separately for each side and we need to keep track of who
// has enabled what. This type helps us more clearly differentiate the sides.
type optionSide bool

const (
	ourside   optionSide = true
	theirside            = false
)

type optionState int

const (
	// StateDisabled is the default state for any option, meaning its
	// activation hasn't yet been agreed upon.
	StateDisabled optionState = iota

	StateDisabling
	StateEnabled
	StateEnabling
)

func (s optionState) On() bool {
	return s == StateEnabled || s == StateEnabling
}

func (s optionState) Off() bool {
	return s == StateDisabled || s == StateDisabling
}

// NVT (Network Virtual Terminal) represents a bi-directional character device
// and is a fundamental concept in the Telnet protocol (RFC 854). It acts as
// both "server" and "client", with both ends of a connection being equal, and
// state requiring negotiation and unanimous agreement.
//
// Negotiation policy lives in a processor (see processors/generic.TelnetNegotiation);
// the NVT itself decodes wire bytes into TelnetCommand and GMCPFrame events
// for the chain and infers option state passively from the bytes flowing
// in both directions. SplitFunc reads that state to decide when SuppressGoAhead
// changes the prompt-termination rules.
type NVT struct {
	net.Conn

	buffer *bufio.Reader

	options map[optionSide]map[byte]optionState

	cmdBuffer []byte

	// events is the channel Run pushes onto. Read pushes directly to it
	// when an IAC sequence completes, so negotiation events surface
	// without waiting for the next text token (pure-IAC bursts would
	// otherwise hold Scan indefinitely). Nil outside of Run; surface
	// then falls back to pendingEvents for any standalone Read usage.
	events chan<- app.Event

	// pendingEvents holds events surfaced outside of Run (tests using
	// the NVT directly via the io.Reader interface). Run drains it at
	// startup so anything queued before the channel was wired up still
	// reaches the engine.
	pendingEvents []app.Event
}

// NewNVT creates a NVT with some sane defaults.
func NewNVT(conn net.Conn) *NVT {
	return &NVT{
		Conn: conn,

		buffer: bufio.NewReader(conn),

		options: map[optionSide]map[byte]optionState{
			ourside:   {},
			theirside: {},
		},
	}
}
