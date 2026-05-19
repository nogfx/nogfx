package generic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/generic"
)

func TestRepeatInputProcessor(t *testing.T) {
	tcs := map[string]struct {
		input  [][]byte
		output [][]byte
	}{
		"empty": {
			input:  nil,
			output: nil,
		},

		"repeat": {
			input:  [][]byte{[]byte("2 asdf")},
			output: [][]byte{[]byte("asdf"), []byte("asdf")},
		},

		"non-numeric": {
			input:  [][]byte{[]byte("x asdf")},
			output: [][]byte{[]byte("x asdf")},
		},

		"zero": {
			input:  [][]byte{[]byte("0 asdf")},
			output: nil,
		},

		"straggler": {
			input: [][]byte{
				[]byte("2 asdf"),
				[]byte("qwer"),
			},
			output: [][]byte{
				[]byte("asdf"),
				[]byte("asdf"),
				[]byte("qwer"),
			},
		},

		"mixed": {
			input: [][]byte{
				[]byte("qwer"),
				[]byte("3 asdf"),
				[]byte("x zxcv"),
				[]byte("2 fdsa"),
				[]byte("rewq"),
				[]byte("0 vcxz"),
			},
			output: [][]byte{
				[]byte("qwer"),
				[]byte("asdf"),
				[]byte("asdf"),
				[]byte("asdf"),
				[]byte("x zxcv"),
				[]byte("fdsa"),
				[]byte("fdsa"),
				[]byte("rewq"),
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			proc := generic.RepeatInputProcessor()

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
