package csvfile

import (
	"bytes"
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
