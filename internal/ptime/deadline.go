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

// DeadlineOf is Deadline for a stored `due` that may be absent, reporting ok
// false where there is no readable deadline at all — §4.2's undefined-pace case
// and §17's "not overdue because there is nothing to be late for".
//
// A `due` that will not parse answers the same way as one that is not there.
// Both are doctor's `invalid` finding to report (§10) and neither is a reason a
// read should fail, so neither may become a deadline nobody wrote.
func DeadlineOf(due string) (time.Time, bool) {
	if due == "" {
		return time.Time{}, false
	}
	t, err := Deadline(due)
	return t, err == nil
}

// StoredAt reads a timestamp para wrote, which is always UTC RFC 3339 (§15.1)
// but is parsed through the progressive-precision grammar so that a hand-edited
// `created = "2026-01-01"` is still read rather than discarded.
//
// ok is false for an absent or unreadable value — again doctor's `invalid`
// finding and not a failure — and the caller must carry that bool rather than
// test the returned time against zero: the zero time is a real instant, and
// treating it as one has already produced one defect (see krvalue.Assessment's
// HasCreated).
func StoredAt(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := ParseAt(s, time.UTC)
	return t, err == nil
}
