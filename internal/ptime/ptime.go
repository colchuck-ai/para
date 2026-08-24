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

// Stamp formats t as every cached instant is stored (§15.1, §28.4): UTC,
// RFC 3339. mutate's write-through step, doctor's stale-projection check, and
// rebuild's backfill must all format the same instant identically, or a
// correct derivation would still compare as stale against a cache that only
// differs in how it was printed.
func Stamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// journalFilenameLayout has no separators, and the "Z" JournalFilename appends
// is part of the format rather than decoration: the name is always UTC.
const journalFilenameLayout = "20060102T150405"

// JournalFilename formats t as a journal file's name (§3.4), in UTC.
//
// §3.4 makes two guarantees that rest on this name: "the newest file is the
// last one lexically", and "closed journal files are immutable". Both are
// claims about string order, and a name in the writer's own zone cannot keep
// them — with no offset recorded in the name, an event at 23:00+13:00 (10:00Z)
// sorts *after* a later event at 12:00-07:00 (19:00Z). Append would then pick a
// closed file as the newest and reopen it. The DST fall-back hour reproduces the
// same inversion annually without anyone leaving their desk.
//
// UTC is the only zone in which lexical order is total, so it is the only zone
// in which those two guarantees hold. A filename is an ordering key that happens
// to be legible, not a timestamp anyone reads for its wall clock; the trailing Z
// says so.
func JournalFilename(t time.Time) string {
	return t.UTC().Format(journalFilenameLayout) + "Z.jsonl"
}
