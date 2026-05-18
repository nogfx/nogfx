package telnet_test

import (
	"bufio"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/nogfx/nogfx/platform/telnet"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify interface fulfilments.
var _ bufio.SplitFunc = (&telnet.NVT{}).SplitFunc

// enableSuppressGoAhead arranges the state a TelnetNegotiation processor
// would have reached after exchanging IAC WILL/DO SuppressGoAhead. SplitFunc
// consults this state to decide whether CRLF terminates a prompt. Without
// an in-process negotiation, tests that depend on SGA being enabled have
// to arrange it explicitly.
func enableSuppressGoAhead(t *testing.T, c *telnet.NVT) {
	t.Helper()
	_, err := c.Write([]byte{telnet.IAC, telnet.Do, telnet.SuppressGoAhead})
	require.NoError(t, err)
}

func TestSplitFunc(t *testing.T) {
	tcs := map[string]struct {
		output     []byte
		conn       net.Conn
		enableSGA  bool
		iterations int
		scanned    [][]byte
	}{
		"empty": {
			output:  []byte{},
			scanned: [][]byte{nil},
		},

		"cr nl no suppressed": {
			output: []byte("x\r\ny\r\n"),
			scanned: [][]byte{
				[]byte("x\r\ny\r\n"),
			},
		},

		"cr nl suppressed": {
			output: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'x', '\r', '\n', 'y', '\r', '\n',
			},
			enableSGA: true,
			scanned: [][]byte{
				[]byte("x\r\n"),
			},
		},

		"cr suppressed": {
			output: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'x', '\r', 'y', '\r',
			},
			enableSGA: true,
			scanned: [][]byte{
				[]byte("x\ry\r"),
			},
		},

		"nl suppressed": {
			output: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'x', '\n', 'y', '\n',
			},
			enableSGA: true,
			scanned: [][]byte{
				[]byte("x\ny\n"),
			},
		},

		"ga one": {
			output: []byte{
				'x', telnet.IAC, telnet.GA,
				'y', telnet.IAC, telnet.GA,
			},
			iterations: 1,
			scanned: [][]byte{
				{'x', telnet.GA},
			},
		},

		"ga two": {
			output: []byte{
				'x', telnet.IAC, telnet.GA,
				'y', telnet.IAC, telnet.GA,
			},
			iterations: 2,
			scanned: [][]byte{
				{'x', telnet.GA},
				{'y', telnet.GA},
			},
		},

		"ga suppressed": {
			output: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'x', telnet.IAC, telnet.GA,
				'y', telnet.IAC, telnet.GA,
			},
			enableSGA: true,
			scanned: [][]byte{
				{'x', telnet.GA},
			},
		},

		"on hold": {
			conn: &MockConn{
				Reader: MockReader(func(p []byte) (n int, err error) {
					return 0, nil
				}),
			},
			scanned: [][]byte{nil},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			conn := tc.conn
			if tc.conn == nil {
				conn = NewMockConn(tc.output)
			}
			client := telnet.NewNVT(conn)

			if tc.enableSGA {
				enableSuppressGoAhead(t, client)
			}

			scanner := bufio.NewScanner(client)
			scanner.Split(client.SplitFunc)

			iterations := 1
			if tc.iterations > 0 {
				iterations = tc.iterations
			}

			var scanned [][]byte
			for i := 0; i < iterations; i++ {
				scanner.Scan()
				scanned = append(scanned, scanner.Bytes())
			}

			assert.Equal(t, tc.scanned, scanned)
		})
	}
}

