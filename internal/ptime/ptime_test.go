package ptime

import (
	"sort"
	"strings"
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
		{
			name: "full RFC 3339 accepts the Z (UTC) spelling of an offset",
			in:   "2026-01-03T09:02:11Z",
			want: time.Date(2026, 1, 3, 9, 2, 11, 0, time.UTC),
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
		// A full date is the minimum precision §15.1 grants `--at` — the
		// coarser year and year-month forms are `due`'s alone (ptime.Deadline,
		// para-xbb), because "by the end of April" only makes sense for a
		// deadline, not for when something happened.
		"2026",
		"2026-01",
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
	want := "20260101T080801Z.jsonl"
	if got != want {
		t.Errorf("JournalFilename(%v) = %q, want %q", at, got, want)
	}
}

func TestJournalFilenameIsUTCWhateverTheEventsOwnOffset(t *testing.T) {
	// The same instant, written from three zones, must produce one name —
	// otherwise the filename records where the writer stood rather than when
	// the event happened, and lexical order stops meaning chronological order.
	instant := time.Date(2026, 1, 1, 16, 15, 2, 0, time.UTC)
	for _, zone := range []*time.Location{
		time.UTC,
		time.FixedZone("PST", -8*3600),
		time.FixedZone("NZDT", 13*3600),
	} {
		if got, want := JournalFilename(instant.In(zone)), "20260101T161502Z.jsonl"; got != want {
			t.Errorf("JournalFilename in %v = %q, want %q", zone, got, want)
		}
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

// TestJournalFilenameOrderSurvivesAZoneChange is the case the old
// UTC-only-inputs test could not see: a tree carried east to west, and the DST
// fall-back hour, both produce a *later* event whose local wall clock is
// numerically *earlier*. With the offset absent from the name, that inverts
// lexical order — so journal.newestFile would return a closed file and Append
// would reopen it, breaking §3.4's immutability guarantee.
func TestJournalFilenameOrderSurvivesAZoneChange(t *testing.T) {
	cases := []struct {
		name           string
		earlier, later time.Time
	}{
		{
			"carried from Auckland to Los Angeles",
			time.Date(2026, 8, 4, 23, 0, 0, 0, time.FixedZone("NZST", 12*3600)), // 11:00Z
			time.Date(2026, 8, 4, 12, 0, 0, 0, time.FixedZone("PDT", -7*3600)),  // 19:00Z
		},
		{
			"across the DST fall-back hour, without leaving the desk",
			time.Date(2026, 11, 1, 1, 30, 0, 0, time.FixedZone("PDT", -7*3600)), // 08:30Z
			time.Date(2026, 11, 1, 1, 15, 0, 0, time.FixedZone("PST", -8*3600)), // 09:15Z
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.earlier.Before(tc.later) {
				t.Fatalf("test setup: %v is not before %v", tc.earlier, tc.later)
			}
			first, second := JournalFilename(tc.earlier), JournalFilename(tc.later)
			if first >= second {
				t.Errorf("lexical order contradicts chronology: %q (earlier) >= %q (later)", first, second)
			}
		})
	}
}

// FuzzParseAtNeverPanics: `--at` and `--created` take a string straight off the
// command line (§15.1), so ParseAt is a parser at the edge of the program and
// owes its caller a value or an error and nothing else.
func FuzzParseAtNeverPanics(f *testing.F) {
	for _, seed := range []string{
		"", "2026-01-03", "2026-01-03T09", "2026-01-03T09:02", "2026-01-03T09:02:11",
		"2026-01-03T09:02:11Z", "2026-01-03T09:02:11-08:00", "2026-13-45",
		"0000-00-00", "2026-01-03T", "T09:02", "9999999999-01-01", "-1",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		for _, loc := range []*time.Location{time.UTC, time.FixedZone("PST", -8*3600)} {
			at, err := ParseAt(s, loc)
			if err != nil {
				if !at.IsZero() {
					t.Errorf("ParseAt(%q) returned both a time and an error", s)
				}
				continue
			}
			// Everything downstream formats it, compares it, and names a
			// journal file after it (§3.4), so each of those must survive
			// whatever the grammar accepted — and a filename that is not
			// lexically orderable would break §3.4's whole claim.
			if formatted := at.Format(time.RFC3339); formatted == "" {
				t.Errorf("ParseAt(%q) produced a time that formats to nothing", s)
			}
			if name := JournalFilename(at); !strings.HasSuffix(name, "Z.jsonl") {
				t.Errorf("ParseAt(%q) produced the journal filename %q, which is not the UTC form §3.4 fixes", s, name)
			}
			if !Equal(at, at) {
				t.Errorf("ParseAt(%q) produced a time that does not equal itself", s)
			}
		}
	})
}

// FuzzParseAtIsZoneSensitiveButNotZoneDependent is §15.1's rule stated as a
// property: what you type is local, what is stored is UTC. A form that carries
// no offset must land at a different instant in two zones — the whole reason the
// zone is an argument — and a form that carries one must land at the same
// instant in both, because the offset in the text is the answer.
func FuzzParseAtIsZoneSensitiveButNotZoneDependent(f *testing.F) {
	for _, seed := range []string{"2026-01-03", "2026-01-03T09:02", "2026-01-03T09:02:11Z", "2026-06-30T12:00:00+05:30"} {
		f.Add(seed)
	}
	east := time.FixedZone("IST", 5*3600+1800)
	f.Fuzz(func(t *testing.T, s string) {
		utc, err1 := ParseAt(s, time.UTC)
		other, err2 := ParseAt(s, east)
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("ParseAt(%q) parsed in one zone and not the other", s)
		}
		if err1 != nil {
			return
		}
		// The wall-clock reading is the same in both; only the instant moves.
		if utc.Format("2006-01-02T15:04:05") != other.Format("2006-01-02T15:04:05") {
			t.Fatalf("ParseAt(%q) read a different wall clock in two zones: %s vs %s", s, utc, other)
		}
		if _, offset := other.Zone(); offset == 0 && !utc.Equal(other) {
			t.Fatalf("ParseAt(%q) moved an instant that carries its own offset", s)
		}
	})
}
