// Package processors holds generic processors that operate on app.Batch
// values: the Input/Decode/Render baseline transformations, plus
// world-agnostic utilities (SplitInputProcessor, RepeatInputProcessor,
// LogProcessor, and the simpex-based MatchInput/MatchOutput helpers).
//
// Worlds compose these alongside their own processors when assembling
// their chain.
package processors

import "github.com/tobiassjosten/nogfx/app"

// Processor is a convenience alias for app.Processor so callers can spell
// the type as processors.Processor where it reads more naturally.
type Processor = app.Processor
