package processors

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/connection"
	"github.com/nogfx/nogfx/ui"
)

// LogProcessor writes the textual contents of the batch (server output,
// prompts, and outgoing send commands) to the given file path. The parent
// directories are created if they don't already exist.
func LogProcessor(dir, filename string) (Processor, error) {
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create logs directory %q: %w", dir, err)
	}

	path := filepath.Join(dir, filename)

	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file %q: %w", path, err)
	}

	return func(batch app.Batch) (app.Batch, error) {
		for _, ev := range batch.Events {
			var bs []byte
			switch e := ev.(type) {
			case connection.TextLine:
				bs = append(append([]byte{}, e.Bytes...), '\n')
			case connection.Prompt:
				bs = append(append([]byte{}, e.Bytes...), '\n')
			case ui.Input:
				bs = append(append([]byte("> "), e.Bytes...), '\n')
			}
			if len(bs) == 0 {
				continue
			}
			if _, err := file.Write(bs); err != nil {
				return batch, fmt.Errorf("failed to write to log: %w", err)
			}
		}
		return batch, nil
	}, nil
}
