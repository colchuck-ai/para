package journal

import "time"

// NewChange constructs a change event (§3.1): the entity's own field
// changed from one value to another, optionally carrying a reason.
func NewChange(at time.Time, field, from, to, note string) Event {
	return Event{At: at, Kind: KindChange, Field: field, From: from, To: to, Note: note}
}

// NewMeasurement constructs a measurement event (§3.1): a key-result's
// reading, in its type's grammar (§4).
func NewMeasurement(at time.Time, value, note string) Event {
	return Event{At: at, Kind: KindMeasurement, Value: value, Note: note}
}

// NewNote constructs a note event (§3.1), landable on any entity or
// container.
func NewNote(at time.Time, note string) Event {
	return Event{At: at, Kind: KindNote, Note: note}
}

// NewChild constructs a child event (§3.1): logged at the parent, naming
// the operation and the child affected. from/to are populated for op ==
// ChildOpMoved and empty otherwise.
func NewChild(at time.Time, op ChildOp, child, from, to, note string) Event {
	return Event{At: at, Kind: KindChild, Op: op, Child: child, From: from, To: to, Note: note}
}
