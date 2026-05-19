package generic

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/app/ui"
)

// LogProcessor writes the textual contents of the batch (server output,
// prompts, and outgoing send commands) to the given file path. The parent
// directories are created if they don't already exist.
func LogProcessor(dir, filename string) (Processor, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create logs directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, filename)

	// #nosec G304 -- dir is supplied by the world's processor (a known
	// per-session log directory under pkg.Directory); filename is a
	// timestamp-formatted string. The "variable path" warning is a false
	// positive in this controlled-input setting.
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %q: %w", path, err)
	}

	return func(batch app.Batch) (app.Batch, error) {
		var bs []byte

		switch e := batch.Event.(type) {
		case connection.TextLine:
			bs = append(append([]byte{}, e.Bytes...), '\n')
		case connection.Prompt:
			bs = append(append([]byte{}, e.Bytes...), '\n')
		case ui.Input:
			bs = append(append([]byte("> "), e.Bytes...), '\n')
		}

		if len(bs) == 0 {
			return batch, nil
		}

		if _, err := file.Write(bs); err != nil {
			return batch, fmt.Errorf("failed to write to log: %w", err)
		}

		return batch, nil
	}, nil
}
