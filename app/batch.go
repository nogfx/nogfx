package app

// Batch is the unit of work flowing through the processor chain. It collects
// the events that triggered this pass and the events and commands produced as
// processors run. Keeping events and commands together as the batch flows
// gives later processors (and scripts) the context to understand why each
// item was added.
type Batch struct {
	Events   []Event
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
