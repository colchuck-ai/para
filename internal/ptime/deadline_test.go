package ptime_test

import (
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/ptime"
)

// TestDeadline pins the reading Phase 8 recorded and Phase 10 depends on: a
// bare `due` date is the whole day, and a `due` carrying a time of day is that
// instant exactly.
func TestDeadline(t *testing.T) {
	cases := []struct {
		due  string
		want string
	}{
		// The last instant the 30th still admits, not the midnight that opens it.
		{"2026-09-30", "2026-09-30T23:59:59Z"},
		{"2026-09-30T17", "2026-09-30T17:00:00Z"},
		{"2026-09-30T17:30", "2026-09-30T17:30:00Z"},
		{"2026-09-30T17:30:15", "2026-09-30T17:30:15Z"},
		// A stored offset is honoured: the deadline is an instant, and the
		// typist's zone is what decided which one.
		{"2026-09-30T17:00:00-07:00", "2026-10-01T00:00:00Z"},
		// A month boundary is arithmetic, not string surgery.
		{"2026-02-28", "2026-02-28T23:59:59Z"},
		{"2026-12-31", "2026-12-31T23:59:59Z"},
	}
	for _, tc := range cases {
		got, err := ptime.Deadline(tc.due)
		if err != nil {
			t.Errorf("Deadline(%q): %v", tc.due, err)
			continue
		}
		if want, _ := time.Parse(time.RFC3339, tc.want); !got.Equal(want) {
			t.Errorf("Deadline(%q): got %s, want %s", tc.due, got.Format(time.RFC3339), tc.want)
		}
	}
}

func TestDeadlineRejectsWhatParseAtRejects(t *testing.T) {
	for _, due := range []string{"", "tomorrow", "2026-13-01", "30/09/2026"} {
		if _, err := ptime.Deadline(due); err == nil {
			t.Errorf("Deadline(%q): want an error", due)
		}
	}
}

// TestDeadlineOnTheDueDayIsNotPast is the consequence the whole-day reading
// exists for: something due today is not overdue until today is over.
func TestDeadlineOnTheDueDayIsNotPast(t *testing.T) {
	deadline, err := ptime.Deadline("2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	for _, now := range []string{"2026-09-30T00:00:00Z", "2026-09-30T23:59:59Z"} {
		at, _ := time.Parse(time.RFC3339, now)
		if at.After(deadline) {
			t.Errorf("%s reads as past a deadline of 2026-09-30", now)
		}
	}
	at, _ := time.Parse(time.RFC3339, "2026-10-01T00:00:00Z")
	if !at.After(deadline) {
		t.Error("2026-10-01T00:00:00Z should be past a deadline of 2026-09-30")
	}
}
