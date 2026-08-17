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
// container. It moves the attention clock (§3.6, §18.6) — see
// NewNoteNoAttention for the note that does not.
func NewNote(at time.Time, note string) Event {
	return Event{At: at, Kind: KindNote, Note: note}
}

// NewNoteNoAttention constructs a note event that does not move the attention
// clock (§3.6, §18.6): `para note --no-attention` for a note that records
// something true about an entity without claiming the entity was tended,
// which is the one distinction §3.6's "newest note or measurement, full stop"
// left a stale-after threshold with no way to make for itself.
//
// It is a sibling constructor rather than a bool parameter on NewNote, for
// the same reason Env.Add and Env.AddDryRun are two methods rather than one
// with a dryRun bool (internal/mutate/add.go): NewNote already has on the
// order of 75 call sites across this repo's tests, every one of them meaning
// an ordinary attending note, and a parameter would touch all of them for a
// flag almost none of them want. mutate.Env.Note took the opposite shape —
// see its own comment for why a smaller call-site count changes the trade.
func NewNoteNoAttention(at time.Time, note string) Event {
	e := NewNote(at, note)
	e.NoAttention = true
	return e
}

// NewChild constructs a child event (§3.1): logged at the parent, naming
// the operation and the child affected. from/to are populated for op ==
// ChildOpMoved and empty otherwise.
func NewChild(at time.Time, op ChildOp, child, from, to, note string) Event {
	return Event{At: at, Kind: KindChild, Op: op, Child: child, From: from, To: to, Note: note}
}
