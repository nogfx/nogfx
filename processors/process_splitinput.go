package processors

import (
	"bytes"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
)

// SplitInputProcessor splits each connection.Send command on the given
// separator, replacing it with one Send per part. Trailing/leading
// whitespace per part is trimmed.
//
// E.g. a Send{"one;two;three"} becomes Send{"one"}, Send{"two"}, Send{"three"}.
func SplitInputProcessor(sep []byte) Processor {
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
			parts := bytes.Split(send.Bytes, sep)
			if len(parts) == 1 {
				out = append(out, cmd)
				continue
			}
			for _, part := range parts {
				out = append(out, connection.Send{Bytes: bytes.TrimSpace(part)})
			}
		}
		batch.Commands = out
		return batch, nil
	}
}
