package journal

import (
	"path/filepath"
	"sort"
	"time"
)

// ReadOnOrAfter reads the events in dir dated at or after since, opening only
// the journal files that can hold them, and reports how many files it opened.
//
// It is what makes §3.5's cheap write cheap: "only today's section is
// re-derived on a mutation ... so a write reads one journal file, not all of
// them". The file count is returned rather than inferred so that guarantee can
// be asserted by a test rather than assumed.
//
// The scan walks files newest-first and stops after the first file that
// contributes no qualifying event, so an unrotated journal — one file, which
// is every entity until it accumulates 4 MiB of events (§3.4) — costs exactly
// one open however long its history. Past a rotation it costs one file beyond
// the last qualifying one: the file it must open to learn it can stop.
//
// That stopping rule is sound for events written when they happened, since a
// new event always lands in the newest file and the qualifying events
// therefore occupy a contiguous run of the newest files. It has one contrived
// hole: enough backdated events (§15.1's `--at`) to fill an entire rotation
// would leave a file of exclusively old events sitting between two files of
// new ones, ending the scan early. That degrades to a stale prior-day section
// in ACTIVITY.md — precisely the drift doctor's full-fidelity check exists to
// catch and rebuild to repair (§3.5, §10) — so it is a known cost of the cheap
// write rather than a hole in the model.
func ReadOnOrAfter(dir string, since time.Time) (events []Event, filesRead int, err error) {
	names, err := listJSONL(dir)
	if err != nil {
		return nil, 0, err
	}

	// Collected newest-file-first, because that is the direction the scan has
	// to walk to stop early, then replayed oldest-file-first below.
	var blocks [][]Event
	for i := len(names) - 1; i >= 0; i-- {
		// Counted before the read, not after: the number reported is how many
		// files were opened, which is the claim §3.5 makes, and a caller
		// asserting it on the failure path needs the failing file included.
		filesRead++
		fileEvents, err := readFile(filepath.Join(dir, names[i]))
		if err != nil {
			return nil, filesRead, err
		}

		var matched []Event
		for _, e := range fileEvents {
			if !e.At.Before(since) {
				matched = append(matched, e)
			}
		}
		blocks = append(blocks, matched)

		// Stop when a file that *had* events contributed none. A file with no
		// events at all says nothing about the files before it — an empty
		// newest file, which a rotation whose first write failed can leave
		// behind, would otherwise end the scan and hide every qualifying
		// event.
		if len(fileEvents) > 0 && len(matched) == 0 {
			break
		}
	}

	// Oldest file first, so that events sharing an instant across two files
	// end up in the same relative order ReadAll produces. Both readers then
	// stable-sort, so both return byte-identical histories — which is what
	// keeps ACTIVITY.md's incremental mode (this reader) agreeing with its
	// full mode (ReadAll) for a tie that straddles a rotation.
	for i := len(blocks) - 1; i >= 0; i-- {
		events = append(events, blocks[i]...)
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].At.Before(events[j].At)
	})
	return events, filesRead, nil
}
