package telnet

import (
	"bytes"
)

// IsCommand determines whether the given sequence is a valid Telnet command.
// Used by Read to know when cmdBuffer has accumulated a full sequence and
// can be acted on.
func IsCommand(cmd []byte) bool {
	l := len(cmd)

	if l < 2 {
		return false
	}

	if bytes.Equal(cmd, []byte{IAC, IAC}) {
		return true
	}

	if bytes.Equal(cmd, []byte{IAC, GA}) {
		return true
	}

	if l == 3 && (cmd[1] == Will || cmd[1] == Wont || cmd[1] == Do || cmd[1] == Dont) {
		return true
	}

	if bytes.Equal(cmd[:2], []byte{IAC, SB}) && bytes.Equal(cmd[l-2:], []byte{IAC, SE}) {
		return true
	}

	return false
}
