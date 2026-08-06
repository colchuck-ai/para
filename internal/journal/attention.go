package journal

import "time"

// Attention implements §3.6's one clock: the newest `at` among note or
// measurement events, else created. change and child events never count —
// recording that something changed is not the same as attending to it, and
// a `child` event on a parent is the child's news, not the parent's.
//
// It folds a slice of events already in hand. AttentionAt is the same
// question asked of a journal on disk, and the two agree by test.
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
