package journal

import (
	"bytes"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/colchuck-ai/para/internal/testutil"
)

func mustParseAt(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", s, err)
	}
	return tm
}

// TestEncode_WorkedExample pins §3.1's own four worked lines byte for byte,
// one per event kind, including "note" as a field on the change event.
func TestEncode_WorkedExample(t *testing.T) {
	cases := []struct {
		name string
		e    Event
		want string
	}{
		{
			name: "change",
			e: Event{
				At:    mustParseAt(t, "2026-01-01T08:15:02-08:00"),
				Kind:  KindChange,
				Field: "status",
				From:  "planned",
				To:    "in-progress",
				Note:  "kickoff done",
			},
			want: `{"at":"2026-01-01T08:15:02-08:00","kind":"change","field":"status","from":"planned","to":"in-progress","note":"kickoff done"}` + "\n",
		},
		{
			name: "measurement",
			e: Event{
				At:    mustParseAt(t, "2026-01-03T09:02:11-08:00"),
				Kind:  KindMeasurement,
				Value: "880/11000",
			},
			want: `{"at":"2026-01-03T09:02:11-08:00","kind":"measurement","value":"880/11000"}` + "\n",
		},
		{
			name: "note",
			e: Event{
				At:   mustParseAt(t, "2026-01-04T17:40:00-08:00"),
				Kind: KindNote,
				Note: "waiting on the ingest team",
			},
			want: `{"at":"2026-01-04T17:40:00-08:00","kind":"note","note":"waiting on the ingest team"}` + "\n",
		},
		{
			name: "child",
			e: Event{
				At:    mustParseAt(t, "2026-01-05T11:00:00-08:00"),
				Kind:  KindChild,
				Op:    ChildOpAdded,
				Child: "q1-growth",
			},
			want: `{"at":"2026-01-05T11:00:00-08:00","kind":"child","op":"added","child":"q1-growth"}` + "\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Encode(c.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != c.want {
				t.Errorf("Encode() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDecode_WorkedExample(t *testing.T) {
	line := []byte(`{"at":"2026-01-01T08:15:02-08:00","kind":"change","field":"status","from":"planned","to":"in-progress","note":"kickoff done"}`)

	e, err := Decode(line)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.Kind != KindChange || e.Field != "status" || e.From != "planned" || e.To != "in-progress" || e.Note != "kickoff done" {
		t.Errorf("Decode() = %+v, unexpected fields", e)
	}
	if !e.At.Equal(mustParseAt(t, "2026-01-01T08:15:02-08:00")) {
		t.Errorf("Decode().At = %v, want the parsed instant", e.At)
	}
}

// TestChildMove proves a child event on a move carries both a locator pair
// and note (§3.1: "op ... and from/to locators for moves"; note is a field
// on every kind).
func TestChildMove(t *testing.T) {
	e := Event{
		At:    mustParseAt(t, "2026-02-01T09:00:00-08:00"),
		Kind:  KindChild,
		Op:    ChildOpMoved,
		Child: "training",
		From:  "areas.health.training",
		To:    "areas.fitness.training",
		Note:  "reorganized",
	}
	data, err := Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != e {
		t.Errorf("Decode(Encode(e)) = %+v, want %+v", got, e)
	}
}

func TestDecode_UnknownKind(t *testing.T) {
	line := []byte(`{"at":"2026-01-01T08:15:02-08:00","kind":"bogus"}`)
	if _, err := Decode(line); err == nil {
		t.Error("Decode with an unknown kind: want error, got nil")
	}
}

func TestDecode_InvalidJSON(t *testing.T) {
	if _, err := Decode([]byte("not json")); err == nil {
		t.Error("Decode with invalid JSON: want error, got nil")
	}
}

// TestEncode_Deterministic pins the byte-stability guarantee (§0.2): two
// independent Encode calls over the same event produce identical bytes.
func TestEncode_Deterministic(t *testing.T) {
	e := Event{At: mustParseAt(t, "2026-01-03T09:02:11-08:00"), Kind: KindMeasurement, Value: "880/11000"}

	a, err := Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b, err := Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two Encode calls produced different bytes:\n%q\n%q", a, b)
	}
}

// TestRoundTrip proves parse → render is a fixed point (§0.2), the
// property doctor's stale-projection check and every journal reader
// depend on.
func TestRoundTrip(t *testing.T) {
	data, err := Encode(Event{
		At:    mustParseAt(t, "2026-01-01T08:15:02-08:00"),
		Kind:  KindChange,
		Field: "status",
		From:  "planned",
		To:    "in-progress",
		Note:  "kickoff done",
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	testutil.AssertRoundTrip(t, data, Decode, Encode)
}

// FuzzEncodeDecode_Note is §0.2's round-trip property, fuzzed over an
// arbitrary note value on a change event. Invalid UTF-8 is out of scope —
// a note always originates from a Go string (a CLI arg), always valid
// UTF-8.
func FuzzEncodeDecode_Note(f *testing.F) {
	for _, seed := range []string{
		"", "plain", `has "quotes"`, `has\backslash`,
		"has\nnewline\ttab\r", "unicode 日本語 — em dash",
	} {
		f.Add(seed)
	}
	at := time.Date(2026, 1, 1, 8, 15, 2, 0, time.FixedZone("-08:00", -8*3600))
	f.Fuzz(func(t *testing.T, note string) {
		if !utf8.ValidString(note) {
			t.Skip()
		}
		e := Event{At: at, Kind: KindChange, Field: "status", From: "planned", To: "in-progress", Note: note}

		data, err := Encode(e)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		got, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(%q): %v", data, err)
		}
		if !got.At.Equal(e.At) || got.Kind != e.Kind || got.Field != e.Field ||
			got.From != e.From || got.To != e.To || got.Note != e.Note {
			t.Fatalf("round trip mismatch: got %+v, want %+v (encoded = %q)", got, e, data)
		}
	})
}
