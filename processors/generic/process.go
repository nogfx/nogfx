// Package generic holds MUD-agnostic processors that operate on
// app.Batch values: input/output translation, telnet-option negotiation,
// turn aggregation, the heartbeat and lag-watching pair, the tracker
// queue, and assorted utilities (SplitInputProcessor, RepeatInputProcessor,
// LogProcessor, EventLogProcessor, the simpex-based MatchInput /
// MatchOutput helpers).
//
// Worlds compose these alongside their own processors when assembling
// their chain.
package generic

import "github.com/nogfx/nogfx/app"

// Processor is a convenience alias for app.Processor so callers can spell
// the type as generic.Processor where it reads more naturally.
type Processor = app.Processor
