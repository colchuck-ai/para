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

// TestEncode_NoAttentionNote pins the one field this codec has gained since
// §3.1's worked examples were written: a note that opts out of the attention
// clock (§3.6, §18.6) carries `no_attention` after `note`, and an ordinary
// note — the overwhelming majority of every note ever written — carries
// nothing new at all, which TestEncode_WorkedExample's unchanged "note" case
// already pins.
func TestEncode_NoAttentionNote(t *testing.T) {
	e := NewNoteNoAttention(mustParseAt(t, "2026-01-04T17:40:00-08:00"), "retired the old beads IDs")
	want := `{"at":"2026-01-04T17:40:00-08:00","kind":"note","note":"retired the old beads IDs","no_attention":true}` + "\n"
	got, err := Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

// TestDecode_NoteWithoutNoAttentionCounts is the backward-compatibility case
// no_attention's omitempty exists for: every note ever written before this
// field existed has no `no_attention` key at all, and it must decode exactly
// as it always attended — a zero-value bool defaulting to "counts" is the only
// reading that does not retroactively change what old journals mean.
func TestDecode_NoteWithoutNoAttentionCounts(t *testing.T) {
	line := []byte(`{"at":"2026-01-04T17:40:00-08:00","kind":"note","note":"waiting on the ingest team"}`)
	e, err := Decode(line)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.NoAttention {
		t.Errorf("Decode() of a pre-existing note set NoAttention, want false")
	}
	if !attends(e) {
		t.Errorf("a decoded pre-existing note must still count toward attention")
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

// TestSuppress_RoundTrip proves a suppress event round-trips through
// Encode/Decode, and that Decode accepts an empty `value` — the shape
// `para unsuppress` writes (§28.2) — as a legal, decodable event.
func TestSuppress_RoundTrip(t *testing.T) {
	cases := []struct {
		name string
		e    Event
	}{
		{
			name: "suppress",
			e: Event{
				At:    mustParseAt(t, "2026-08-20T10:00:00-07:00"),
				Kind:  KindSuppress,
				Value: "2027-03-01",
				Note:  "paused, resumes with Q1 relaunch",
			},
		},
		{
			name: "unsuppress (empty until)",
			e: Event{
				At:   mustParseAt(t, "2026-09-01T09:00:00-07:00"),
				Kind: KindSuppress,
				Note: "relaunch moved up",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := Encode(c.e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got != c.e {
				t.Errorf("Decode(Encode(e)) = %+v, want %+v", got, c.e)
			}
		})
	}
}

// TestDecode_SuppressEncoding pins the wire shape: `value` carries `until` as
// typed (due's precision, §15.1, §28.2), not a UTC instant.
func TestDecode_SuppressEncoding(t *testing.T) {
	line := []byte(`{"at":"2026-08-20T10:00:00-07:00","kind":"suppress","value":"2027-03-01","note":"paused, resumes with Q1 relaunch"}`)
	e, err := Decode(line)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if e.Kind != KindSuppress || e.Value != "2027-03-01" || e.Note != "paused, resumes with Q1 relaunch" {
		t.Errorf("Decode() = %+v, unexpected fields", e)
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

// TestDecode_MissingAt covers §3.1's other required key. A line with no `at`
// has no place in an ordering that comes from `at` and never from file
// position, so the codec refuses it here rather than letting it sort as the
// zero instant — which is also what lets doctor report it as a `journal`
// finding with a file and a line (§10).
func TestDecode_MissingAt(t *testing.T) {
	cases := map[string]string{
		"absent": `{"kind":"note","note":"no instant"}`,
		"empty":  `{"at":"","kind":"note","note":"no instant"}`,
		"zero":   `{"at":"0001-01-01T00:00:00Z","kind":"note","note":"no instant"}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode([]byte(line)); err == nil {
				t.Errorf("Decode(%s): want error, got nil", line)
			}
		})
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
