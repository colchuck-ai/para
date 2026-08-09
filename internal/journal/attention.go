package journal

import "time"

// Attention implements §3.6's one clock: the newest `at` among note or
// measurement events, else created. change and child events never count —
// recording that something changed is not the same as attending to it, and
// a `child` event on a parent is the child's news, not the parent's.
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
		switch e.Kind {
		case KindNote, KindMeasurement:
			if e.At.After(attention) {
				attention = e.At
			}
		}
	}
	return attention
}
