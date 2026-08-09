package ptoml

import (
	"bytes"
	"math"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/colchuck-ai/para/internal/testutil"
)

func TestEncode_KeyValueLines(t *testing.T) {
	fields := []Field{
		{Key: "name", Value: String("Acme migration")},
		{Key: "priority", Value: String("high")},
		{Key: "tags", Value: StringArray([]string{"kafka", "consumer"})},
		{Key: "created", Value: String("2026-01-01T08:15:00-08:00")},
	}

	got, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	want := "name = \"Acme migration\"\n" +
		"priority = \"high\"\n" +
		"tags = [\"kafka\", \"consumer\"]\n" +
		"created = \"2026-01-01T08:15:00-08:00\"\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncode_ScalarKinds(t *testing.T) {
	fields := []Field{
		{Key: "count", Value: Int64(4194304)},
		{Key: "pace", Value: Float64(0.8)},
		{Key: "whole", Value: Float64(1)},
		{Key: "enabled", Value: Bool(true)},
		{Key: "disabled", Value: Bool(false)},
		{Key: "empty", Value: StringArray(nil)},
	}

	got, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	want := "count = 4194304\n" +
		"pace = 0.8\n" +
		"whole = 1.0\n" +
		"enabled = true\n" +
		"disabled = false\n" +
		"empty = []\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncode_DottedKey(t *testing.T) {
	fields := []Field{
		{Key: "log.rotate-bytes", Value: Int64(4194304)},
	}
	got, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "log.rotate-bytes = 4194304\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncode_StringEscaping(t *testing.T) {
	fields := []Field{
		{Key: "note", Value: String("she said \"hi\"\nnew line\\backslash")},
	}
	got, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want := "note = \"she said \\\"hi\\\"\\nnew line\\\\backslash\"\n"
	if string(got) != want {
		t.Errorf("Encode() = %q, want %q", got, want)
	}
}

func TestEncode_InvalidKey(t *testing.T) {
	fields := []Field{{Key: "has space", Value: String("x")}}
	if _, err := Encode(fields); err == nil {
		t.Error("Encode with an illegal key: want error, got nil")
	}
}

// TestEncode_Deterministic pins the byte-stability guarantee (§0.2): two
// independent Encode calls over the same fields produce identical bytes.
func TestEncode_Deterministic(t *testing.T) {
	fields := []Field{
		{Key: "name", Value: String("Acme migration")},
		{Key: "tags", Value: StringArray([]string{"kafka", "consumer"})},
	}

	a, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	b, err := Encode(fields)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two Encode calls produced different bytes:\n%q\n%q", a, b)
	}
}

func TestDecode_Accessors(t *testing.T) {
	data := []byte("name = \"Acme migration\"\n" +
		"tags = [\"kafka\", \"consumer\"]\n" +
		"count = 4194304\n" +
		"pace = 0.8\n" +
		"enabled = true\n" +
		"log.rotate-bytes = 2048\n")

	doc, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if s, ok := doc.String("name"); !ok || s != "Acme migration" {
		t.Errorf("String(name) = %q, %v, want %q, true", s, ok, "Acme migration")
	}
	if a, ok := doc.StringArray("tags"); !ok || len(a) != 2 || a[0] != "kafka" || a[1] != "consumer" {
		t.Errorf("StringArray(tags) = %v, %v, want [kafka consumer], true", a, ok)
	}
	if i, ok := doc.Int64("count"); !ok || i != 4194304 {
		t.Errorf("Int64(count) = %v, %v, want 4194304, true", i, ok)
	}
	if f, ok := doc.Float64("pace"); !ok || f != 0.8 {
		t.Errorf("Float64(pace) = %v, %v, want 0.8, true", f, ok)
	}
	if b, ok := doc.Bool("enabled"); !ok || !b {
		t.Errorf("Bool(enabled) = %v, %v, want true, true", b, ok)
	}
	if i, ok := doc.Int64("log.rotate-bytes"); !ok || i != 2048 {
		t.Errorf("Int64(log.rotate-bytes) = %v, %v, want 2048, true", i, ok)
	}
	if _, ok := doc.String("missing"); ok {
		t.Error("String(missing): want ok=false")
	}
}

