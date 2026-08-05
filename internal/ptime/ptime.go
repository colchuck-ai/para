// Package ptime implements the progressive-precision timestamp grammar
// `--at` accepts (design §15.1), the two bounds settable timestamps are
// checked against, the exact-instant comparison §3.1's measurement-collision
// rule depends on, and journal filename formatting (§3.4). It is pure: every
// function takes the instants and locations it needs as arguments and calls
// neither time.Now nor the filesystem.
package ptime

import (
	"time"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// atLayouts are §15.1's progressive-precision forms, in order of increasing
// precision: a bare date, +hour, +minute, +second. Each is tried as a
// complete match against the input — time.ParseInLocation rejects any
// leftover text — so a fuller timestamp never misparses against a shorter
// layout.
var atLayouts = []string{
	"2006-01-02",
	"2006-01-02T15",
	"2006-01-02T15:04",
	"2006-01-02T15:04:05",
}

// ParseAt parses s per §15.1: increasing precision — date, +hour, +minute,
// +second, or a full RFC 3339 timestamp carrying its own offset — with
// everything absent zero-filled in loc. A bare date is midnight in loc.
func ParseAt(s string, loc *time.Location) (time.Time, error) {
	for _, layout := range atLayouts {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, paraerr.Newf(paraerr.KindValidation,
		"%q is not a valid timestamp (date, +hour, +minute, +second, or full RFC 3339)", s)
}

// CheckNotFuture reports an error if t is after now — the one bound shared
// by every settable timestamp (§15.1, and `created`'s first bound, §15).
func CheckNotFuture(t, now time.Time) error {
	if t.After(now) {
		return paraerr.Newf(paraerr.KindValidation, "%s is in the future", t.Format(time.RFC3339))
	}
	return nil
}

// CheckNotAfterDue reports an error if t is after due — `created`'s second
// bound (§15): a creation time may not postdate the thing's own deadline.
func CheckNotAfterDue(t, due time.Time) error {
	if t.After(due) {
		return paraerr.Newf(paraerr.KindValidation, "%s is after due %s", t.Format(time.RFC3339), due.Format(time.RFC3339))
	}
	return nil
}

// Equal reports whether a and b name the exact same instant. It exists
// because §3.1 compares measurement timestamps "at the exact stored
// instant, not the day" — Time.Equal is the correct comparison (unlike ==,
// which also compares monotonic readings and wall-clock representation) and
// this wrapper makes that the only spelling callers reach for.
func Equal(a, b time.Time) bool {
	return a.Equal(b)
}

// journalFilenameLayout has no separators or offset, matching the design's
// own worked examples (e.g. "20260101T080801.jsonl") — a directory listing
// sorts lexically into a chronology (§3.4) because the layout is
// fixed-width and monotonically increasing with the instant it names.
const journalFilenameLayout = "20060102T150405"

// JournalFilename formats t as a journal file's name (§3.4): a new file is
// named for its own first event, in t's own location.
func JournalFilename(t time.Time) string {
	return t.Format(journalFilenameLayout) + ".jsonl"
}
