package generic

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// EventLogProcessor writes one timestamped line per batch trigger event to
// the given file, regardless of event type. It is intended as a debugging
// instrument — not part of the regular chain — and the format is chosen
// for easy eyeballing and awk:
//
//	15:04:05.000 TXT You see an orc here.
//	15:04:05.012 GMC Char.Vitals
//	15:04:05.013 PRM h:100 m:100 e:100 w:100 -
//	15:04:05.140 GMC Char.Items.List
//
// The four three-letter tags let you scan for sequences like "GMC not
// followed by PRM within N ms" with awk one-liners.
func EventLogProcessor(dir, filename string) (Processor, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create event log directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, filename)

	// #nosec G304 -- dir/filename are caller-controlled (composition root).
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create event log %q: %w", path, err)
	}

	return func(batch app.Batch) (app.Batch, error) {
		ts := time.Now().Format("15:04:05.000")

		var line string
		switch e := batch.Event.(type) {
		case connection.TextLine:
			line = fmt.Sprintf("%s TXT %s\n", ts, sanitize(e.Bytes))
		case connection.Prompt:
			line = fmt.Sprintf("%s PRM %s\n", ts, sanitize(e.Bytes))
		case connection.GMCPFrame:
			line = fmt.Sprintf("%s GMC %s\n", ts, gmcpHead(e.Payload))
		case connection.TelnetCommand:
			line = fmt.Sprintf("%s IAC %x\n", ts, e.Bytes)
		case connection.StateChanged:
			line = fmt.Sprintf("%s STA connected=%v err=%v\n", ts, e.Connected, e.Err)
		case connection.Message:
			line = fmt.Sprintf("%s MSG lines=%d gmcp=%d prompt=%q\n",
				ts, len(e.Lines), len(e.GMCP), sanitize(e.Prompt.Bytes))
		case ui.Input:
			line = fmt.Sprintf("%s INP %s\n", ts, sanitize(e.Bytes))
		case ui.Resize:
			line = fmt.Sprintf("%s RSZ %dx%d\n", ts, e.Width, e.Height)
		case ui.ReFormatting:
			line = fmt.Sprintf("%s RFM id=%d\n", ts, e.Line.ID)
		default:
			line = fmt.Sprintf("%s ??? %T\n", ts, batch.Event)
		}

		if _, err := file.WriteString(line); err != nil {
			return batch, fmt.Errorf("failed to write event log: %w", err)
		}
		return batch, nil
	}, nil
}

// sanitize collapses an event's payload onto a single log line: ANSI escape
// sequences stripped, embedded newlines and carriage returns replaced with
// a visible glyph, long payloads truncated. We care about boundaries
// between events, not faithful content.
func sanitize(payload []byte) string {
	const maxLen = 120

	out := make([]byte, 0, len(payload))
	for i := 0; i < len(payload); i++ {
		c := payload[i]
		if c == 0x1b && i+1 < len(payload) && payload[i+1] == '[' {
			j := i + 2
			for j < len(payload) && payload[j] != 'm' {
				j++
			}
			i = j
			continue
		}
		switch c {
		case '\n':
			out = append(out, '\\', 'n')
		case '\r':
			out = append(out, '\\', 'r')
		case '\t':
			out = append(out, '\\', 't')
		default:
			out = append(out, c)
		}
	}

	if len(out) > maxLen {
		return string(out[:maxLen]) + "…"
	}
	return string(out)
}

// gmcpHead returns the GMCP module.message head — the first
// whitespace-separated token of the frame payload — so the log shows e.g.
// "Char.Vitals" instead of the full JSON body.
func gmcpHead(payload []byte) []byte {
	if i := bytes.IndexAny(payload, " \t"); i > 0 {
		return payload[:i]
	}
	return payload
}
