package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestSplitInputProcessor(t *testing.T) {
	tcs := map[string]struct {
		sep    []byte
		input  [][]byte
		output [][]byte
	}{
		"empty": {
			input:  nil,
			output: nil,
		},

		"split semicolon": {
			sep:    []byte{';'},
			input:  [][]byte{[]byte("asdf;qwer")},
			output: [][]byte{[]byte("asdf"), []byte("qwer")},
		},

		"split pipes": {
			sep:    []byte("||"),
			input:  [][]byte{[]byte("asdf||qwer")},
			output: [][]byte{[]byte("asdf"), []byte("qwer")},
		},

		"no separator passes through": {
			sep:    []byte{';'},
			input:  [][]byte{[]byte("asdf")},
			output: [][]byte{[]byte("asdf")},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			proc := generic.SplitInputProcessor(tc.sep)

			var batch app.Batch
			for _, b := range tc.input {
				batch = batch.AppendCommand(connection.Send{Bytes: b})
			}

			out, err := proc(batch)
			require.NoError(t, err)

			var got [][]byte

			for _, cmd := range out.Commands {
				send, ok := cmd.(connection.Send)
				require.True(t, ok, "command should be a connection.Send, got %T", cmd)

				got = append(got, send.Bytes)
			}

			assert.Equal(t, tc.output, got)
		})
	}
}
