package journal

import "path/filepath"

// ActiveSuppression is the events-in-hand fold for §28.2's rule: the newest
// suppress event by `at`, read only when its `until` is non-empty. An
// unsuppress — the same kind, with `until` empty — folds to "no active
// suppression" even when it is the newest event, since that is exactly what
// unsuppress means; there is no second kind and no delete-style event to
// look for.
//
// It is Attention's twin for suppression, kept apart from LatestOf's own
// disk read for the same reason Attention is kept apart from AttentionAt
// (see attention.go): the write-through step (§28.4) must read the journal
// as it will look immediately after this mutation's own new event lands,
// before anything is written to disk, and a slice already in hand is the
// only way to ask that question.
//
// A tie in `at` is broken by keeping the LAST event encountered, the
// opposite of bound()'s shared tie-break (bounds.go). That is deliberate:
// suppress and unsuppress take no --at, so a same-second suppress
// immediately followed by an unsuppress is an ordinary, legal sequence —
// not the exact-instant collision measure's own uniqueness check exists to
// refuse — and this fold's whole job is to say which of two
// administratively sequential events is the one currently in force. The
// slice's own order (a mutation's prior events, from disk, followed by the
// event it is about to add) is already write order even when `at` ties, so
// the last one encountered is the one that actually happened last.
func ActiveSuppression(events []Event) (until, note string) {
	var best Event
	found := false
	for _, e := range events {
		if e.Kind != KindSuppress {
			continue
		}
		if !found || !e.At.Before(best.At) {
			best, found = e, true
		}
	}
	if !found || best.Value == "" {
		return "", ""
	}
	return best.Value, best.Note
}

// ActiveSuppressionAt is ActiveSuppression asked of a journal on disk: the
// same "empty until means cleared" reading, so doctor's stale-projection
// check and rebuild share this one interpretation rather than each deciding
// for itself what an unsuppress at the top of the fold means.
//
// It does not go through LatestOf (bounds.go): LatestOf shares bound()'s one
// tie-break with every other caller of it, and this fold needs the opposite
// one, for the reason ActiveSuppression's own comment gives. A forward walk
// over listJSONL's lexically-sorted names, then each file's events in the
// order readFile returns them, is write order — §3.4 rotation only ever
// opens a new file forward in time, and a file's own lines are in append
// order — so keeping the last match encountered here is keeping the one
// that was actually appended last, tie or not.
func ActiveSuppressionAt(dir string) (until, note string, err error) {
	names, err := listJSONL(dir)
	if err != nil {
		return "", "", err
	}
	var best Event
	found := false
	for _, name := range names {
		events, err := readFile(filepath.Join(dir, name))
		if err != nil {
			return "", "", err
		}
		for _, e := range events {
			if e.Kind != KindSuppress {
				continue
			}
			if !found || !e.At.Before(best.At) {
				best, found = e, true
			}
		}
	}
	if !found || best.Value == "" {
		return "", "", nil
	}
	return best.Value, best.Note, nil
}
