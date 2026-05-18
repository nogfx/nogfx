package ui

// Line carries one unit of user-facing output through the chain. Raw is
// the exact bytes from the source (used by reformatters to re-match), and
// Formatted is what the UI should currently render. ID is an opaque
// per-line identity assigned by the UI on first print; processors pass it
// through unchanged when rewriting, so the UI can find the existing
// scrollback slot and overwrite it.
//
// New prints set ID == 0; the UI assigns the real ID on receive.
// Replacements (emitted in response to ReFormatting events) reuse the
// incoming line's ID so the UI overwrites in place instead of appending.
type Line struct {
	Raw       []byte
	Formatted []byte
	ID        uint64
}
