package ptime

import (
	"sort"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

// TestParseAt covers every progressive-precision form in §15.1, each
// zero-filled in the given location, plus the full-RFC-3339 form that
// carries its own offset instead.
func TestParseAt(t *testing.T) {
	pt := mustLoc(t, "America/Los_Angeles")

	cases := []struct {
		name string
		in   string
		want time.Time
	}{
		{
			name: "bare date is midnight local",
			in:   "2026-01-03",
			want: time.Date(2026, 1, 3, 0, 0, 0, 0, pt),
		},
		{
			name: "+hour zero-fills minute and second",
			in:   "2026-01-03T09",
			want: time.Date(2026, 1, 3, 9, 0, 0, 0, pt),
		},
		{
			name: "+minute zero-fills second",
			in:   "2026-01-03T09:02",
			want: time.Date(2026, 1, 3, 9, 2, 0, 0, pt),
		},
		{
			name: "+second is exact",
			in:   "2026-01-03T09:02:11",
			want: time.Date(2026, 1, 3, 9, 2, 11, 0, pt),
		},
		{
			name: "full RFC 3339 carries its own offset",
			in:   "2026-01-03T09:02:11-08:00",
			want: time.Date(2026, 1, 3, 9, 2, 11, 0, time.FixedZone("-08:00", -8*3600)),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseAt(c.in, pt)
			if err != nil {
				t.Fatalf("ParseAt(%q): %v", c.in, err)
			}
			if !got.Equal(c.want) {
				t.Errorf("ParseAt(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseAt_Invalid(t *testing.T) {
	pt := mustLoc(t, "America/Los_Angeles")
	cases := []string{
		"",
		"not-a-date",
		"2026-13-01",
		"2026-01-03 09:02",
		"2026/01/03",
	}
	for _, in := range cases {
		if _, err := ParseAt(in, pt); err == nil {
			t.Errorf("ParseAt(%q): want error, got nil", in)
		}
	}
}

func TestCheckNotFuture(t *testing.T) {
	now := time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC)

	if err := CheckNotFuture(now.Add(-time.Hour), now); err != nil {
		t.Errorf("past instant rejected: %v", err)
	}
	if err := CheckNotFuture(now, now); err != nil {
		t.Errorf("exact now rejected: %v", err)
	}
	if err := CheckNotFuture(now.Add(time.Hour), now); err == nil {
		t.Error("future instant accepted, want error")
	}
}

func TestCheckNotAfterDue(t *testing.T) {
	due := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)

	if err := CheckNotAfterDue(due.Add(-time.Hour), due); err != nil {
		t.Errorf("instant before due rejected: %v", err)
	}
	if err := CheckNotAfterDue(due, due); err != nil {
		t.Errorf("instant equal to due rejected: %v", err)
	}
	if err := CheckNotAfterDue(due.Add(time.Hour), due); err == nil {
		t.Error("instant after due accepted, want error")
	}
}

// TestEqual proves the exact-instant rule of §3.1: the same instant
// recorded in two different offsets is one collision, not two distinct
// measurements.
func TestEqual(t *testing.T) {
	utc := time.Date(2026, 1, 3, 17, 2, 11, 0, time.UTC)
	pt := utc.In(mustLoc(t, "America/Los_Angeles"))

	if !Equal(utc, pt) {
		t.Error("same instant in different offsets: want Equal, got not equal")
	}
	if Equal(utc, utc.Add(time.Second)) {
		t.Error("distinct instants one second apart: want not equal, got Equal")
	}
}

func TestJournalFilename(t *testing.T) {
	at := time.Date(2026, 1, 1, 8, 8, 1, 0, time.UTC)
	got := JournalFilename(at)
	want := "20260101T080801.jsonl"
	if got != want {
		t.Errorf("JournalFilename(%v) = %q, want %q", at, got, want)
	}
}

// TestJournalFilename_LexicalOrderMatchesChronology pins §3.4's claim that
// "the newest file is the last one lexically."
func TestJournalFilename_LexicalOrderMatchesChronology(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var names []string
	for i := 0; i < 5; i++ {
		names = append(names, JournalFilename(base.Add(time.Duration(i)*time.Hour)))
	}

	sorted := append([]string(nil), names...)
	sort.Strings(sorted)

	for i := range names {
		if names[i] != sorted[i] {
			t.Fatalf("filenames not already in lexical/chronological order: %v", names)
		}
	}
}
