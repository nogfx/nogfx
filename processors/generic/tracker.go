package generic

import (
	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// Resolved is the derived event Tracker.Resolve appends when an
// incoming event resolves a queued MUD command. Command is the
// original command text the player (or the server, via a forcing
// effect) issued; Reply is the event that resolved it.
//
// Note the deliberate naming: "Command" here is the MUD-domain
// meaning — the line of text sent to the server — not app.Effect
// (the engine-dispatch abstraction). Consumers that need timing
// information (LagWatcher) keep their own send-time state; the
// Tracker queue itself is timeless.
type Resolved struct {
	app.EventMarker
	Command string
	Reply   app.Event
}

// Timeout is the derived event Tracker.Resolve appends for each
// queued command that preceded the resolved one. Reason carries the
// event that triggered the resolution.
type Timeout struct {
	app.EventMarker
	Command string
	Reason  app.Event
}

// Tracker is the central queue of in-flight MUD commands — text we've
// sent to the server and are still expecting a visible effect for.
// Other processors take a *Tracker dependency at construction and
// consult or mutate it via methods. Methods are safe only on the
// engine's chain goroutine; the queue carries no internal
// synchronisation, matching the rest of the processor chain.
//
// The queue holds plain strings, not app.Effect values. GMCP and
// other protocol-level traffic is correlated elsewhere (see
// LagWatcher for the heartbeat path); the Tracker is specifically
// about player-issued (or forced) command lines and the use cases
// they enable: illusion detection, swallowed-command detection,
// reorder detection.
//
// Empty strings never enter the queue — see Record's doc.
type Tracker struct {
	pending []string
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker {
	return &Tracker{}
}

// Record appends a command to the queue. Called by the Recorder
// processor for every connection.Sent event whose Effect is a
// connection.Send (a player-issued text command), and directly by
// world-specific processors that recognise server-forced actions in
// the output stream.
//
// Empty input is rejected: a blank player Enter (Recorder skips it
// before calling here) and an explicit Record("") from a
// world-specific processor are both no-ops. If a future world
// scenario needs to represent "something was forced but we couldn't
// tell what", that's the cue to add a typed sentinel — not to
// overload the empty string.
func (t *Tracker) Record(command string) {
	if command == "" {
		return
	}

	t.pending = append(t.pending, command)
}

// List returns a copy of the queue, oldest-first. Callers may
// inspect freely without affecting Tracker state. The main use is
// answering "do we have anything pending?" / "how many?" / "what's
// in flight right now?" without mutating the queue.
func (t *Tracker) List() []string {
	out := make([]string, len(t.pending))
	copy(out, t.pending)

	return out
}

// Find returns the first queued command the predicate accepts
// without mutating state. Used by pure queries — illusion detection
// ("is there anything in flight that would explain this
// affliction?"), scramble detection ("did we send the command whose
// effect we see out of order?"), and similar.
func (t *Tracker) Find(pred func(string) bool) (string, bool) {
	for _, cmd := range t.pending {
		if pred(cmd) {
			return cmd, true
		}
	}

	return "", false
}

// Resolve walks the queue oldest-first and removes the first command
// the predicate accepts. It returns a Resolved derived event for
// that command plus one Timeout event for every command that
// preceded it. The caller appends the returned events to its batch
// — Tracker stays a data structure and does not emit into the chain
// itself, keeping event flow visible at the call site.
//
// If no command matches, the queue is left untouched and the
// returned slice is nil.
func (t *Tracker) Resolve(pred func(string) bool, reply app.Event) []app.Event {
	matchedIdx := -1

	for i, cmd := range t.pending {
		if pred(cmd) {
			matchedIdx = i

			break
		}
	}

	if matchedIdx < 0 {
		return nil
	}

	events := make([]app.Event, 0, matchedIdx+1)

	for _, skipped := range t.pending[:matchedIdx] {
		events = append(events, Timeout{Command: skipped, Reason: reply})
	}

	events = append(events, Resolved{Command: t.pending[matchedIdx], Reply: reply})

	t.pending = append([]string(nil), t.pending[matchedIdx+1:]...)

	return events
}

// Recorder returns a processor that records every MUD command we
// send. It watches connection.Sent events emitted by the Connection
// endpoint and, when the Effect is a connection.Send (player-issued
// text), calls tracker.Record with the byte string. SendGMCP effects
// are not MUD commands — they're protocol traffic — and are
// ignored; protocol-level correlation lives in LagWatcher.
//
// Empty Send.Bytes is also ignored. A player hitting Enter on a
// blank input line is a routine interaction (refresh the prompt,
// page through output); recording it would clutter the queue with
// non-attributable entries that confuse future resolvers.
func Recorder(tracker *Tracker) app.Processor {
	return func(batch app.Batch) (app.Batch, error) {
		sent, ok := batch.Event.(connection.Sent)
		if !ok {
			return batch, nil
		}

		send, ok := sent.Effect.(connection.Send)
		if !ok {
			return batch, nil
		}

		if len(send.Bytes) == 0 {
			return batch, nil
		}

		tracker.Record(string(send.Bytes))

		return batch, nil
	}
}