// TestRoundTrip proves parse → render is a fixed point (§0.2): the property
// doctor's stale-projection check depends on. Order is supplied by the
// caller's declared schema, not recovered from the source — Decode gives
// random access by key, and the schema below is what a real file kind will
// declare once one exists (Phase 7/8).
func TestRoundTrip(t *testing.T) {
	schema := []string{"name", "priority", "tags", "count", "pace", "enabled", "log.rotate-bytes"}

	data, err := Encode([]Field{
		{Key: "name", Value: String("Acme migration")},
		{Key: "priority", Value: String("high")},
		{Key: "tags", Value: StringArray([]string{"kafka", "consumer"})},
		{Key: "count", Value: Int64(4194304)},
		{Key: "pace", Value: Float64(0.8)},
		{Key: "enabled", Value: Bool(true)},
		{Key: "log.rotate-bytes", Value: Int64(2048)},
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	decode := func(b []byte) (Document, error) { return Decode(b) }
	encode := func(doc Document) ([]byte, error) {
		fields := make([]Field, len(schema))
		for i, key := range schema {
			v, ok := doc.Value(key)
			if !ok {
				t.Fatalf("schema key %q missing from decoded document", key)
			}
			fields[i] = Field{Key: key, Value: v}
		}
		return Encode(fields)
	}

	testutil.AssertRoundTrip(t, data, decode, encode)
}

// FuzzEncodeDecode_String is §0.2's round-trip property, fuzzed over an
// arbitrary string value: Encode then Decode must recover exactly what was
// given. Seeds carry the escaping edge cases quoteString handles by hand.
// Invalid UTF-8 is out of scope — every string value this codec ever
// carries originates from a Go string (a CLI arg, a parsed file), which is
// always valid UTF-8.
func FuzzEncodeDecode_String(f *testing.F) {
	for _, seed := range []string{
		"", "plain", `has "quotes"`, `has\backslash`,
		"has\nnewline\ttab\r", "has\x00control\x1f", "unicode 日本語 — em dash",
		"has\x7fDEL", // TOML forbids DEL unescaped too, not just the C0 range
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		data, err := Encode([]Field{{Key: "v", Value: String(s)}})
		if err != nil {
			t.Fatalf("Encode(%q): %v", s, err)
		}
		doc, err := Decode(data)
		if err != nil {
			t.Fatalf("Decode(%q) (from Encode(%q)): %v", data, s, err)
		}
		got, ok := doc.String("v")
		if !ok {
			t.Fatalf("String(v) missing after decode; encoded = %q", data)
		}
		if got != s {
			t.Fatalf("round trip mismatch: got %q, want %q (encoded = %q)", got, s, data)
		}
	})
}

func TestDocumentKeysFlattensNestedTablesInLexicalOrder(t *testing.T) {
	tests := []struct {
		name string
		toml string
		want []string
	}{
		{
			name: "dotted keys flatten back to the dotted form they were written as",
			toml: "emit.claude = true\nemit.claude-skills = \"symlink\"\nlog.rotate-bytes = 4194304\n",
			want: []string{"emit.claude", "emit.claude-skills", "log.rotate-bytes"},
		},
		{
			name: "a [table] header flattens the same way, so both spellings read alike",
			toml: "[emit]\nclaude = true\ngitattributes = false\n",
			want: []string{"emit.claude", "emit.gitattributes"},
		},
		{
			name: "order is lexical, not the order the file happened to be written in",
			toml: "zeta = 1\nalpha = 2\nmiddle = 3\n",
			want: []string{"alpha", "middle", "zeta"},
		},
		{
			name: "top-level scalars and nested tables sort together",
			toml: "project.stale-after = 14\narea.stale-after = 30\nreview.cadence = 90\n",
			want: []string{"area.stale-after", "project.stale-after", "review.cadence"},
		},
		{
			name: "an empty document has no keys",
			toml: "",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Decode([]byte(tt.toml))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			got, err := doc.Keys()
			if err != nil {
				t.Fatalf("Keys(): %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Keys() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Keys is what lets a config.toml be rewritten without dropping a key para
// does not recognise, so every key it reports must be readable back through
// Value.
func TestDocumentKeysReachEveryValue(t *testing.T) {
	const doc = `s = "str"
i = 12
f = 0.5
b = true
a = ["x", "y"]
nested.deep.key = "here"
`
	d, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	keys, err := d.Keys()
	if err != nil {
		t.Fatalf("Keys(): %v", err)
	}
	want := []string{"a", "b", "f", "i", "nested.deep.key", "s"}
	if !slices.Equal(keys, want) {
		t.Fatalf("Keys() = %v, want %v", keys, want)
	}
	for _, k := range keys {
		if _, ok := d.Value(k); !ok {
			t.Errorf("Value(%q) missing, but Keys() reported it", k)
		}
	}
}

// TOML has literal spellings for the three floats that are not numbers, and
// they are not Go's. Appending ".0" to "NaN" — which is what the
// make-it-look-like-a-float fixup did before it learned about these — writes
// a file no reader can parse, and para would have written it over a file the
// user could still read.
func TestEncodeFloatsThatAreNotNumbers(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  string
	}{
		{"not a number", math.NaN(), "v = nan\n"},
		{"positive infinity", math.Inf(1), "v = inf\n"},
		{"negative infinity", math.Inf(-1), "v = -inf\n"},
		{"negative zero keeps its sign", math.Copysign(0, -1), "v = -0.0\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode([]Field{{Key: "v", Value: Float64(tt.value)}})
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if string(got) != tt.want {
				t.Fatalf("Encode() = %q, want %q", got, tt.want)
			}
			// The point of the exercise: what para writes, para can read.
			doc, err := Decode(got)
			if err != nil {
				t.Fatalf("Decode(%q): %v", got, err)
			}
			back, ok := doc.Float64("v")
			if !ok {
				t.Fatalf("Float64(v) missing after decoding %q", got)
			}
			if math.Float64bits(back) != math.Float64bits(tt.value) {
				t.Errorf("round trip gave %v, want %v", back, tt.value)
			}
		})
	}
}

// A key para cannot write back is worse than a key it refuses to read: the
// two spellings "a.b" (one quoted key) and [a] b (a table) flatten to the
// same dotted string, so a document holding both has two different facts
// under one name. Keys reports that rather than handing back a list with a
// duplicate in it.
func TestDocumentKeysRefusesKeysItCannotReproduce(t *testing.T) {
	tests := []struct {
		name string
		toml string
	}{
		{"a quoted key containing a dot collides with a real table", "\"a.b\" = 1\n\n[a]\nb = 2\n"},
		{"a quoted key containing a dot, alone", "\"a.b\" = 1\n"},
		{"a quoted key containing a space", "\"my key\" = 1\n"},
		{"a quoted key outside the bare charset", "\"café\" = 1\n"},
		{"a quoted key inside a table", "[emit]\n\"my key\" = 1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Decode([]byte(tt.toml))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if _, err := doc.Keys(); err == nil {
				t.Errorf("Keys() = nil error, want a refusal naming the key")
			}
		})
	}
}
