package mdfile

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/colchuck-ai/para/internal/testutil"
)

func TestSplit(t *testing.T) {
	data := []byte("---\nname: \"Acme migration\"\nstatus: \"in-progress\"\n---\n# Acme migration\n\nSome human prose.\n")

	fm, body, err := Split(data)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	wantFM := "name: \"Acme migration\"\nstatus: \"in-progress\"\n"
	if string(fm) != wantFM {
		t.Errorf("frontmatter = %q, want %q", fm, wantFM)
	}
	wantBody := "# Acme migration\n\nSome human prose.\n"
	if string(body) != wantBody {
		t.Errorf("body = %q, want %q", body, wantBody)
	}
}

func TestSplit_EmptyFrontmatter(t *testing.T) {
	data := []byte("---\n---\nbody only\n")
	fm, body, err := Split(data)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(fm) != 0 {
		t.Errorf("frontmatter = %q, want empty", fm)
	}
	if string(body) != "body only\n" {
		t.Errorf("body = %q, want %q", body, "body only\n")
	}
}

func TestSplit_PreservesBodyExactly(t *testing.T) {
	body := "line one\n\n\nline two with trailing spaces   \nno trailing newline at all"
	data := []byte("---\nname: \"x\"\n---\n" + body)

	_, gotBody, err := Split(data)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if string(gotBody) != body {
		t.Errorf("body = %q, want %q", gotBody, body)
	}
}

func TestSplit_MissingOpeningDelimiter(t *testing.T) {
	if _, _, err := Split([]byte("no frontmatter here\n")); err == nil {
		t.Error("Split with no opening delimiter: want error, got nil")
	}
}

func TestSplit_MissingClosingDelimiter(t *testing.T) {
	if _, _, err := Split([]byte("---\nname: \"x\"\nbody with no closing delimiter\n")); err == nil {
		t.Error("Split with no closing delimiter: want error, got nil")
	}
}

