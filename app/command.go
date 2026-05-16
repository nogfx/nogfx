package app

import "errors"

// Command identifies something to be done. Concrete command types live in
// the contract packages owned by each endpoint (connection/, ui/, …); app/
// only knows the abstract shape. The engine routes each command to the
// endpoint that owns its type by attempting dispatch through each endpoint
// in turn.
type Command interface {
	isCommand()
}

// ErrCommandNotApplicable is returned by an endpoint's Apply method when the
// command does not target that endpoint. The engine treats this as a signal
// to try the next candidate endpoint rather than as a real error.
var ErrCommandNotApplicable = errors.New("command not applicable to this endpoint")

// CommandMarker is embedded by concrete command types so that they satisfy
// Command. External packages cannot define their own implementation of
// Command without embedding this marker, which keeps the set of command
// types in the project inspectable.
type CommandMarker struct{}

func (CommandMarker) isCommand() {}