func TestRead(t *testing.T) {
	tcs := map[string]struct {
		serverWrite  []byte
		enableSGA    bool
		iterations   int
		bufferLength int
		serverRead   []byte
	}{
		"simple": {
			serverWrite: []byte("hello"),
			serverRead:  []byte("hello"),
		},

		"empty": {
			serverWrite: []byte{},
			serverRead:  []byte{},
		},

		"small buffer": {
			serverWrite:  []byte("hello"),
			bufferLength: 3,
			serverRead:   []byte("hel"),
		},

		"big buffer": {
			serverWrite:  []byte("hello"),
			bufferLength: 7,
			serverRead:   []byte("hello"),
		},

		"no split cr nl w/o suppress": {
			serverWrite: []byte("he\r\nllo"),
			serverRead:  []byte("he\r\nllo"),
		},

		"split cr nl w/ suppress one": {
			serverWrite: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'h', 'e', '\r', '\n', 'l', 'l', 'o',
			},
			enableSGA:  true,
			serverRead: []byte("he\r\n"),
		},

		"split cr nl w/ suppress two": {
			serverWrite: []byte{
				telnet.IAC, telnet.Will, telnet.SuppressGoAhead,
				'h', 'e', '\r', '\n', 'l', 'l', 'o',
			},
			enableSGA:  true,
			iterations: 2,
			serverRead: []byte("he\r\nllo"),
		},

		"no split cr-only": {
			serverWrite: []byte("he\rllo"),
			serverRead:  []byte("he\rllo"),
		},

		"no split nl-only": {
			serverWrite: []byte("he\nllo"),
			serverRead:  []byte("he\nllo"),
		},

		"split iac ga one": {
			serverWrite: []byte{'x', telnet.IAC, telnet.GA, 'y'},
			serverRead:  []byte{'x', telnet.GA},
		},

		"split iac ga two": {
			serverWrite: []byte{'x', telnet.IAC, telnet.GA, 'y'},
			iterations:  2,
			serverRead:  []byte{'x', telnet.GA, 'y'},
		},

		"swallow iac iac": {
			serverWrite: []byte{'x', telnet.IAC, telnet.IAC, 'y'},
			serverRead:  []byte("xy"),
		},

		"swallow iac do a": {
			serverWrite: []byte{'x', telnet.IAC, telnet.Do, 'a', 'y'},
			serverRead:  []byte("xy"),
		},

		"swallow incomplete": {
			serverWrite: []byte{'x', telnet.IAC, telnet.Do},
			serverRead:  []byte("x"),
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			client := telnet.NewNVT(NewMockConn(tc.serverWrite))

			if tc.enableSGA {
				enableSuppressGoAhead(t, client)
			}

			bufferLength := len(tc.serverWrite)
			if buffer := tc.bufferLength; buffer > 0 {
				bufferLength = buffer
			}

			iterations := 1
			if reads := tc.iterations; reads > 0 {
				iterations = reads
			}

			serverRead := []byte{}
			for i := 0; i < iterations; i++ {
				buffer := make([]byte, bufferLength)
				count, err := client.Read(buffer)
				require.True(t, err == nil || errors.Is(err, io.EOF), err)
				serverRead = append(serverRead, buffer[:count]...)
			}

			assert.Equal(t, tc.serverRead, serverRead)
		})
	}
}

// Note: a previous TestCommandFunc was removed when negotiation/command
// dispatch moved out of the NVT — IAC sequences now surface to the engine
// as TelnetCommand / GMCPFrame events (see TestRunSurfacesIACEvents in
// run_test.go) and policy belongs to processors.

func TestWrite(t *testing.T) {
	tcs := map[string]struct {
		clientWrite []byte
		clientRead  []byte
		verifier    func(*telnet.NVT) bool
	}{
		"simple": {
			clientWrite: []byte("hello"),
			clientRead:  []byte("hello\r\n"),
		},

		"empty": {
			clientWrite: []byte{},
			clientRead:  []byte("\r\n"),
		},

		"single ine break": {
			clientWrite: []byte{},
			clientRead:  []byte("\r\n"),
		},

		"double line break": {
			clientWrite: []byte("\r\n\r\n"),
			clientRead:  []byte("\r\n\r\n"),
		},

		"command": {
			clientWrite: []byte{telnet.IAC, telnet.Do, telnet.Echo},
			clientRead:  []byte{telnet.IAC, telnet.Do, telnet.Echo},
			// verifier: func(client *telnet.NVT) bool {
			// 	return client.
			// },
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			conn := NewMockConn([]byte{})
			client := telnet.NewNVT(conn)

			_, err := client.Write(tc.clientWrite)
			require.NoError(t, err)

			assert.Equal(t, tc.clientRead, conn.Written)
		})
	}
}
