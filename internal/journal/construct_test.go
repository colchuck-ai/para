package journal

import "testing"

func TestNewChange(t *testing.T) {
	at := mustParseAt(t, "2026-01-01T08:15:02-08:00")
	got := NewChange(at, "status", "planned", "in-progress", "kickoff done")
	want := Event{At: at, Kind: KindChange, Field: "status", From: "planned", To: "in-progress", Note: "kickoff done"}
	if got != want {
		t.Errorf("NewChange() = %+v, want %+v", got, want)
	}
}

func TestNewMeasurement(t *testing.T) {
	at := mustParseAt(t, "2026-01-03T09:02:11-08:00")
	got := NewMeasurement(at, "880/11000", "")
	want := Event{At: at, Kind: KindMeasurement, Value: "880/11000"}
	if got != want {
		t.Errorf("NewMeasurement() = %+v, want %+v", got, want)
	}
}

func TestNewNote(t *testing.T) {
	at := mustParseAt(t, "2026-01-04T17:40:00-08:00")
	got := NewNote(at, "waiting on the ingest team")
	want := Event{At: at, Kind: KindNote, Note: "waiting on the ingest team"}
	if got != want {
		t.Errorf("NewNote() = %+v, want %+v", got, want)
	}
}

func TestNewChild(t *testing.T) {
	at := mustParseAt(t, "2026-01-05T11:00:00-08:00")
	got := NewChild(at, ChildOpAdded, "q1-growth", "", "", "")
	want := Event{At: at, Kind: KindChild, Op: ChildOpAdded, Child: "q1-growth"}
	if got != want {
		t.Errorf("NewChild() = %+v, want %+v", got, want)
	}
}

// TestNewChild_Moved pins the from/to locator pair a move carries (§3.1).
func TestNewChild_Moved(t *testing.T) {
	at := mustParseAt(t, "2026-02-01T09:00:00-08:00")
	got := NewChild(at, ChildOpMoved, "training", "areas.health.training", "areas.fitness.training", "reorganized")
	want := Event{
		At: at, Kind: KindChild, Op: ChildOpMoved, Child: "training",
		From: "areas.health.training", To: "areas.fitness.training", Note: "reorganized",
	}
	if got != want {
		t.Errorf("NewChild() = %+v, want %+v", got, want)
	}
}
