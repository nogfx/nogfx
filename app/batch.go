package app

// Batch is one unit of work flowing through the processor chain. Each
// batch corresponds to a single triggering event; processors may read
// that trigger, append derived events that will become their own batches
// downstream, and append commands the engine will apply to endpoints.
//
// Within a batch the engine applies all Commands (in order) before any
// derived Events are re-emitted, and derived Events are re-emitted in
// order before any apply-consequence event from an endpoint is
// processed. See Engine.Run for the loop that enforces this contract.
type Batch struct {
	// Event is the trigger that started this batch. A processor decides
	// whether to act based on its type. Processors may replace it (to
	// transform the trigger for subsequent processors in the chain) or
	// set it to nil to suppress the rest of the chain for this batch.
	Event Event

	// Events collects derived events that will be re-emitted as their
	// own batches after this batch's commands have been applied.
	Events []Event

	// Commands collects commands to dispatch to the endpoints when the
	// chain has finished running.
	Commands []Command
}

func (b Batch) AppendEvent(e Event) Batch {
	b.Events = append(b.Events, e)

	return b
}

func (b Batch) AppendCommand(c Command) Batch {
	b.Commands = append(b.Commands, c)

	return b
}
