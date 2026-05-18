package telnet

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
)

// Run reads from the underlying transport, tokenises bytes into typed
// connection events, and pushes them onto events until ctx is cancelled or
// the transport closes. Run satisfies connection.Connection.
func (nvt *NVT) Run(ctx context.Context, events chan<- app.Event) error {
	// Wire the channel so Read.surface() can push IAC events directly.
	// Direct push avoids the deadlock that would occur if events queued
	// behind a Scan that's blocked waiting for text after a pure-IAC
	// burst.
	nvt.events = events
	defer func() { nvt.events = nil }()

	// Drain anything Read queued before Run started.
	for _, ev := range nvt.pendingEvents {
		events <- ev
	}
	nvt.pendingEvents = nil

	scanner := bufio.NewScanner(nvt)
	scanner.Split(nvt.SplitFunc)

	var pending [][]byte
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}

		tok := append([]byte{}, scanner.Bytes()...)
		tok = bytes.TrimRight(tok, "\r\n")

		// Lines without a trailing GA are accumulated; a paragraph
		// ends when a token terminates with GA. The final line of the
		// paragraph is the prompt; everything prior is regular output.
		if len(tok) == 0 || tok[len(tok)-1] != GA {
			if len(tok) > 0 {
				pending = append(pending, tok)
			}
			continue
		}

		pending = append(pending, tok[:len(tok)-1])
		lines := pending
		pending = nil

		for i, line := range lines {
			if i == len(lines)-1 {
				events <- connection.Prompt{Bytes: line}
			} else {
				events <- connection.TextLine{Bytes: line}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if errors.Is(err, io.EOF) {
			events <- connection.StateChanged{Connected: false}
			return nil
		}
		events <- connection.StateChanged{Connected: false, Err: err}
		return err
	}

	events <- connection.StateChanged{Connected: false}
	return nil
}

// Apply applies a single command to the connection. Commands not targeting
// this endpoint return app.ErrCommandNotApplicable.
func (nvt *NVT) Apply(cmd app.Command) error {
	switch c := cmd.(type) {
	case connection.Send:
		_, err := nvt.Write(c.Bytes)
		return err

	case connection.SendGMCP:
		frame := make([]byte, 0, len(c.Payload)+5)
		frame = append(frame, IAC, SB, GMCP)
		frame = append(frame, c.Payload...)
		frame = append(frame, IAC, SE)
		_, err := nvt.Write(frame)
		return err

	case connection.Reconnect, connection.Disconnect:
		// Not yet implemented. Returning nil avoids the engine treating
		// these as routing failures; the actual transport-control wiring
		// arrives in a follow-up.
		_ = c
		return nil
	}
	return app.ErrCommandNotApplicable
}
