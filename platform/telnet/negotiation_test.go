package telnet_test

import (
	"testing"

	"github.com/nogfx/nogfx/platform/telnet"

	"github.com/stretchr/testify/assert"
)

// Negotiation behavior is no longer in the NVT — see
// processors/generic/negotiate_test.go for the policy tests. This file
// retains TestIsCommand because IsCommand still gates Read's buffer
// recognition of complete IAC sequences.

func TestIsCommand(t *testing.T) {
	tcs := map[string]struct {
		data    []byte
		verdict bool
	}{
		"empty": {
			data:    []byte{},
			verdict: false,
		},

		"a": {
			data:    []byte("a"),
			verdict: false,
		},

		"as": {
			data:    []byte("as"),
			verdict: false,
		},

		"asd": {
			data:    []byte("asd"),
			verdict: false,
		},

		"asdf": {
			data:    []byte("asdf"),
			verdict: false,
		},

		"iac iac": {
			data:    []byte{telnet.IAC, telnet.IAC},
			verdict: true,
		},

		"iac ga": {
			data:    []byte{telnet.IAC, telnet.GA},
			verdict: true,
		},

		"iac will echo": {
			data:    []byte{telnet.IAC, telnet.Will, telnet.Echo},
			verdict: true,
		},

		"iac will a": {
			data:    []byte{telnet.IAC, telnet.Will, 'a'},
			verdict: true,
		},

		"iac will echo a": {
			data:    []byte{telnet.IAC, telnet.Will, telnet.Echo, 'a'},
			verdict: false,
		},

		"iac wont echo": {
			data:    []byte{telnet.IAC, telnet.Wont, telnet.Echo},
			verdict: true,
		},

		"iac wont a": {
			data:    []byte{telnet.IAC, telnet.Wont, 'a'},
			verdict: true,
		},

		"iac wont echo a": {
			data:    []byte{telnet.IAC, telnet.Wont, telnet.Echo, 'a'},
			verdict: false,
		},

		"iac do echo": {
			data:    []byte{telnet.IAC, telnet.Do, telnet.Echo},
			verdict: true,
		},

		"iac do a": {
			data:    []byte{telnet.IAC, telnet.Do, 'a'},
			verdict: true,
		},

		"iac do echo a": {
			data:    []byte{telnet.IAC, telnet.Do, telnet.Echo, 'a'},
			verdict: false,
		},

		"iac dont echo": {
			data:    []byte{telnet.IAC, telnet.Dont, telnet.Echo},
			verdict: true,
		},

		"iac dont a": {
			data:    []byte{telnet.IAC, telnet.Dont, 'a'},
			verdict: true,
		},

		"iac dont echo a": {
			data:    []byte{telnet.IAC, telnet.Dont, telnet.Echo, 'a'},
			verdict: false,
		},

		"iac 239 echo": {
			data:    []byte{telnet.IAC, 239, telnet.Echo},
			verdict: false,
		},

		"iac 239 a": {
			data:    []byte{telnet.IAC, 239, 'a'},
			verdict: false,
		},

		"sub-negotiation empty": {
			data:    []byte{telnet.IAC, telnet.SB, telnet.IAC, telnet.SE},
			verdict: true,
		},

		"sub-negotiation complete": {
			data:    []byte{telnet.IAC, telnet.SB, 'a', telnet.IAC, telnet.SE},
			verdict: true,
		},

		"sub-negotiation unterminated": {
			data:    []byte{telnet.IAC, telnet.SB, 'a', telnet.IAC},
			verdict: false,
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.verdict, telnet.IsCommand(tc.data))
		})
	}
}
