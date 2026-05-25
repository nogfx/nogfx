package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatLag(t *testing.T) {
	tcs := map[string]struct {
		lag  time.Duration
		want string
	}{
		"sub-second renders as ms": {
			lag:  42 * time.Millisecond,
			want: "42ms",
		},
		"four-digit ms still fits": {
			lag:  999 * time.Millisecond,
			want: "999ms",
		},
		"second-boundary switches to seconds": {
			lag:  1500 * time.Millisecond,
			want: "1.5s",
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, formatLag(tc.lag))
		})
	}
}

func TestRenderLag_BlankWhenZero(t *testing.T) {
	tui := newRenderLagTUI(0)

	row := tui.RenderLag(lagWidth)

	assert.Len(t, row, lagWidth)
	assert.Equal(t, "        ", row.String())

	for _, cell := range row {
		assert.Equal(t, lagStyle, cell.Style, "cells must carry the grey lag style")
	}
}

func TestRenderLag_RightAlignsMeasurement(t *testing.T) {
	tui := newRenderLagTUI(42 * time.Millisecond)

	row := tui.RenderLag(lagWidth)

	// " 42ms " (right-aligned, one-cell trailing gap, width 8).
	assert.Equal(t, "   42ms ", row.String())
}

func TestRenderLag_CachesUntilInvalidated(t *testing.T) {
	tui := newRenderLagTUI(50 * time.Millisecond)

	first := tui.RenderLag(lagWidth).String()

	// Mutating lag directly (bypassing Apply, no cache invalidation) is
	// what the cache test wants to catch — the cached row should win.
	tui.lagMu.Lock()
	tui.lag = 500 * time.Millisecond
	tui.lagMu.Unlock()

	second := tui.RenderLag(lagWidth).String()
	assert.Equal(t, first, second, "cached row must persist until setCache(paneLag, nil)")

	tui.setCache(paneLag, nil)

	third := tui.RenderLag(lagWidth).String()
	assert.NotEqual(t, first, third, "invalidated cache must re-render")
	assert.Equal(t, "  500ms ", third)
}

// newRenderLagTUI constructs a TUI with just enough state for the
// lag-rendering tests. It avoids the full screen wiring that NewTUI does
// because RenderLag only touches the cache and the lag field.
func newRenderLagTUI(lag time.Duration) *TUI {
	return &TUI{
		panesCache: map[string]Rows{},
		input:      &Input{},
		lag:        lag,
	}
}

func TestRenderInputGutter_FillsEveryRowWithGreyBackground(t *testing.T) {
	tui := newRenderLagTUI(0)

	rows := tui.RenderInputGutter(lagWidth, 3)

	assert.Len(t, rows, 3, "gutter must cover every input row")

	for i, row := range rows {
		assert.Lenf(t, row, lagWidth, "row %d", i)
		assert.Equalf(t, "        ", row.String(), "row %d should be all-spaces", i)
	}
}

func TestRenderInputGutter_ZeroWidthOrHeightYieldsNothing(t *testing.T) {
	tui := newRenderLagTUI(0)

	assert.Empty(t, tui.RenderInputGutter(0, 2), "zero width must yield no rows")
	assert.Empty(t, tui.RenderInputGutter(lagWidth, 0), "zero height must yield no rows")
}
