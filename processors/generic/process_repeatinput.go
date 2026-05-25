package generic

import (
	"bytes"
	"strconv"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// RepeatInputProcessor expands each connection.Send effect whose bytes start
// with a number-prefix (e.g. "3 kick") into that effect repeated the given
// number of times.
func RepeatInputProcessor() Processor {
	return func(batch app.Batch) (app.Batch, error) {
		if len(batch.Effects) == 0 {
			return batch, nil
		}

		out := make([]app.Effect, 0, len(batch.Effects))
		for _, eff := range batch.Effects {
			send, ok := eff.(connection.Send)
			if !ok {
				out = append(out, eff)

				continue
			}

			n, rest, ok := splitRepeatPrefix(send.Bytes)
			if !ok {
				out = append(out, eff)

				continue
			}

			for range n {
				out = append(out, connection.Send{Bytes: rest})
			}
		}

		batch.Effects = out

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
