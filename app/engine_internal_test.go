package app

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCheckEventBufferFill exercises the threshold tracking: each
// upward crossing logs once, downward crossings reset, and crossings
// of multiple thresholds in one step still log only once (so a burst
// jump from 0 to 95% doesn't spam three lines).
func TestCheckEventBufferFill(t *testing.T) {
	var buf bytes.Buffer

	origOut := log.Writer()
	origFlags := log.Flags()

	log.SetOutput(&buf)
	log.SetFlags(0)

	t.Cleanup(func() {
		log.SetOutput(origOut)
		log.SetFlags(origFlags)
	})

	below := eventBufferWarnThresholds[0] - 1
	at50 := eventBufferWarnThresholds[0]
	at75 := eventBufferWarnThresholds[1]
	at90 := eventBufferWarnThresholds[2]

	// Below the first threshold: no warning.
	idx := checkEventBufferFill(below, 0)
	assert.Equal(t, 0, idx)
	assert.Empty(t, buf.String(), "no warning while below 50%%")

	// First crossing into 50% bucket: one warning.
	idx = checkEventBufferFill(at50, idx)
	assert.Equal(t, 1, idx)
	assert.Equal(t, 1, strings.Count(buf.String(), "event buffer at"))

	// Stay in 50% bucket: no new warning.
	idx = checkEventBufferFill(at50+1, idx)
	assert.Equal(t, 1, idx)
	assert.Equal(t, 1, strings.Count(buf.String(), "event buffer at"))

	// Cross into 75% bucket: another warning.
	idx = checkEventBufferFill(at75, idx)
	assert.Equal(t, 2, idx)
	assert.Equal(t, 2, strings.Count(buf.String(), "event buffer at"))

	// Cross into 90% bucket: another warning.
	idx = checkEventBufferFill(at90, idx)
	assert.Equal(t, 3, idx)
	assert.Equal(t, 3, strings.Count(buf.String(), "event buffer at"))

	// Drain back below all thresholds: silent (downward shouldn't log).
	idx = checkEventBufferFill(0, idx)
	assert.Equal(t, 0, idx)
	assert.Equal(t, 3, strings.Count(buf.String(), "event buffer at"))

	// Rise again to 75%: logs again (one warning, not two — the bucket
	// index jumps from 0 to 2 but only the highest threshold is logged).
	idx = checkEventBufferFill(at75, idx)
	assert.Equal(t, 2, idx)
	assert.Equal(t, 4, strings.Count(buf.String(), "event buffer at"))
}
