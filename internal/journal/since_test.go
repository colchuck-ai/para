package journal_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/journal"
)

func writeJournalFile(t *testing.T, dir, name string, events ...journal.Event) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for _, e := range events {
		line, err := journal.Encode(e)
		if err != nil {
			t.Fatal(err)
		}
		data = append(data, line...)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func at(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts
}

func TestReadOnOrAfterReturnsOnlyEventsAtOrAfterSince(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T080000.jsonl",
		journal.NewNote(at(t, "2026-01-01T08:00:00Z"), "first"),
		journal.NewNote(at(t, "2026-01-05T08:00:00Z"), "second"),
	)

	events, files, err := journal.ReadOnOrAfter(dir, at(t, "2026-01-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 1 || events[0].Note != "second" {
		t.Fatalf("events = %+v, want just the 2026-01-05 note", events)
	}
	if files != 1 {
		t.Errorf("files read = %d, want 1", files)
	}
}

func TestReadOnOrAfterReadsOneFileForAnUnrotatedJournal(t *testing.T) {
	// §3.5's headline claim — "a write reads one journal file, not all of
	// them" — is a claim about file opens, so it is asserted by counting
	// them. An entity that has not yet rotated has exactly one file, and
	// re-deriving today's section must open only it however long its history.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T080000.jsonl",
		journal.NewNote(at(t, "2026-01-01T08:00:00Z"), "january"),
		journal.NewNote(at(t, "2026-02-01T08:00:00Z"), "february"),
		journal.NewNote(at(t, "2026-03-05T08:00:00Z"), "today"),
	)

	events, files, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 1 || events[0].Note != "today" {
		t.Fatalf("events = %+v, want just today's note", events)
	}
	if files != 1 {
		t.Errorf("files read = %d, want 1", files)
	}
}

func TestReadOnOrAfterStopsAtTheFirstFileThatContributesNothing(t *testing.T) {
	// With rotations behind it, the scan pays for one file beyond the last
	// qualifying one — the file it has to open to learn it can stop. It never
	// pays for the ones before that.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T080000.jsonl", journal.NewNote(at(t, "2026-01-01T08:00:00Z"), "oldest"))
	writeJournalFile(t, dir, "20260201T080000.jsonl", journal.NewNote(at(t, "2026-02-01T08:00:00Z"), "middle"))
	writeJournalFile(t, dir, "20260301T080000.jsonl", journal.NewNote(at(t, "2026-03-01T08:00:00Z"), "newest"))

	events, files, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-01T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 1 || events[0].Note != "newest" {
		t.Fatalf("events = %+v, want just the newest note", events)
	}
	if files != 2 {
		t.Errorf("files read = %d, want 2 of 3", files)
	}
}

func TestReadOnOrAfterSpansASameDayRotation(t *testing.T) {
	// A rotation partway through a day leaves that day's events split across
	// two files. Reading only the newest would silently drop the earlier
	// half, so the scan continues backwards until a file contributes nothing.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260305T080000.jsonl",
		journal.NewNote(at(t, "2026-03-05T08:00:00Z"), "morning"),
	)
	writeJournalFile(t, dir, "20260305T170000.jsonl",
		journal.NewNote(at(t, "2026-03-05T17:00:00Z"), "evening"),
	)

	events, files, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %+v, want both of the day's notes", events)
	}
	if events[0].Note != "morning" || events[1].Note != "evening" {
		t.Errorf("events = %+v, want morning then evening", events)
	}
	if files != 2 {
		t.Errorf("files read = %d, want 2", files)
	}
}

func TestReadOnOrAfterOrdersByAtNotByFilePosition(t *testing.T) {
	// §3.1: ordering comes from `at`, never from file position — which a
	// backdated --at makes observable, since the new event lands in the
	// newest file with an older timestamp.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260305T080000.jsonl",
		journal.NewNote(at(t, "2026-03-05T08:00:00Z"), "written first"),
		journal.NewNote(at(t, "2026-03-05T06:00:00Z"), "backdated"),
	)

	events, _, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 2 || events[0].Note != "backdated" {
		t.Errorf("events = %+v, want the backdated note first", events)
	}
}

func TestReadOnOrAfterOnAnAbsentDirectoryIsEmpty(t *testing.T) {
	events, files, err := journal.ReadOnOrAfter(filepath.Join(t.TempDir(), "nope"), time.Now())
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 0 || files != 0 {
		t.Errorf("got %d events from %d files, want none", len(events), files)
	}
}

func TestReadOnOrAfterAgreesWithReadAllOnTieOrder(t *testing.T) {
	// Two events at the identical instant in two different files. ACTIVITY.md's
	// incremental mode reads through ReadOnOrAfter and its full mode through
	// ReadAll, so if the two readers order a tie differently the same history
	// renders as different bytes — drift no rebuild could settle, because the
	// two sides never converge. A batch of events from one clock can straddle a
	// rotation, so this is reachable, not hypothetical.
	dir := t.TempDir()
	tie := at(t, "2026-03-05T09:00:00Z")
	writeJournalFile(t, dir, "20260301T080000.jsonl",
		journal.NewNote(at(t, "2026-03-01T08:00:00Z"), "older file, earlier event"),
		journal.NewChange(tie, "status", "planned", "active", ""),
	)
	writeJournalFile(t, dir, "20260305T090000.jsonl",
		journal.NewChild(tie, journal.ChildOpAdded, "signups", "", "", ""),
	)

	all, err := journal.ReadAll(dir)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	since, _, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}

	// Compare the tail of ReadAll's history — the part at or after `since` —
	// against everything ReadOnOrAfter returned.
	tail := all[len(all)-len(since):]
	for i := range since {
		if since[i] != tail[i] {
			t.Fatalf("reader disagreement at %d:\n  ReadOnOrAfter %+v\n  ReadAll       %+v", i, since[i], tail[i])
		}
	}
}

func TestReadOnOrAfterSkipsPastAnEmptyNewestFile(t *testing.T) {
	// A rotation whose O_CREATE succeeded but whose write failed leaves an
	// empty file behind. Treating "no events" as "nothing qualifies" would end
	// the scan there and report an empty history.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260305T080000.jsonl", journal.NewNote(at(t, "2026-03-05T08:00:00Z"), "real"))
	writeJournalFile(t, dir, "20260305T170000.jsonl")

	events, _, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err != nil {
		t.Fatalf("ReadOnOrAfter: %v", err)
	}
	if len(events) != 1 || events[0].Note != "real" {
		t.Errorf("events = %+v, want the one real note", events)
	}
}

func TestReadOnOrAfterReportsFilesReadEvenOnError(t *testing.T) {
	// The file count is the evidence for §3.5's one-open guarantee, so a caller
	// asserting it on the failure path must get a real number.
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260305T080000.jsonl", journal.NewNote(at(t, "2026-03-05T08:00:00Z"), "fine"))
	if err := os.WriteFile(filepath.Join(dir, "20260305T170000.jsonl"), []byte("{not json\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, files, err := journal.ReadOnOrAfter(dir, at(t, "2026-03-05T00:00:00Z"))
	if err == nil {
		t.Fatal("ReadOnOrAfter accepted a corrupt journal line")
	}
	if files != 1 {
		t.Errorf("files read = %d, want 1", files)
	}
}
