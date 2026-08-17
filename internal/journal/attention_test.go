package journal

import "testing"

// TestAttention walks every event kind against the clock rule (§3.6): note
// and measurement move it, change and child never do, and an entity with no
// qualifying events falls back to created.
func TestAttention(t *testing.T) {
	created := mustParseAt(t, "2026-01-01T00:00:00-08:00")
	noteAt := mustParseAt(t, "2026-01-04T17:40:00-08:00")
	measurementAt := mustParseAt(t, "2026-01-03T09:02:11-08:00")
	changeAt := mustParseAt(t, "2026-06-01T00:00:00-08:00")
	childAt := mustParseAt(t, "2026-07-01T00:00:00-08:00")

	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{name: "no events falls back to created", events: nil, want: "created"},
		{
			name:   "note moves it",
			events: []Event{NewNote(noteAt, "waiting")},
			want:   "note",
		},
		{
			name:   "measurement moves it",
			events: []Event{NewMeasurement(measurementAt, "880/11000", "")},
			want:   "measurement",
		},
		{
			name: "newest of note and measurement wins",
			events: []Event{
				NewMeasurement(measurementAt, "880/11000", ""),
				NewNote(noteAt, "waiting"),
			},
			want: "note",
		},
		{
			name:   "change never moves it, even a status change",
			events: []Event{NewChange(changeAt, "status", "in-progress", "blocked", "")},
			want:   "created",
		},
		{
			name:   "child never moves it",
			events: []Event{NewChild(childAt, ChildOpAdded, "q1-growth", "", "", "")},
			want:   "created",
		},
		{
			name: "change and child after a note still lose to the note",
			events: []Event{
				NewNote(noteAt, "waiting"),
				NewChange(changeAt, "status", "in-progress", "blocked", ""),
				NewChild(childAt, ChildOpAdded, "q1-growth", "", "", ""),
			},
			want: "note",
		},
		{
			name:   "a no-attention note never moves it",
			events: []Event{NewNoteNoAttention(noteAt, "retired the old beads IDs")},
			want:   "created",
		},
		{
			name: "a no-attention note loses to an older measurement that does count",
			events: []Event{
				NewMeasurement(measurementAt, "880/11000", ""),
				NewNoteNoAttention(noteAt, "retired the old beads IDs"),
			},
			want: "measurement",
		},
		{
			name: "an ordinary note after a no-attention one still moves it",
			events: []Event{
				NewNoteNoAttention(measurementAt, "retired the old beads IDs"),
				NewNote(noteAt, "actually wrote an entry"),
			},
			want: "note",
		},
	}

	instants := map[string]string{
		"created":     created.Format("2006-01-02T15:04:05"),
		"note":        noteAt.Format("2006-01-02T15:04:05"),
		"measurement": measurementAt.Format("2006-01-02T15:04:05"),
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Attention(c.events, created)
			if got.Format("2006-01-02T15:04:05") != instants[c.want] {
				t.Errorf("Attention() = %v, want the %s instant (%s)", got, c.want, instants[c.want])
			}
		})
	}
}

// TestAttention_ContainerAlwaysCreated: a container's journal carries only
// child events, so its attention is always created (§3.6) — nothing reads
// it, but the rule must not special-case containers to reach that answer.
func TestAttention_ContainerAlwaysCreated(t *testing.T) {
	created := mustParseAt(t, "2026-01-01T00:00:00-08:00")
	events := []Event{
		NewChild(mustParseAt(t, "2026-01-05T00:00:00-08:00"), ChildOpAdded, "acme-migration", "", "", ""),
		NewChild(mustParseAt(t, "2026-02-01T00:00:00-08:00"), ChildOpRemoved, "old-project", "", "", ""),
	}

	got := Attention(events, created)
	if !got.Equal(created) {
		t.Errorf("Attention() = %v, want created %v", got, created)
	}
}
