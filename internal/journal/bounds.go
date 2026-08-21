package journal

import (
	"path/filepath"
	"time"
)

// LatestOf and OldestOf are the two ends of an entity's history, filtered to
// the given event kinds — the three questions every read command asks of a
// journal without wanting the journal itself:
//
//   - §3.6's clock is the newest note or measurement (LatestOf);
//   - §4.2's `current` is the newest reading (LatestOf);
//   - §4.1's default baseline is the oldest reading (OldestOf).
//
// Both scan every file in the journal. That is not a missed optimisation: the
// obvious shortcut — read backwards and stop at the newest *file* holding a
// match — is unsound, and the reason is worth recording, because §3.4's
// naming makes it look safe. A file is named for its own first event and
// rotation only ever opens a new file, so the files are in chronological order
// *by write time*. `at` is a different clock. It is bounded above (§15.1
// refuses the future) and not below, so `para note x --at 2026-01-05` written
// after a rotation puts a January event in the newest file while the real
// newest note sits in the file before it. Stopping there would answer with the
// backdated one, which is precisely §3.1's "ordering comes from `at`, never
// from file position" being violated by the reader.
//
// What the filter does buy is that neither function decodes an entity's whole
// history into a slice, and neither sorts: one pass, two comparisons per
// matching line. ReadAll remains the call for anything wanting the events.
//
// ok is false when no event of those kinds exists, which for LatestOf is the
// case §3.6 falls back to `created` for and for OldestOf the case §4.1 has no
// baseline in.
func LatestOf(dir string, kinds ...Kind) (Event, bool, error) {
	return bound(dir, kinds, nil, func(candidate, best Event) bool {
		return candidate.At.After(best.At)
	})
}

// OldestOf is the earliest event of the given kinds, by `at`. See LatestOf.
func OldestOf(dir string, kinds ...Kind) (Event, bool, error) {
	return bound(dir, kinds, nil, func(candidate, best Event) bool {
		return candidate.At.Before(best.At)
	})
}

// bound walks every journal file in dir and keeps the one matching event that
// better satisfies the comparison.
//
// extra is a second filter beyond kind, checked only when non-nil. It exists
// for AttentionAt alone: every other caller's question is answered by kind by
// itself, and giving them an unused nil to pass is cheaper than a second
// almost-identical walk, which is the exact drift bound exists to prevent
// (see AttentionAt's own comment on the bug an earlier "cheap shortcut" here
// caused).
//
// Ties keep the first encountered, which is file-then-line order — the same
// tie-break ReadAll's stable sort gives, so the two agree on a journal holding
// two events at one instant.
func bound(dir string, kinds []Kind, extra func(Event) bool, better func(candidate, best Event) bool) (Event, bool, error) {
	names, err := listJSONL(dir)
	if err != nil {
		return Event{}, false, err
	}
	want := make(map[Kind]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}

	var best Event
	found := false
	for _, name := range names {
		events, err := readFile(filepath.Join(dir, name))
		if err != nil {
			return Event{}, false, err
		}
		for _, e := range events {
			if !want[e.Kind] {
				continue
			}
			if extra != nil && !extra(e) {
				continue
			}
			if !found || better(e, best) {
				best, found = e, true
			}
		}
	}
	return best, found, nil
}

// LatestAt is the `at` of the newest event of the given kinds — LatestOf when
// only the instant is wanted, which is what §3.6's clock is.
func LatestAt(dir string, kinds ...Kind) (time.Time, bool, error) {
	e, ok, err := LatestOf(dir, kinds...)
	return e.At, ok, err
}

// AttentionAt is §3.6's clock for the entity whose journal is in dir: the
// newest note or measurement, else `created`.
//
// It is Attention over the journal on disk rather than over a slice of events,
// so a read command can answer it for every entity it lists without decoding
// each one's whole history. The two must agree — that is a test — because
// `review --stale` and `list --sort attention` would otherwise rank a tree
// differently from the way `show` describes each row of it.
//
// It is a thin wrapper over AttentionEvent: the instant alone is what most
// callers want, and computing it independently would be a second walk asking
// the same question bound's own comment warns against.
func AttentionAt(dir string, created time.Time) (time.Time, error) {
	e, ok, err := AttentionEvent(dir, created)
	if err != nil {
		return time.Time{}, err
	}
	if !ok {
		return created, nil
	}
	return e.At, nil
}

// AttentionEvent is AttentionAt's whole answer, not just its instant: the
// event that set the clock, or ok=false when nothing on disk beats `created`
// (an entity with no qualifying event yet, or a hand-edited `created` later
// than every one it has).
//
// It exists so a read command can say *what* set attention — a note's first
// words, or "measurement" — rather than only how long ago, which is
// otherwise invisible without opening ACTIVITY.md (§18.6, §20).
//
// attends, not a kind filter, is bound's extra predicate here: LatestOf
// filters by kind alone and would report a no-attention note as the winner
// merely for being newest, which is the exact bug the two-lens review found
// in an earlier version of this comparison (see bounds_test.go).
func AttentionEvent(dir string, created time.Time) (Event, bool, error) {
	e, ok, err := bound(dir, []Kind{KindNote, KindMeasurement}, attends, func(candidate, best Event) bool {
		return candidate.At.After(best.At)
	})
	if err != nil {
		return Event{}, false, err
	}
	if !ok || created.After(e.At) {
		// The same guard Attention takes: a hand-edited `created` later than
		// every event must not read as attention in the past.
		return Event{}, false, nil
	}
	return e, true, nil
}