func TestRender(t *testing.T) {
	fields := []Field{
		{Key: "name", Value: String("Acme migration")},
		{Key: "tags", Value: StringArray([]string{"kafka", "consumer"})},
	}
	body := []byte("# Acme migration\n\nSome human prose.\n")

	got, err := Render(fields, body)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "---\n" +
		"name: \"Acme migration\"\n" +
		"tags: [\"kafka\", \"consumer\"]\n" +
		"---\n" +
		"# Acme migration\n\nSome human prose.\n"
	if string(got) != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

func TestRender_EmptyFields(t *testing.T) {
	got, err := Render(nil, []byte("body\n"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "---\n---\nbody\n"
	if string(got) != want {
		t.Errorf("Render() = %q, want %q", got, want)
	}
}

// TestRender_Deterministic pins §0.2's byte-stability guarantee: two
// independent Render calls over the same fields and body produce identical
// bytes.
func TestRender_Deterministic(t *testing.T) {
	fields := []Field{{Key: "name", Value: String("x")}}
	body := []byte("body\n")

	a, err := Render(fields, body)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	b, err := Render(fields, body)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("two Render calls produced different bytes:\n%q\n%q", a, b)
	}
}

// TestEncodeFrontmatter_EscapesControlCharacters proves a stray control
// byte in a string value is escaped, not written raw — an unescaped one
// would make the frontmatter invalid YAML on the next parse (§0.2).
func TestEncodeFrontmatter_EscapesControlCharacters(t *testing.T) {
	got, err := EncodeFrontmatter([]Field{{Key: "note", Value: String("a\x01b")}})
	if err != nil {
		t.Fatalf("EncodeFrontmatter: %v", err)
	}
	want := "note: \"a\\u0001b\"\n"
	if string(got) != want {
		t.Errorf("EncodeFrontmatter() = %q, want %q", got, want)
	}

	doc, err := DecodeFrontmatter(got)
	if err != nil {
		t.Fatalf("DecodeFrontmatter: %v", err)
	}
	if s, ok := doc.String("note"); !ok || s != "a\x01b" {
		t.Errorf("String(note) = %q, %v, want %q, true", s, ok, "a\x01b")
	}
}

func TestDecodeFrontmatter_Accessors(t *testing.T) {
	raw := []byte("name: \"Acme migration\"\n" +
		"tags: [\"kafka\", \"consumer\"]\n" +
		"priority_rank: 2\n" +
		"pace: 0.8\n" +
		"archived: true\n")

	doc, err := DecodeFrontmatter(raw)
	if err != nil {
		t.Fatalf("DecodeFrontmatter: %v", err)
	}

	if s, ok := doc.String("name"); !ok || s != "Acme migration" {
		t.Errorf("String(name) = %q, %v, want %q, true", s, ok, "Acme migration")
	}
	if a, ok := doc.StringArray("tags"); !ok || len(a) != 2 || a[0] != "kafka" || a[1] != "consumer" {
		t.Errorf("StringArray(tags) = %v, %v, want [kafka consumer], true", a, ok)
	}
	if i, ok := doc.Int64("priority_rank"); !ok || i != 2 {
		t.Errorf("Int64(priority_rank) = %v, %v, want 2, true", i, ok)
	}
	if f, ok := doc.Float64("pace"); !ok || f != 0.8 {
		t.Errorf("Float64(pace) = %v, %v, want 0.8, true", f, ok)
	}
	if b, ok := doc.Bool("archived"); !ok || !b {
		t.Errorf("Bool(archived) = %v, %v, want true, true", b, ok)
	}
	if _, ok := doc.String("missing"); ok {
		t.Error("String(missing): want ok=false")
	}
}

// TestRoundTrip_FrontmatterBody proves parse → render is a fixed point
// (§0.2): the body survives untouched, and re-encoding the decoded
// frontmatter fields in their declared order reproduces the source bytes.
func TestRoundTrip_FrontmatterBody(t *testing.T) {
	schema := []string{"name", "tags"}

	data, err := Render([]Field{
		{Key: "name", Value: String("Acme migration")},
		{Key: "tags", Value: StringArray([]string{"kafka", "consumer"})},
	}, []byte("# Acme migration\n\nSome human prose, unchanged forever.\n"))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	decode := func(b []byte) (parsed, error) {
		fm, body, err := Split(b)
		if err != nil {
			return parsed{}, err
		}
		doc, err := DecodeFrontmatter(fm)
		if err != nil {
			return parsed{}, err
		}
		return parsed{doc: doc, body: body}, nil
	}
	encode := func(p parsed) ([]byte, error) {
		fields := make([]Field, len(schema))
		for i, key := range schema {
			v, ok := p.doc.Value(key)
			if !ok {
				t.Fatalf("schema key %q missing from decoded document", key)
			}
			fields[i] = Field{Key: key, Value: v}
		}
		return Render(fields, p.body)
	}

	testutil.AssertRoundTrip(t, data, decode, encode)
}

type parsed struct {
	doc  Document
	body []byte
}

// FuzzEncodeDecodeFrontmatter_String is §0.2's round-trip property, fuzzed
// over an arbitrary string field value. Invalid UTF-8 is out of scope —
// see the identical note on ptoml's version of this fuzz target.
func FuzzEncodeDecodeFrontmatter_String(f *testing.F) {
	for _, seed := range []string{
		"", "plain", `has "quotes"`, `has\backslash`,
		"has\nnewline\ttab\r", "has\x00control\x1f", "unicode 日本語 — em dash",
		"has\x7fDEL", "hasC1control", // YAML forbids these unescaped too
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip()
		}
		fm, err := EncodeFrontmatter([]Field{{Key: "v", Value: String(s)}})
		if err != nil {
			t.Fatalf("EncodeFrontmatter(%q): %v", s, err)
		}
		doc, err := DecodeFrontmatter(fm)
		if err != nil {
			t.Fatalf("DecodeFrontmatter(%q) (from EncodeFrontmatter(%q)): %v", fm, s, err)
		}
		got, ok := doc.String("v")
		if !ok {
			t.Fatalf("String(v) missing after decode; encoded = %q", fm)
		}
		if got != s {
			t.Fatalf("round trip mismatch: got %q, want %q (encoded = %q)", got, s, fm)
		}
	})
}

// FuzzRender_BodyPreserved is §2.2's promise for README.md/SKILL.md,
// fuzzed over an arbitrary body: a rewrite must preserve the body byte for
// byte, no matter what it contains — including text that looks like a
// frontmatter delimiter, since Split only ever looks for the *first*
// closing "---" line.
func FuzzRender_BodyPreserved(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("# Title\n\nSome prose.\n"),
		[]byte(""),
		[]byte("---\nlooks like frontmatter but isn't\n---\n"),
		[]byte("no trailing newline"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		data, err := Render([]Field{{Key: "name", Value: String("x")}}, body)
		if err != nil {
			t.Fatalf("Render: %v", err)
		}
		_, gotBody, err := Split(data)
		if err != nil {
			t.Fatalf("Split(%q): %v", data, err)
		}
		if !bytes.Equal(gotBody, body) {
			t.Fatalf("body not preserved: got %q, want %q", gotBody, body)
		}
	})
}
