package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestTableSizesColumnsToTheContentPrinted is the layout rule the read commands
// share: no terminal probing, no truncation, columns as wide as the rows
// actually printed and no wider.
func TestTableSizesColumnsToTheContentPrinted(t *testing.T) {
	var tbl table
	tbl.add("projects.acme", "project", "in-progress")
	tbl.add("areas.h", "area", dash)

	var out bytes.Buffer
	tbl.write(&out)

	want := "projects.acme  project  in-progress\n" +
		"areas.h        area     —\n"
	if got := out.String(); got != want {
		t.Errorf("table =\n%q\nwant\n%q", got, want)
	}
}

// TestTableLeavesNoTrailingWhitespace matters because these rows are compared
// byte for byte as golden files, and a row that stops early must not pad out to
// the width of one that did not.
func TestTableLeavesNoTrailingWhitespace(t *testing.T) {
	var tbl table
	tbl.add("a", "long-value-here")
	tbl.add("b", "")

	var out bytes.Buffer
	tbl.write(&out)

	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("line %q has trailing whitespace", line)
		}
	}
}

// TestTableNoteHangsWithoutWideningAColumn is why `note` exists: §16.1's
// key-result numbers are longer than any name, and threading them through the
// name column would push every status in the block off to the right.
func TestTableNoteHangsWithoutWideningAColumn(t *testing.T) {
	var tbl table
	tbl.add("q1-growth", "Grow signups", "in-progress")
	tbl.add("signups", "Weekly signups", "at-risk")
	tbl.note("480/9000 → 880/11000 / 2000/12000   progress 0.24")

	var out bytes.Buffer
	tbl.write(&out)

	want := "q1-growth  Grow signups    in-progress\n" +
		"signups    Weekly signups  at-risk\n" +
		"           480/9000 → 880/11000 / 2000/12000   progress 0.24\n"
	if got := out.String(); got != want {
		t.Errorf("table =\n%s\nwant\n%s", got, want)
	}
}

func TestAgoAndUntil(t *testing.T) {
	cases := []struct {
		days             int
		wantAgo, wantFwd string
	}{
		{0, "today", "today"},
		{1, "1 day ago", "in 1 day"},
		{31, "31 days ago", "in 31 days"},
		{181, "181 days ago", "in 181 days"},
		// A deadline in the past reads as the past in either direction, rather
		// than as "in -3 days".
		{-3, "in the future", "3 days ago"},
		{-1, "in the future", "1 day ago"},
	}
	for _, tc := range cases {
		if got := ago(tc.days); got != tc.wantAgo {
			t.Errorf("ago(%d) = %q, want %q", tc.days, got, tc.wantAgo)
		}
		if got := until(tc.days); got != tc.wantFwd {
			t.Errorf("until(%d) = %q, want %q", tc.days, got, tc.wantFwd)
		}
	}
}

// TestExactAndNumber keeps the two number spellings apart: a threshold is
// printed as it was configured, a derived ratio to a fixed two places.
func TestExactAndNumber(t *testing.T) {
	if got := exact(14); got != "14" {
		t.Errorf("exact(14) = %q, want 14", got)
	}
	if got := exact(0.8); got != "0.8" {
		t.Errorf("exact(0.8) = %q, want 0.8", got)
	}
	if got := number(0.23529411764705885); got != "0.24" {
		t.Errorf("number(0.2353) = %q, want 0.24", got)
	}
	if got := number(1); got != "1.00" {
		t.Errorf("number(1) = %q, want 1.00", got)
	}
}

// TestTableHeadingTakesNoPartInSizing is why `head` exists: §20's group
// headings break one table into sections while every row's columns stay aligned
// down the whole output, which is how §26 prints a review. A heading that
// widened a column would defeat exactly that.
func TestTableHeadingTakesNoPartInSizing(t *testing.T) {
	tbl := table{indent: "  "}
	tbl.head("stale (2)")
	tbl.add("areas.fitness", "63 days", "area.stale-after 30")
	tbl.add("projects.late", "59 days", "project.stale-after 30")
	tbl.head("blocked (1)")
	tbl.add("projects.website", "0 days")

	var out bytes.Buffer
	tbl.write(&out)

	// The heading is unindented and unpadded; the third row's columns line up
	// with the first two even though its group has one member, and its trailing
	// empty cell adds no whitespace.
	want := "stale (2)\n" +
		"  areas.fitness     63 days  area.stale-after 30\n" +
		"  projects.late     59 days  project.stale-after 30\n" +
		"blocked (1)\n" +
		"  projects.website  0 days\n"
	if got := out.String(); got != want {
		t.Errorf("table =\n%s\nwant\n%s", got, want)
	}
}

// TestSpanIsTheRootOfAgoAndUntil keeps one plural rule rather than three: §20's
// column prints the bare span and §16.1's prints it as a reference to a point in
// time, and the two disagreeing about "1 day" would be a spelling bug nobody
// would think to test for.
func TestSpanIsTheRootOfAgoAndUntil(t *testing.T) {
	for _, n := range []int{0, 1, 2, 31} {
		if n > 0 {
			if got, want := ago(n), span(n)+" ago"; got != want {
				t.Errorf("ago(%d) = %q, want %q", n, got, want)
			}
			if got, want := until(n), "in "+span(n); got != want {
				t.Errorf("until(%d) = %q, want %q", n, got, want)
			}
		}
	}
	if got := span(1); got != "1 day" {
		t.Errorf("span(1) = %q, want %q", got, "1 day")
	}
	if got := span(0); got != "0 days" {
		t.Errorf("span(0) = %q, want %q", got, "0 days")
	}
}

// TestTruncateRunesCutsByRuneNotByte is para-c3u: attentionSource shortens a
// note's text to attentionSnippetLength runes, and a multi-byte character
// sitting at the cut point must not be split into invalid UTF-8.
func TestTruncateRunesCutsByRuneNotByte(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{name: "empty string", s: "", n: 5, want: ""},
		{name: "under the limit", s: "waiting", n: 50, want: "waiting"},
		{name: "exactly at the limit", s: "12345", n: 5, want: "12345"},
		{name: "one over the limit", s: "123456", n: 5, want: "12345…"},
		{
			name: "multi-byte rune sitting exactly at the cut",
			// Each of these three is a multi-byte rune (é is two bytes in
			// UTF-8); a byte-based cut at 2 would split the second one.
			s: "éééé", n: 2, want: "éé…",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncateRunes(c.s, c.n)
			if got != c.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("truncateRunes(%q, %d) = %q, not valid UTF-8", c.s, c.n, got)
			}
		})
	}
}
