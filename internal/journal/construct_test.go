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

func TestNewNoteNoAttention(t *testing.T) {
	at := mustParseAt(t, "2026-01-04T17:40:00-08:00")
	got := NewNoteNoAttention(at, "retired the old beads IDs from the ported entries")
	want := Event{At: at, Kind: KindNote, Note: "retired the old beads IDs from the ported entries", NoAttention: true}
	if got != want {
		t.Errorf("NewNoteNoAttention() = %+v, want %+v", got, want)
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

func TestNewSuppress(t *testing.T) {
	at := mustParseAt(t, "2026-08-20T10:00:00-07:00")
	got := NewSuppress(at, "2027-03-01", "paused, resumes with Q1 relaunch")
	want := Event{At: at, Kind: KindSuppress, Value: "2027-03-01", Note: "paused, resumes with Q1 relaunch"}
	if got != want {
		t.Errorf("NewSuppress() = %+v, want %+v", got, want)
	}
}

// TestNewSuppress_EmptyUntilIsUnsuppress pins §28.2's "no second kind": para
// unsuppress calls NewSuppress with until == "", and that is the whole
// distinction — same kind, empty Value.
func TestNewSuppress_EmptyUntilIsUnsuppress(t *testing.T) {
	at := mustParseAt(t, "2026-09-01T09:00:00-07:00")
	got := NewSuppress(at, "", "relaunch moved up")
	want := Event{At: at, Kind: KindSuppress, Note: "relaunch moved up"}
	if got != want {
		t.Errorf("NewSuppress() = %+v, want %+v", got, want)
	}
	if got.Value != "" {
		t.Errorf("NewSuppress with until=\"\" has Value %q, want empty", got.Value)
	}
}
