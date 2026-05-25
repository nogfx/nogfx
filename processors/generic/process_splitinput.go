package generic

import (
	"bytes"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// SplitInputProcessor splits each connection.Send effect on the given
// separator, replacing it with one Send per part. Trailing/leading
// whitespace per part is trimmed.
//
// E.g. a Send{"one;two;three"} becomes Send{"one"}, Send{"two"}, Send{"three"}.
func SplitInputProcessor(sep []byte) Processor {
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

			parts := bytes.Split(send.Bytes, sep)
			if len(parts) == 1 {
				out = append(out, eff)

				continue
			}

			for _, part := range parts {
				out = append(out, connection.Send{Bytes: bytes.TrimSpace(part)})
			}
		}

		batch.Effects = out

		return batch, nil
	}
}
