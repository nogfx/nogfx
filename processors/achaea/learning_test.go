package achaea_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nogfx/nogfx/app"
	"github.com/nogfx/nogfx/app/connection"
	"github.com/nogfx/nogfx/processors/achaea"
)

// send is a small helper that wraps a Send command.
func send(s string) connection.Send {
	return connection.Send{Bytes: []byte(s)}
}

// line is a small helper that wraps a TextLine event.
func line(s string) connection.TextLine {
	return connection.TextLine{Bytes: []byte(s)}
}

// sendStrings extracts the bytes of every connection.Send command.
func sendStrings(b app.Batch) []string {
	var out []string

	for _, c := range b.Commands {
		if s, ok := c.(connection.Send); ok {
			out = append(out, string(s.Bytes))
		}
	}

	return out
}

// textLines returns the batch's trigger event as a TextLine string, or
// nil if it isn't a TextLine. Tests use this to check whether the trigger
// passed through (or was rewritten) or was suppressed (Event == nil).
func textLines(b app.Batch) []string {
	tl, ok := b.Event.(connection.TextLine)
	if !ok {
		return nil
	}

	return []string{string(tl.Bytes)}
}

func TestLearning_SmallAmountUntouched(t *testing.T) {
	lrn := &achaea.Learning{}
	p := lrn.Processor()

	got, err := p(app.Batch{Commands: []app.Command{send("learn 5 swordsmanship from Galen")}})
	require.NoError(t, err)
	assert.Equal(t, []string{"learn 5 swordsmanship from Galen"}, sendStrings(got),
		"requests at or below maxLessons should not be chunked")
}

func TestLearning_LargeAmountSplitIntoFirstChunk(t *testing.T) {
	lrn := &achaea.Learning{}
	p := lrn.Processor()

	got, err := p(app.Batch{Commands: []app.Command{send("learn 35 swordsmanship from Galen")}})
	require.NoError(t, err)
	require.Len(t, got.Commands, 1)

	first := string(got.Commands[0].(connection.Send).Bytes)
	assert.Equal(t, "learn 15 swordsmanship from Galen", first,
		"the first chunk should be sized at maxLessons")
}

func TestLearning_ChainsToCompletion(t *testing.T) {
	lrn := &achaea.Learning{}
	p := lrn.Processor()

	// User submits 25 lessons.
	got, err := p(app.Batch{Commands: []app.Command{send("learn 25 swordsmanship from Galen")}})
	require.NoError(t, err)
	require.Equal(t, []string{"learn 15 swordsmanship from Galen"}, sendStrings(got))

	// Server confirms the first session's begin → progress line shown.
	got, err = p(app.Batch{Event: line("Galen begins the lesson in Swordsmanship.")})
	require.NoError(t, err)
	require.Len(t, textLines(got), 1)
	assert.Contains(t, textLines(got)[0], "15 of 25 lessons learned")

	// Continue line is suppressed.
	got, err = p(app.Batch{Event: line("Galen continues your training in Swordsmanship.")})
	require.NoError(t, err)
	assert.Empty(t, textLines(got), "continue lines should be suppressed")

	// Finish line triggers next chunk + a progress update.
	got, err = p(app.Batch{Event: line("Galen finishes the lesson in Swordsmanship.")})
	require.NoError(t, err)
	require.Equal(t, []string{"learn 10 swordsmanship from Galen"}, sendStrings(got),
		"finish should queue the next chunk")
	require.Len(t, textLines(got), 1)
	assert.Contains(t, textLines(got)[0], "of 25 lessons learned")

	// The second session's begin should NOT show another progress line
	// (only the first begin of the whole sequence does).
	got, err = p(app.Batch{Event: line("Galen begins the lesson in Swordsmanship.")})
	require.NoError(t, err)
	assert.Empty(t, textLines(got), "subsequent begin lines stay suppressed")

	// The final finish completes the sequence: show "25 of 25" and stop.
	got, err = p(app.Batch{Event: line("Galen finishes the lesson in Swordsmanship.")})
	require.NoError(t, err)
	assert.Empty(t, sendStrings(got), "no more chunks after completion")
	require.Len(t, textLines(got), 1)
	assert.True(t, strings.HasPrefix(textLines(got)[0], "25 of 25 lessons learned"))

	// State has been reset; further finish lines pass through unchanged.
	got, err = p(app.Batch{Event: line("Galen finishes the lesson in Swordsmanship.")})
	require.NoError(t, err)
	assert.Empty(t, sendStrings(got))
	require.Len(t, textLines(got), 1)
	assert.Equal(t, "Galen finishes the lesson in Swordsmanship.", textLines(got)[0],
		"unrelated finish lines pass through once the session has ended")
}

func TestLearning_AlternativeFinishPatterns(t *testing.T) {
	lrn := &achaea.Learning{}
	p := lrn.Processor()

	_, err := p(app.Batch{Commands: []app.Command{send("learn 20 inscription from Belluno")}})
	require.NoError(t, err)

	// The "bows to you - the lesson in X is over" variant should also
	// drive the chain.
	got, err := p(app.Batch{Event: line("Belluno bows to you - the lesson in Inscription is over.")})
	require.NoError(t, err)
	assert.Equal(t, []string{"learn 5 inscription from Belluno"}, sendStrings(got))
}

func TestLearning_NonNumericPrefixPassesThrough(t *testing.T) {
	lrn := &achaea.Learning{}
	p := lrn.Processor()

	got, err := p(app.Batch{Commands: []app.Command{send("learn x from Galen")}})
	require.NoError(t, err)
	assert.Equal(t, []string{"learn x from Galen"}, sendStrings(got))
}
