package processors

import (
	"fmt"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/lib/simpex"
)

// Match is the result of a pattern successfully applied to a single event
// or command in a batch. Index is the position of the matched item in the
// slice that was searched; Captures are the simpex captures.
type Match struct {
	Index    int
	Captures [][]byte
}

// Callback is invoked when one or more patterns match. It receives the list
// of matches and the current batch, and returns the modified batch.
type Callback func(matches []Match, batch app.Batch) app.Batch

// MatchInput matches the pattern against the bytes of every connection.Send
// command currently in the batch. On match(es), the callback is invoked.
func MatchInput(pat string, cb Callback) Processor {
	return MatchInputs([]string{pat}, cb)
}

// MatchInputs matches any of the patterns against connection.Send commands.
func MatchInputs(pats []string, cb Callback) Processor {
	pbs := patternBytes(pats)
	return func(batch app.Batch) (b app.Batch, err error) {
		defer recoverCallback(&err)

		var matches []Match
		for i, cmd := range batch.Commands {
			send, ok := cmd.(connection.Send)
			if !ok {
				continue
			}
			if caps := firstMatch(pbs, send.Bytes); caps != nil {
				matches = append(matches, Match{Index: i, Captures: caps})
			}
		}
		if len(matches) > 0 {
			batch = cb(matches, batch)
		}
		return batch, nil
	}
}

// MatchOutput matches the pattern against the bytes of every
// connection.TextLine event currently in the batch.
func MatchOutput(pat string, cb Callback) Processor {
	return MatchOutputs([]string{pat}, cb)
}

// MatchOutputs matches any of the patterns against connection.TextLine events.
func MatchOutputs(pats []string, cb Callback) Processor {
	pbs := patternBytes(pats)
	return func(batch app.Batch) (b app.Batch, err error) {
		defer recoverCallback(&err)

		var matches []Match
		for i, ev := range batch.Events {
			line, ok := ev.(connection.TextLine)
			if !ok {
				continue
			}
			if caps := firstMatch(pbs, line.Bytes); caps != nil {
				matches = append(matches, Match{Index: i, Captures: caps})
			}
		}
		if len(matches) > 0 {
			batch = cb(matches, batch)
		}
		return batch, nil
	}
}

func patternBytes(pats []string) [][]byte {
	out := make([][]byte, len(pats))
	for i, p := range pats {
		out[i] = []byte(p)
	}
	return out
}

func firstMatch(pats [][]byte, text []byte) [][]byte {
	for _, pat := range pats {
		if caps := simpex.Match(pat, text); caps != nil {
			return caps
		}
	}
	return nil
}

func recoverCallback(err *error) {
	if r := recover(); r != nil {
		if rerr, ok := r.(error); ok {
			*err = fmt.Errorf("match callback failed: %w", rerr)
			return
		}
		*err = fmt.Errorf("match callback failed: %v", r)
	}
}
