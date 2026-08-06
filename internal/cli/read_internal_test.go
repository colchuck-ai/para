package cli

import (
	"bytes"
	"strings"
	"testing"
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
