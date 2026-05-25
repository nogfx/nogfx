package app

import "errors"

// Effect identifies something for an endpoint to do — write bytes to
// the wire, update the UI, etc. Concrete effect types live in the
// contract packages owned by each endpoint (connection/, ui/, …); app/
// only knows the abstract shape. The engine routes each effect to the
// endpoint that owns its type by attempting dispatch through each
// endpoint in turn.
//
// "Effect" is deliberately distinct from "Command" in this codebase:
// Command is reserved for the MUD-domain meaning (the plain-text input
// a player would type and send to the server). An Effect can carry a
// Command on the wire (connection.Send.Bytes), but Effect itself is
// the engine-level dispatch abstraction.
//
// Convention: variables and parameters holding an Effect are named
// `eff` (not `cmd`), and Endpoint implementations name the Apply
// receiver `eff app.Effect`. This keeps `cmd` available for code
// that legitimately handles a MUD command (a string).
type Effect interface {
	isEffect()
}

// ErrEffectNotApplicable is returned by an endpoint's Apply method
// when the effect does not target that endpoint. The engine treats
// this as a signal to try the next candidate endpoint rather than as
// a real error.
var ErrEffectNotApplicable = errors.New("effect not applicable to this endpoint")

// EffectMarker is embedded by concrete effect types so that they
// satisfy Effect. External packages cannot define their own
// implementation of Effect without embedding this marker, which keeps
// the set of effect types in the project inspectable.
type EffectMarker struct{}

func (EffectMarker) isEffect() {}
