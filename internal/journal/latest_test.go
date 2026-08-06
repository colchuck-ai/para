package journal_test

import (
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
)

func TestLatestAtReadsOnlyTheNewestFileThatHasAMatch(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-01T00:00:00Z"), "old note"))
	writeJournalFile(t, dir, "20260201T000000Z.jsonl",
		journal.NewNote(at(t, "2026-02-01T00:00:00Z"), "newer note"))
	writeJournalFile(t, dir, "20260301T000000Z.jsonl",
		journal.NewChange(at(t, "2026-03-01T00:00:00Z"), "status", "planned", "done", ""))

	got, ok, err := journal.LatestAt(dir, journal.KindNote, journal.KindMeasurement)
	if err != nil {
		t.Fatalf("LatestAt: %v", err)
	}
	if !ok {
		t.Fatal("LatestAt found nothing")
	}
	// The newest file holds only a change, which never counts (§3.6), so the
	// answer comes from the file before it — and not from the one before that.
	if want := at(t, "2026-02-01T00:00:00Z"); !got.Equal(want) {
		t.Errorf("LatestAt: got %s, want %s", got, want)
	}
}

func TestLatestAtOrdersByAtNotByFilePosition(t *testing.T) {
	dir := t.TempDir()
	// A backdated --at lands an earlier event after a later one in one file.
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-20T00:00:00Z"), "later"),
		journal.NewNote(at(t, "2026-01-05T00:00:00Z"), "backdated"))

	got, ok, err := journal.LatestAt(dir, journal.KindNote)
	if err != nil || !ok {
		t.Fatalf("LatestAt: %v ok=%v", err, ok)
	}
	if want := at(t, "2026-01-20T00:00:00Z"); !got.Equal(want) {
		t.Errorf("LatestAt: got %s, want %s", got, want)
	}
}

// TestLatestAtSeesPastABackdatedEventInANewerFile is the case that makes
// "stop at the newest file holding a match" unsound. `--at` is only bounded
// above (§15.1's not-future rule), and an append always lands in the newest
// file (§3.4) — so a note backdated after a rotation puts an *older* `at` in a
// *newer* file, and the real newest note is in the file before it.
func TestLatestAtSeesPastABackdatedEventInANewerFile(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260120T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-20T00:00:00Z"), "the real newest"))
	writeJournalFile(t, dir, "20260105T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-05T00:00:00Z"), "backdated, written later"))

	got, ok, err := journal.LatestAt(dir, journal.KindNote)
	if err != nil || !ok {
		t.Fatalf("LatestAt: %v ok=%v", err, ok)
	}
	if want := at(t, "2026-01-20T00:00:00Z"); !got.Equal(want) {
		t.Errorf("LatestAt: got %s, want %s — ordering is by `at`, never by file (§3.1)", got, want)
	}
}

func TestLatestAtOnAJournalWithNoMatchFindsNothing(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewChild(at(t, "2026-01-01T00:00:00Z"), journal.ChildOpAdded, "q1", "", "", ""))

	_, ok, err := journal.LatestAt(dir, journal.KindNote, journal.KindMeasurement)
	if err != nil {
		t.Fatalf("LatestAt: %v", err)
	}
	if ok {
		t.Error("a journal of child events has no attention of its own (§3.6)")
	}
}

func TestLatestAtOnAnAbsentDirectoryFindsNothing(t *testing.T) {
	_, ok, err := journal.LatestAt(t.TempDir()+"/never", journal.KindNote)
	if err != nil {
		t.Fatalf("LatestAt: %v", err)
	}
	if ok {
		t.Error("an absent logs/ is an empty journal")
	}
}

func TestAttentionAtAgreesWithAttentionOverTheWholeJournal(t *testing.T) {
	created := at(t, "2026-01-01T00:00:00Z")
	events := []journal.Event{
		journal.NewNote(at(t, "2026-01-05T00:00:00Z"), "one"),
		journal.NewMeasurement(at(t, "2026-02-09T00:00:00Z"), "3/4", ""),
		journal.NewChange(at(t, "2026-03-01T00:00:00Z"), "due", "", "2026-12-31", ""),
	}

	dir := t.TempDir()
	for _, e := range events {
		if _, err := journal.Append(dir, e, journal.DefaultRotateBytes); err != nil {
			t.Fatal(err)
		}
	}

	got, err := journal.AttentionAt(dir, created)
	if err != nil {
		t.Fatalf("AttentionAt: %v", err)
	}
	// The cheap read and the whole-journal fold are the same answer, which is
	// what makes substituting one for the other safe.
	if want := journal.Attention(events, created); !got.Equal(want) {
		t.Errorf("AttentionAt: got %s, want %s", got, want)
	}
}

func TestAttentionAtFallsBackToCreated(t *testing.T) {
	created := at(t, "2026-01-01T00:00:00Z")
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewChild(created, journal.ChildOpAdded, "q1", "", "", ""))

	got, err := journal.AttentionAt(dir, created)
	if err != nil {
		t.Fatalf("AttentionAt: %v", err)
	}
	if !got.Equal(created) {
		t.Errorf("AttentionAt: got %s, want created %s", got, created)
	}
}

func TestAttentionAtNeverPrecedesCreated(t *testing.T) {
	created := at(t, "2026-06-01T00:00:00Z")
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-05T00:00:00Z"), "before it existed"))

	got, err := journal.AttentionAt(dir, created)
	if err != nil {
		t.Fatalf("AttentionAt: %v", err)
	}
	// Same guard Attention takes: a hand-edited created later than every event
	// must not read as attention in the past.
	if !got.Equal(created) {
		t.Errorf("AttentionAt: got %s, want created %s", got, created)
	}
}
