package ptime

import (
	"strings"
	"time"
)

// Deadline is the last instant a stored `due` still admits, in UTC.
//
// A bare date is a whole day, not the midnight that opens it: "due 2026-09-30"
// means by the end of the 30th, so something created today with a deadline of
// today has to be legal (§15's `created ≤ due` bound) and must not read as
// overdue while the day is still running (§4.3's `missed`). A `due` carrying a
// time of day is that instant exactly — the precision the user typed is the
// precision they meant.
//
// It lives here rather than beside either caller because the two questions it
// answers are asked from opposite ends of the system: the write path checks a
// bound against it (§15) and the read path derives `missed` and "in 181 days"
// from it (§4.3, §16.1). One deadline per stored `due`, or `measure` and `show`
// disagree about whether the same key-result is late.
//
// `due` is stored exactly as typed rather than resolved to UTC at write time
// (§15.1 governs *instants*, and a deadline is a date somebody chose), so the
// stored string is the only place its precision survives — which is why this
// takes the string and not a parsed time.
func Deadline(due string) (time.Time, error) {
	t, err := ParseAt(due, time.UTC)
	if err != nil {
		return time.Time{}, err
	}
	if strings.Contains(due, "T") {
		return t, nil
	}
	// AddDate rather than a fixed 24 hours: a bare date carries no offset and
	// is therefore read in UTC, but going through the calendar keeps the
	// arithmetic honest about month and year ends.
	return t.AddDate(0, 0, 1).Add(-time.Second), nil
}
