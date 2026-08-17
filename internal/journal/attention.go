package journal

import "time"

// Attention implements §3.6's one clock: the newest `at` among note or
// measurement events, else created. change and child events never count —
// recording that something changed is not the same as attending to it, and
// a `child` event on a parent is the child's news, not the parent's.
//
// A note whose NoAttention is set never counts either (§18.6): it exists so a
// note can record something true without claiming the entity was tended,
// which is the one thing that used to let any note — however unrelated to
// what a stale-after threshold watches — silently satisfy it. A measurement
// has no such flag: a key-result's reading is inherently the activity its own
// clock watches, so there is nothing for it to opt out of.
//
// It folds a slice of events already in hand, and AttentionAt is the same
// question asked of a journal on disk. **AttentionAt is what production
// calls**; this is the plain statement of the rule, kept because it is what
// AttentionAt is proved equal to. A cheap read that nothing checks against the
// definition is a cheap read that will quietly stop being the same answer —
// which has already happened once here, when an earlier AttentionAt stopped at
// the newest file holding a match (see bounds.go).
func Attention(events []Event, created time.Time) time.Time {
	attention := created
	for _, e := range events {
		if !attends(e) {
			continue
		}
		if e.At.After(attention) {
			attention = e.At
		}
	}
	return attention
}

// attends reports whether e is one §3.6's clock counts: a measurement, or a
// note that has not opted out with NoAttention. It is the one place that
// question is answered, shared by Attention and AttentionEvent so a future
// third kind of "did this count" caller cannot drift from either.
func attends(e Event) bool {
	switch e.Kind {
	case KindMeasurement:
		return true
	case KindNote:
		return !e.NoAttention
	default:
		return false
	}
}
