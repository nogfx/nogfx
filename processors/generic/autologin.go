package generic

import (
	"bytes"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/platform/gmcp"
)

// CredentialsUser and CredentialsPass are the credentials-map keys
// AutoLogin reads. Worlds that share GMCP-based authentication conventions
// (the Iron Realms family, Mudlet-spec servers) can use AutoLogin
// directly; worlds with their own auth wire-format will need their own
// processor.
const (
	CredentialsUser = "user"
	CredentialsPass = "pass"
)

// AutoLogin returns a processor that authenticates via a GMCP Char.Login
// frame. The processor watches for the server announcing GMCP support
// (IAC WILL GMCP) and, on first occurrence, appends a SendGMCP command
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
// With missing user, missing pass, or no credentials at all, the returned
// processor is a pass-through.
func AutoLogin(creds map[string]string) app.Processor {
	user := creds[CredentialsUser]

	pass := creds[CredentialsPass]
	if user == "" || pass == "" {
		return func(b app.Batch) (app.Batch, error) { return b, nil }
	}

	payload := []byte((&gmcp.CharLogin{Name: user, Password: pass}).Marshal())
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

		reply := make([]byte, len(payload))
		copy(reply, payload)

		return batch.AppendCommand(connection.SendGMCP{Payload: reply}), nil
	}
}
