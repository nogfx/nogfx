package ui

// Target is the UI-facing snapshot of the current target. World adapters
// project their rich state into this shape when emitting SetTarget.
type Target struct {
	// Name is the displayable identifier of the target.
	Name string

	// Health is a number on a 0-100 scale representing the target's
	// remaining health. -1 means unknown.
	Health int

	// Queue is the number of additional valid targets present beyond the
	// current one. The UI may render this as e.g. " (+2)" alongside the
	// name. Zero means no extras.
	Queue int
}
