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
