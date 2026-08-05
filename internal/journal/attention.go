package journal

import "time"

// Attention implements §3.6's one clock: the newest `at` among note or
// measurement events, else created. change and child events never count —
// including status changes and containment changes — because if any of
// them reset the clock, a silent field edit would buy silence from every
// staleness check.
func Attention(events []Event, created time.Time) time.Time {
	attention := created
	for _, e := range events {
		if e.Kind != KindNote && e.Kind != KindMeasurement {
			continue
		}
		if e.At.After(attention) {
			attention = e.At
		}
	}
	return attention
}
