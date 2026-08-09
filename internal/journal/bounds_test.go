package journal_test

import (
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
)

// TestLatestOfAndOldestOfSpanRotation: §4.1's baseline is the oldest reading
// and §4.2's current is the newest, so a key-result whose history has rotated
// needs both ends of it — and only both ends.
func TestLatestOfAndOldestOfSpanRotation(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewMeasurement(at(t, "2026-01-01T00:00:00Z"), "480/9000", ""),
		journal.NewNote(at(t, "2026-01-02T00:00:00Z"), "a note"))
	writeJournalFile(t, dir, "20260201T000000Z.jsonl",
		journal.NewMeasurement(at(t, "2026-02-01T00:00:00Z"), "700/10000", ""))
	writeJournalFile(t, dir, "20260301T000000Z.jsonl",
		journal.NewMeasurement(at(t, "2026-03-01T00:00:00Z"), "880/11000", ""))

	newest, ok, err := journal.LatestOf(dir, journal.KindMeasurement)
	if err != nil || !ok {
		t.Fatalf("LatestOf: %v ok=%v", err, ok)
	}
	if newest.Value != "880/11000" {
		t.Errorf("LatestOf: got %q, want 880/11000", newest.Value)
	}

	oldest, ok, err := journal.OldestOf(dir, journal.KindMeasurement)
	if err != nil || !ok {
		t.Fatalf("OldestOf: %v ok=%v", err, ok)
	}
	if oldest.Value != "480/9000" {
		t.Errorf("OldestOf: got %q, want 480/9000", oldest.Value)
	}
}

// TestBoundsOrderByAtNotByFilePosition: a backdated --at can land an earlier
// event after a later one, in a later file, so neither end may be read off a
// position (§3.1).
func TestBoundsOrderByAtNotByFilePosition(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260201T000000Z.jsonl",
		journal.NewMeasurement(at(t, "2026-02-01T00:00:00Z"), "middle", ""))
	writeJournalFile(t, dir, "20260301T000000Z.jsonl",
		journal.NewMeasurement(at(t, "2026-03-01T00:00:00Z"), "newest", ""),
		journal.NewMeasurement(at(t, "2026-01-01T00:00:00Z"), "backdated-oldest", ""))

	newest, _, err := journal.LatestOf(dir, journal.KindMeasurement)
	if err != nil {
		t.Fatal(err)
	}
	if newest.Value != "newest" {
		t.Errorf("LatestOf: got %q, want newest", newest.Value)
	}

	// The oldest event by `at` sits in the *last* file, so a forward scan that
	// stopped at the first file holding any match would answer "middle".
	oldest, _, err := journal.OldestOf(dir, journal.KindMeasurement)
	if err != nil {
		t.Fatal(err)
	}
	if oldest.Value != "backdated-oldest" {
		t.Errorf("OldestOf: got %q, want backdated-oldest", oldest.Value)
	}
}

func TestBoundsFindNothingInAJournalWithNoMatch(t *testing.T) {
	dir := t.TempDir()
	writeJournalFile(t, dir, "20260101T000000Z.jsonl",
		journal.NewNote(at(t, "2026-01-01T00:00:00Z"), "no readings here"))

	if _, ok, err := journal.LatestOf(dir, journal.KindMeasurement); err != nil || ok {
		t.Errorf("LatestOf: ok=%v err=%v, want false/nil", ok, err)
	}
	if _, ok, err := journal.OldestOf(dir, journal.KindMeasurement); err != nil || ok {
		t.Errorf("OldestOf: ok=%v err=%v, want false/nil", ok, err)
	}
}

func TestBoundsOnAnAbsentDirectory(t *testing.T) {
	dir := t.TempDir() + "/never"
	if _, ok, err := journal.LatestOf(dir, journal.KindNote); err != nil || ok {
		t.Errorf("LatestOf: ok=%v err=%v", ok, err)
	}
	if _, ok, err := journal.OldestOf(dir, journal.KindNote); err != nil || ok {
		t.Errorf("OldestOf: ok=%v err=%v", ok, err)
	}
}

// TestBoundsAgreeWithReadAll is the property that makes substituting the cheap
// reads for a whole-journal fold safe.
func TestBoundsAgreeWithReadAll(t *testing.T) {
	dir := t.TempDir()
	for _, e := range []journal.Event{
		journal.NewMeasurement(at(t, "2026-02-01T00:00:00Z"), "b", ""),
		journal.NewMeasurement(at(t, "2026-01-01T00:00:00Z"), "a", ""),
		journal.NewChange(at(t, "2026-04-01T00:00:00Z"), "status", "", "done", ""),
		journal.NewMeasurement(at(t, "2026-03-01T00:00:00Z"), "c", ""),
	} {
		if _, err := journal.Append(dir, e, journal.DefaultRotateBytes); err != nil {
			t.Fatal(err)
		}
	}

	all, err := journal.ReadAll(dir)
	if err != nil {
		t.Fatal(err)
	}
	var measurements []journal.Event
	for _, e := range all {
		if e.Kind == journal.KindMeasurement {
			measurements = append(measurements, e)
		}
	}

	newest, _, _ := journal.LatestOf(dir, journal.KindMeasurement)
	oldest, _, _ := journal.OldestOf(dir, journal.KindMeasurement)
	if want := measurements[len(measurements)-1]; !newest.At.Equal(want.At) || newest.Value != want.Value {
		t.Errorf("LatestOf: got %v, want %v", newest, want)
	}
	if want := measurements[0]; !oldest.At.Equal(want.At) || oldest.Value != want.Value {
		t.Errorf("OldestOf: got %v, want %v", oldest, want)
	}
}
