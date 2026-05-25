package tui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

// lagStyle is the grey-on-white style shared with the input row, so the
// widget reads as one continuous strip across the bottom of the screen.
var lagStyle = (tcell.Style{}).
	Foreground(tcell.ColorWhite).
	Background(tcell.Color235)

// RenderLag renders the lag widget at the given width. A zero current lag
// (no measurement yet, or explicitly cleared) renders as a blank grey
// strip; a measured value renders right-aligned with a trailing one-cell
// gap so the digits don't touch the screen edge.
//
// The read of tui.lag and the cache write are both performed under
// lagMu so a concurrent Apply that updates lag and clears the cache
// cannot interleave between them — without this, RenderLag could
// repopulate the cache with a stale value just after Apply cleared it.
// Apply takes the same locks in the same order (lagMu, then cacheMutex
// via setCache).
func (tui *TUI) RenderLag(width int) Row {
	if rows, ok := tui.getCache(paneLag); ok {
		if len(rows) > 0 {
			return rows[0]
		}

		return Row{}
	}

	row := NewRow(width, NewCell(' ', lagStyle))

	tui.lagMu.Lock()
	defer tui.lagMu.Unlock()

	if tui.lag > 0 {
		text := formatLag(tui.lag)

		// Right-align with one-cell padding on the right.
		start := width - 1 - len(text)
		if start < 0 {
			start = 0
		}

		for i, r := range text {
			pos := start + i
			if pos >= len(row) {
				break
			}

			row[pos].Content = r
		}
	}

	tui.setCache(paneLag, Rows{row})

	return row
}

// formatLag turns a measured lag into a short display string. Sub-second
// values render as integer milliseconds ("42ms"); anything beyond renders
// as one-decimal seconds ("1.4s") so the widget stays inside its
// reserved cells.
func formatLag(d time.Duration) string {
	if d >= time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}

	return fmt.Sprintf("%dms", d.Milliseconds())
}
