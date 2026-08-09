package render

import (
	"bytes"
	"sort"
)

// ActivityDrift reports the earliest day whose section differs between the
// ACTIVITY.md on disk and the one re-derived from the journal, and whether a
// day could be named at all.
//
// It is what makes §10's report dated rather than merely detected: "its report
// names the file and, for ACTIVITY.md, the earliest day that differs — so drift
// is dated rather than merely detected". Earliest rather than latest, because
// the earliest differing day is where the divergence began, and the days after
// it are usually consequences of the same edit; a report naming the newest one
// would point at the end of a story and leave the reader to find its start.
//
// A day is named when the two files disagree about one: a section present in
// one and absent from the other, or present in both with different lines.
// Anything the day sections cannot account for — a file with prose before its
// first heading, day headers out of order, a difference that lies only in the
// separators between sections — answers false, and the caller reports the file
// without a date. That is the honest answer: the drift is real, and no single
// day is where it is.
func ActivityDrift(existing, derived []byte) (string, bool) {
	// The file on disk is the one that may be malformed; the derived side comes
	// from a renderer and always parses.
	before, err := parseActivity(existing)
	if err != nil {
		return "", false
	}
	after, err := parseActivity(derived)
	if err != nil {
		return "", false
	}

	bodies := func(sections []section) map[string][]byte {
		out := make(map[string][]byte, len(sections))
		for _, s := range sections {
			out[s.Day] = s.Body
		}
		return out
	}
	beforeDays, afterDays := bodies(before), bodies(after)

	// Both slices are walked, never the maps, so nothing here depends on a
	// map's iteration order (§0.2).
	var days []string
	seen := map[string]bool{}
	for _, s := range append(append([]section{}, before...), after...) {
		if !seen[s.Day] {
			seen[s.Day] = true
			days = append(days, s.Day)
		}
	}
	sort.Strings(days)

	for _, day := range days {
		b, hadBefore := beforeDays[day]
		a, hadAfter := afterDays[day]
		if hadBefore != hadAfter || !bytes.Equal(b, a) {
			return day, true
		}
	}
	return "", false
}
