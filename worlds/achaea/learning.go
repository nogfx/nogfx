package achaea

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/tobiassjosten/nogfx/app"
	"github.com/tobiassjosten/nogfx/connection"
	"github.com/tobiassjosten/nogfx/lib/simpex"
)

// maxLessons is the cap on lessons that can be learnt in a single session,
// per the game's mechanics. Learning chains multiple sessions together to
// reach larger totals.
//
// @todo Make this 20 lessons when the myrrh/bisemutum defence ("Your mind
// is racing with enhanced speed.") is active.
var maxLessons = 15

// learnTimeout is how long Learning waits for lesson progress before
// assuming the session has died (interrupted, target left, etc.) and
// clearing its state.
var learnTimeout = 15 * time.Second

var (
	learnInputPattern   = []byte("learn {^} {^ from *}")
	lessonBeginPatterns = [][]byte{
		[]byte("* begins the lesson in ^."),
		[]byte("* bows to you and commences the lesson in ^."),
	}
	lessonContinuePattern = []byte("* continues your training in ^.")
	lessonFinishPatterns  = [][]byte{
		[]byte("* finishes the lesson in ^."),
		[]byte("Storing ^ remaining inks, * bows to you, the lesson in Tattoos complete."),
		[]byte("* bows to you - the lesson in ^ is over."),
	}
)

// Learning lets players learn an unlimited number of lessons in one
// command by automatically chaining learning sessions together. The user
// types "learn 35 X from Y"; Learning intercepts and sends "learn 15 X
// from Y" (the per-session cap), then on each "finishes the lesson"
// confirmation it issues the next chunk until the total is reached.
//
// Begin and continue lines are suppressed; finish lines are replaced with
// a progress summary that includes a remaining-time estimate.
type Learning struct {
	total     int
	remaining int
	target    []byte
	start     time.Time
	timer     *time.Timer
}

// Processor returns the Learning processor.
func (lrn *Learning) Processor() app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		// 1. Intercept "learn N X from Y" Send commands.
		for i, cmd := range batch.Commands {
			send, ok := cmd.(connection.Send)
			if !ok {
				continue
			}
			caps := simpex.Match(learnInputPattern, send.Bytes)
			if caps == nil {
				continue
			}
			n, err := strconv.Atoi(string(caps[0]))
			if err != nil || n <= maxLessons {
				continue
			}

			lrn.start = time.Now()
			lrn.total = n
			lrn.remaining = n
			lrn.target = caps[1]
			lrn.armTimer()

			batch.Commands[i] = connection.Send{Bytes: lrn.nextChunk()}
		}

		// 2. If a session isn't active, leave server events alone.
		if lrn.timer == nil {
			return batch, nil
		}

		// 3. Walk TextLine events, suppress/replace as appropriate.
		out := batch.Events[:0]
		for _, ev := range batch.Events {
			line, ok := ev.(connection.TextLine)
			if !ok {
				out = append(out, ev)
				continue
			}

			switch {
			case matchesAny(lessonBeginPatterns, line.Bytes):
				// Show the progress line on the first begin of a
				// session so the user sees what's happening.
				if lrn.total-lrn.remaining == maxLessons {
					out = append(out, connection.TextLine{Bytes: lrn.progressLine()})
				}
				lrn.armTimer()

			case simpex.Match(lessonContinuePattern, line.Bytes) != nil:
				// Drop the noisy "continues your training" line.
				lrn.armTimer()

			case matchesAny(lessonFinishPatterns, line.Bytes):
				if lrn.remaining <= 0 {
					out = append(out, connection.TextLine{Bytes: lrn.completionLine()})
					lrn.reset()
					continue
				}
				batch = batch.AppendCommand(connection.Send{Bytes: lrn.nextChunk()})
				out = append(out, connection.TextLine{Bytes: lrn.progressLine()})
				lrn.start = time.Now()
				lrn.armTimer()

			default:
				out = append(out, ev)
			}
		}
		batch.Events = out
		return batch, nil
	}
}

// nextChunk returns the bytes of the next "learn N X" command to send and
// decrements the remaining counter accordingly.
func (lrn *Learning) nextChunk() []byte {
	count := maxLessons
	if lrn.remaining < count {
		count = lrn.remaining
	}
	lrn.remaining -= count
	return []byte(fmt.Sprintf("learn %d %s", count, lrn.target))
}

// progressLine renders the "X of Y lessons learned, T remaining" status.
func (lrn *Learning) progressLine() []byte {
	done := lrn.total - lrn.remaining

	timeleft := ""
	duration := time.Since(lrn.start)
	remaining := math.Ceil(float64(lrn.remaining) / float64(maxLessons))
	estimate := duration * time.Duration(remaining)

	if mins := estimate.Minutes(); mins >= 1 {
		timeleft += fmt.Sprintf("%.0f minutes ", mins)
		estimate -= time.Duration(mins) * time.Minute
	}
	timeleft += fmt.Sprintf("%.0f seconds", estimate.Seconds())

	return []byte(fmt.Sprintf("%d of %d lessons learned, %s remaining.",
		done, lrn.total, timeleft))
}

// completionLine renders the final "X of X lessons learned" message.
func (lrn *Learning) completionLine() []byte {
	return []byte(fmt.Sprintf("%d of %d lessons learned.", lrn.total, lrn.total))
}

func (lrn *Learning) armTimer() {
	if lrn.timer != nil {
		lrn.timer.Stop()
	}
	lrn.timer = time.AfterFunc(learnTimeout, lrn.reset)
}

func (lrn *Learning) reset() {
	if lrn.timer != nil {
		lrn.timer.Stop()
	}
	lrn.total = 0
	lrn.remaining = 0
	lrn.target = nil
	lrn.start = time.Time{}
	lrn.timer = nil
}

func matchesAny(patterns [][]byte, text []byte) bool {
	for _, p := range patterns {
		if simpex.Match(p, text) != nil {
			return true
		}
	}
	return false
}
