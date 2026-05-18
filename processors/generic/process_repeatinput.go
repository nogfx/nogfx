package generic

import (
	"bytes"
	"strconv"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// RepeatInputProcessor expands each connection.Send command whose bytes start
// with a number-prefix (e.g. "3 kick") into that command repeated the given
// number of times.
func RepeatInputProcessor() Processor {
	return func(batch app.Batch) (app.Batch, error) {
		if len(batch.Commands) == 0 {
			return batch, nil
		}

		out := make([]app.Command, 0, len(batch.Commands))
		for _, cmd := range batch.Commands {
			send, ok := cmd.(connection.Send)
			if !ok {
				out = append(out, cmd)
				continue
			}
			n, rest, ok := splitRepeatPrefix(send.Bytes)
			if !ok {
				out = append(out, cmd)
				continue
			}
			for range n {
				out = append(out, connection.Send{Bytes: rest})
			}
		}
		batch.Commands = out
		return batch, nil
	}
}

// splitRepeatPrefix parses "<count> <rest>" where count is an integer.
// Returns the count, the rest of the bytes, and ok=true if the prefix
// parsed. A count of 0 (or negative) is a valid parse and results in the
// command being dropped.
func splitRepeatPrefix(b []byte) (int, []byte, bool) {
	sp := bytes.IndexByte(b, ' ')
	if sp <= 0 {
		return 0, nil, false
	}
	n, err := strconv.Atoi(string(b[:sp]))
	if err != nil {
		return 0, nil, false
	}
	return n, b[sp+1:], true
}
