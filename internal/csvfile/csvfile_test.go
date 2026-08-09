package csvfile

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/colchuck-ai/para/internal/testutil"
)

func TestEncode_Header(t *testing.T) {
	got, err := Encode(nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "at,value,decimal,progress,note\n"
	if string(got) != want {
		t.Errorf("Encode(nil) = %q, want %q", got, want)
	}
}

// TestEncode_WorkedExample pins §4.4's own worked example byte for byte:
// oldest first, decimal and progress at four decimal places, an empty note
// leaving its trailing comma with nothing after it.
func TestEncode_WorkedExample(t *testing.T) {
	rows := []Row{
		{At: "2026-01-03T09:02:11-08:00", Value: "880/11000", Decimal: 0.08, Progress: 0.47, Note: ""},
		{At: "2026-01-17T09:10:04-08:00", Value: "1320/12400", Decimal: 0.1065, Progress: 0.68, Note: "denominator grew after the launch"},
	}

	got, err := Encode(rows)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	testutil.Golden(t, "testdata/worked_example.csv", got)
}

func TestEncode_QuotesNoteContainingComma(t *testing.T) {
	rows := []Row{
		{At: "2026-01-03T09:02:11-08:00", Value: "42", Decimal: 42, Progress: 1, Note: "grew, then shrank"},
	}
	got, err := Encode(rows)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "at,value,decimal,progress,note\n" +
		"2026-01-03T09:02:11-08:00,42,42.0000,1.0000,\"grew, then shrank\"\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncode_NegativeProgress(t *testing.T) {
	rows := []Row{
		{At: "2026-01-03T09:02:11-08:00", Value: "400", Decimal: 400, Progress: -0.05, Note: ""},
	}
	got, err := Encode(rows)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "at,value,decimal,progress,note\n" +
		"2026-01-03T09:02:11-08:00,400,400.0000,-0.0500,\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

// TestEncode_Deterministic pins the byte-stability guarantee (§0.2): two
// independent Encode calls over the same rows produce identical bytes.
func TestEncode_Deterministic(t *testing.T) {
	rows := []Row{
		{At: "2026-01-03T09:02:11-08:00", Value: "880/11000", Decimal: 0.08, Progress: 0.47, Note: "a note"},
	}

	a, err := Encode(rows)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b, err := Encode(rows)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two Encode calls produced different bytes:\n%q\n%q", a, b)
	}
}

// FuzzEncodeIsStableAndParsable is the byte-stability guarantee §0.2 requires of
// every generated file, fuzzed at the one codec that has no Decode.
//
// MEASUREMENTS.csv is never read back as truth, so a round trip is not the
// property to check. Two are:
//
//   - Encode is a function of its rows. `doctor` re-derives this file in memory
//     and compares it byte for byte (§10), so a second Encode over the same rows
//     that differed by a byte would report drift on a clean tree.
//   - What it writes is CSV. A note carrying a comma, a quote, or a newline is
//     ordinary — §3.1 lets a note say anything — and the writer has to quote it
//     rather than produce a file with a different number of columns per row.
func FuzzEncodeIsStableAndParsable(f *testing.F) {
	seeds := [][]string{
		{"2026-01-03T08:00:00Z", "880/11000", "measured on the way in"},
		{"2026-01-03T08:00:00Z", "880/11000", "a note, with a comma"},
		{"2026-01-03T08:00:00Z", "880/11000", "a note with \"quotes\""},
		{"2026-01-03T08:00:00Z", "880/11000", "a note\nwith a newline"},
		{"", "", ""},
		{"2026-01-03T08:00:00Z", "true", "\r\n"},
	}
	for _, s := range seeds {
		f.Add(s[0], s[1], s[2], 0.08, 0.2353)
	}

	f.Fuzz(func(t *testing.T, at, value, note string, decimal, progress float64) {
		// A bound on the input, not on the property: a note is one line a human
		// typed (§3.1), and letting the engine grow one to megabytes buys no new
		// behaviour while costing every execution after it.
		if len(at)+len(value)+len(note) > 4096 {
			t.Skip()
		}
		rows := []Row{
			{At: at, Value: value, Decimal: decimal, Progress: progress, Note: note},
			// A second row, so a writer that lost track of the column count
			// between records has somewhere to show it.
			{At: at, Value: value, Decimal: progress, Progress: decimal, Note: note + note},
		}

		first, err := Encode(rows)
		if err != nil {
			return
		}
		second, err := Encode(rows)
		if err != nil {
			t.Fatalf("Encode succeeded and then failed over the same rows: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Fatalf("two Encode calls over the same rows differed:\n%q\n%q", first, second)
		}

		records, err := csv.NewReader(bytes.NewReader(first)).ReadAll()
		if err != nil {
			t.Fatalf("Encode produced something the csv reader rejects (%v):\n%q", err, first)
		}
		if len(records) != len(rows)+1 {
			t.Fatalf("Encode produced %d records for %d rows plus a header:\n%q", len(records), len(rows), first)
		}
		for i, rec := range records {
			if len(rec) != len(header) {
				t.Fatalf("record %d has %d columns, want %d:\n%q", i, len(rec), len(header), first)
			}
		}
	})
}
