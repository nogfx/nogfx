package generic

import (
	"bytes"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// Credential is one character's login pair. The composition root parses
// the per-host credentials file into an ordered slice of these and hands
// it to AutoLogin; future per-character selection (game-specific prompt
// detection) will pick from the same slice by name.
type Credential struct {
	Name     string
	Password string
}

// AutoLogin returns a processor that authenticates via a GMCP Char.Login
// frame. The processor watches for the server announcing GMCP support
// (IAC WILL GMCP) and, on first occurrence, appends a SendGMCP effect
// carrying the Char.Login JSON. The step is single-use: even if the
// server later re-announces GMCP, the credentials are not re-sent.
//
// Char.Login is the de-facto cross-MUD authentication message defined by
// the Mudlet GMCP spec; the Iron Realms games (Achaea, Aetolia, Lusternia,
// Imperian) all support it. Servers that don't speak GMCP, or that
// negotiate GMCP after some unrelated trigger, simply never see a
// matching event and AutoLogin stays dormant — the manual login flow then
// applies.
//
// The processor uses the first credential in the slice. Multi-character
// selection at runtime is a future feature that requires game-specific
// name/password prompt detection (and the GMCP path being disabled then,
// since it commits to one character before the user has chosen). With an
// empty slice, the returned processor is a pass-through.
func AutoLogin(creds []Credential) app.Processor {
	if len(creds) == 0 || creds[0].Name == "" || creds[0].Password == "" {
		return func(b app.Batch) (app.Batch, error) { return b, nil }
	}

	payload := []byte((&gmcp.CharLogin{
		Name:     creds[0].Name,
		Password: creds[0].Password,
	}).Marshal())
	consumed := false

	return func(batch app.Batch) (app.Batch, error) {
		if consumed {
			return batch, nil
		}

		ev, ok := batch.Event.(connection.TelnetCommand)
		if !ok || !bytes.Equal(ev.Bytes, connection.IACWillGMCP) {
			return batch, nil
		}

		consumed = true

		// AutoLogin only fires once per session, so we could equally
		// hand `payload` over directly, but a single send is single-use:
		// the read-only contract on connection.SendGMCP.Payload means
		// sharing the slice is fine either way.
		return batch.AppendEffect(connection.SendGMCP{Payload: payload}), nil
	}
}
