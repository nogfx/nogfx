// Package headless implements a non-interactive UI endpoint: input is read
// from an io.Reader (typically stdin, one MUD command per line) and
// ui.PrintLine effects are written to an io.Writer (typically stdout). It
// exists so the assistant can drive a session for protocol and feature
// investigation without a tcell screen — see docs/agent/conduct.md for the
// operational rules that apply when it is in use.
//
// The headless endpoint deliberately does not render vitals, the target, or
// any other live-UI state; consumers that need that information should read
// it from the always-on event log (see processors/generic.EventLogProcessor)
// or by sending their own probe commands.
package headless

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/ui"
)

// Headless satisfies app.Endpoint. One instance corresponds to one session.
type Headless struct {
	in  io.Reader
	out io.Writer

	// writeMu serialises Fprintln calls so concurrent Apply invocations
	// don't interleave bytes on the output stream.
	writeMu sync.Mutex
}

// New constructs a Headless reading from os.Stdin and writing to os.Stdout.
// Use NewWithIO to override either side (tests, alternative transports).
func New() *Headless {
	return NewWithIO(os.Stdin, os.Stdout)
}

// NewWithIO constructs a Headless with caller-supplied I/O. Either argument
// may be nil — nil in means Run returns immediately (no input source); nil
// out means Apply discards PrintLine output.
func NewWithIO(in io.Reader, out io.Writer) *Headless {
	return &Headless{in: in, out: out}
}

// Run reads lines from the configured input and emits one ui.Input per line.
// EOF on input is the clean shutdown signal — Run returns nil. Context
// cancellation also returns nil; a scanner error returns the wrapped error.
func (h *Headless) Run(ctx context.Context, events chan<- app.Event) error {
	if h.in == nil {
		<-ctx.Done()

		return nil
	}

	// Lines are read on a worker goroutine so context cancellation isn't
	// blocked behind a Read. The worker closes lines on EOF and sends any
	// scanner error on errs.
	lines := make(chan []byte)
	errs := make(chan error, 1)

	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(h.in)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- b:
			case <-ctx.Done():
				return
			}
		}

		errs <- scanner.Err()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case b, ok := <-lines:
			if !ok {
				select {
				case err := <-errs:
					if err != nil {
						return fmt.Errorf("stdin scan: %w", err)
					}
				default:
				}

				return nil
			}

			select {
			case events <- ui.Input{Bytes: b}:
			case <-ctx.Done():
				return nil
			}
		}
	}
}

// Apply writes ui.PrintLine output to the configured writer and accepts the
// remaining UI effects as no-ops so the engine doesn't log them as
// unhandled. Effects targeting the connection endpoint return
// app.ErrEffectNotApplicable so engine routing can fall through.
// Headless emits no apply-consequence events, so the events slice is
// always nil.
func (h *Headless) Apply(eff app.Effect) ([]app.Event, error) {
	switch c := eff.(type) {
	case ui.PrintLine:
		if h.out == nil {
			return nil, nil
		}

		h.writeMu.Lock()
		defer h.writeMu.Unlock()

		_, err := fmt.Fprintln(h.out, string(c.Line.Formatted))

		return nil, err

	case ui.ReFormat,
		ui.SetHealth, ui.SetMana,
		ui.AddVital, ui.SetVital, ui.RemoveVital,
		ui.SetCharacter, ui.SetTarget, ui.SetRoom,
		ui.MaskInput, ui.UnmaskInput:
		// Accepted as no-ops: the headless endpoint doesn't render UI
		// chrome, but each of these is a UI effect — returning nil keeps
		// the engine from logging them as unhandled. MaskInput in
		// particular: stdin echo is the caller's concern (auto-login
		// reads credentials from a file, not stdin).
		return nil, nil

	default:
		return nil, app.ErrEffectNotApplicable
	}
}
