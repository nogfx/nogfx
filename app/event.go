package app

// Event identifies something that has happened. Concrete event types live in
// the contract packages owned by each endpoint (connection/, ui/, …); app/
// only knows the abstract shape. The interface is intentionally minimal —
// downstream code matches on concrete types via type assertion or type switch.
type Event interface {
	isEvent()
}

// EventMarker is embedded by concrete event types so that they satisfy Event.
// External packages cannot define their own implementation of Event without
// embedding this marker, which keeps the set of event types in the project
// inspectable.
type EventMarker struct{}

func (EventMarker) isEvent() {}

// GuardedEvent is an opt-in interface for events that forbid certain
// effects from appearing in their batch. The engine checks `batch.Event`
// for this interface after the processor chain runs and drops (with a log
// line) any effect the event forbids. The intent is to break re-entrant
// loops at the contract level — e.g. a ReFormatting event forbids a
// further ReFormat effect, since emitting one would replay the same
// scrollback and re-enter the same code path.
//
// Most events implement nothing extra. GuardedEvent is the exception, not
// the norm.
type GuardedEvent interface {
	Event
	Forbids(Effect) bool
}
