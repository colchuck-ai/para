package config

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
)

func TestDecodeEncodeRoundTripIsAFixedPoint(t *testing.T) {
	const src = `emit.claude = true
emit.claude-skills = "copy"
key-result.at-risk-pace = 0.8
log.rotate-bytes = 4194304
project.stale-after = 14
`
	f, err := Decode([]byte(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(got) != src {
		t.Errorf("round trip changed the file:\ngot:\n%s\nwant:\n%s", got, src)
	}

	again, err := Decode(got)
	if err != nil {
		t.Fatalf("Decode (second): %v", err)
	}
	twice, err := again.Encode()
	if err != nil {
		t.Fatalf("Encode (second): %v", err)
	}
	if string(twice) != string(got) {
		t.Errorf("a second round trip changed the file:\ngot:\n%s\nwant:\n%s", twice, got)
	}
}

// The file is written in one declared order regardless of the order it was
// read in, so two people setting the same keys in different orders produce
// the same bytes (§0.2).
func TestEncodeIsLexicalRegardlessOfInputOrder(t *testing.T) {
	const scrambled = `project.stale-after = 14
emit.gitattributes = false
area.stale-after = 30
`
	const want = `area.stale-after = 30
emit.gitattributes = false
project.stale-after = 14
`
	f, err := Decode([]byte(scrambled))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(got) != want {
		t.Errorf("Encode() =\n%s\nwant\n%s", got, want)
	}
}

// A [table] header is TOML's other spelling of the same fact, and it must
// survive a rewrite as the dotted form rather than being dropped.
func TestDecodeAcceptsTableHeaders(t *testing.T) {
	const src = "[emit]\nclaude = true\ngitattributes = false\n"
	f, err := Decode([]byte(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	const want = "emit.claude = true\nemit.gitattributes = false\n"
	if string(got) != want {
		t.Errorf("Encode() =\n%s\nwant\n%s", got, want)
	}
}

// A key para does not recognise belongs to a newer para or to a hand edit.
// Dropping it on rewrite would make `config set` a data-loss operation, so
// it survives — reporting it is doctor's job (§10), not this codec's.
func TestUnknownKeysSurviveARewrite(t *testing.T) {
	const src = `emit.claude = true
future.knob = "whatever"
zzz.last = 3
`
	f, err := Decode([]byte(src))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	mustSet(t, &f, "project.stale-after", ptoml.Int64(30))
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	const want = `emit.claude = true
future.knob = "whatever"
project.stale-after = 30
zzz.last = 3
`
	if string(got) != want {
		t.Errorf("Encode() =\n%s\nwant\n%s", got, want)
	}
}

// The other half of not losing data: a value this codec cannot carry is
// refused loudly, rather than dropped quietly on the next rewrite.
func TestDecodeRefusesAValueItCannotCarry(t *testing.T) {
	for _, src := range []string{
		"when = 2026-01-01T00:00:00Z\n", // a TOML datetime, not a string
		"mixed = [1, \"two\"]\n",
	} {
		_, err := Decode([]byte(src))
		if err == nil {
			t.Errorf("Decode(%q) = nil error, want a refusal", src)
			continue
		}
		var perr *paraerr.Error
		if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
			t.Errorf("Decode(%q) error = %v, want a %v error", src, err, paraerr.KindValidation)
		}
	}
}

func TestSetReportsWhetherItChangedAnything(t *testing.T) {
	f, err := Decode([]byte("project.stale-after = 14\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if changed := mustSet(t, &f, "project.stale-after", ptoml.Int64(14)); changed {
		t.Error("Set() to the value already stored reported a change; §3.1 says it writes nothing")
	}
	if changed := mustSet(t, &f, "project.stale-after", ptoml.Int64(30)); !changed {
		t.Error("Set() to a new value reported no change")
	}
	if changed := mustSet(t, &f, "emit.claude", ptoml.Bool(true)); !changed {
		t.Error("Set() of an absent key reported no change")
	}
	// The same value in a different TOML type is a change: 14 and "14"
	// are not the same stored fact.
	if changed := mustSet(t, &f, "project.stale-after", ptoml.String("30")); !changed {
		t.Error("Set() to the same text in another type reported no change")
	}
}

func TestUnsetRemovesAndReportsWhetherItChangedAnything(t *testing.T) {
	f, err := Decode([]byte("project.stale-after = 14\nemit.claude = true\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if changed := f.Unset("project.stale-after"); !changed {
		t.Error("Unset() of a present key reported no change")
	}
	if _, ok := f.Get("project.stale-after"); ok {
		t.Error("Get() still finds the key after Unset()")
	}
	if changed := f.Unset("project.stale-after"); changed {
		t.Error("Unset() of an absent key reported a change")
	}

	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(got) != "emit.claude = true\n" {
		t.Errorf("Encode() = %q, want %q", got, "emit.claude = true\n")
	}
}

func TestEmptyFileEncodesToNoBytes(t *testing.T) {
	var f File
	got, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Encode() = %q, want no bytes", got)
	}
	if f.Len() != 0 {
		t.Errorf("Len() = %d, want 0", f.Len())
	}
}

func TestKeysAreLexical(t *testing.T) {
	f, err := Decode([]byte("zzz.last = 1\nemit.claude = true\narea.stale-after = 3\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := []string{"area.stale-after", "emit.claude", "zzz.last"}
	if got := f.Keys(); !slices.Equal(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

// Read is how every level of the chain is loaded, and most levels have no
// config.toml at all — an absent file is an empty config, not an error.
func TestReadTreatsAnAbsentFileAsEmpty(t *testing.T) {
	dir := t.TempDir()

	f, err := Read(filepath.Join(dir, ".para", "config.toml"))
	if err != nil {
		t.Fatalf("Read of an absent file: %v", err)
	}
	if f.Len() != 0 {
		t.Errorf("Len() = %d, want 0", f.Len())
	}
}

func TestReadParsesAFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("project.stale-after = 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	v, ok := f.Get("project.stale-after")
	if !ok {
		t.Fatal("Get(project.stale-after) not found")
	}
	if Format(v) != "30" {
		t.Errorf("value = %q, want %q", Format(v), "30")
	}
}

func TestReadReportsAMalformedFileWithItsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("this is not = = toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Read(path)
	if err == nil {
		t.Fatal("Read of malformed TOML = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the file %q", err, path)
	}
}

// §0.2 requires every codec to be a proven round-trip fixed point. For this
// one the property is slightly weaker than identity on arbitrary input —
// TOML has more than one spelling of the same fact, and Encode picks one — so
// what must hold is that encoding is idempotent from the second pass on, and
// that no key/value pair is lost along the way.
func FuzzFileEncodeIsAFixedPoint(f *testing.F) {
	for _, seed := range []string{
		"",
		"project.stale-after = 14\n",
		"emit.claude = true\nemit.claude-skills = \"copy\"\n",
		"[emit]\nclaude = true\n",
		"key-result.at-risk-pace = 0.8\n",
		"a = []\n",
		"z = \"last\"\na = \"first\"\n",
		"nested.deep.key = 1\nnested.deep.other = 2\n",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		first, err := Decode([]byte(src))
		if err != nil {
			t.Skip()
		}
		once, err := first.Encode()
		if err != nil {
			// Encode may legitimately refuse a key TOML admits but para's
			// writer does not (a quoted key, say); it must not lose one.
			t.Skip()
		}

		second, err := Decode(once)
		if err != nil {
			t.Fatalf("re-decoding para's own output failed: %v\noutput was %q", err, once)
		}
		twice, err := second.Encode()
		if err != nil {
			t.Fatalf("re-encoding para's own output failed: %v\noutput was %q", err, once)
		}
		if !bytes.Equal(once, twice) {
			t.Fatalf("encoding is not a fixed point:\nfirst:  %q\nsecond: %q", once, twice)
		}
		if !slices.Equal(first.Keys(), second.Keys()) {
			t.Fatalf("keys changed across a round trip: %v then %v", first.Keys(), second.Keys())
		}
	})
}

func mustSet(t *testing.T, f *File, key string, v ptoml.Value) bool {
	t.Helper()
	changed, err := f.Set(key, v)
	if err != nil {
		t.Fatalf("Set(%q): %v", key, err)
	}
	return changed
}

// TOML cannot hold `emit = "x"` and `emit.claude = true` at once: the first
// makes emit a value, the second makes it a table. Appending the second
// anyway produced a file nothing could parse — including the `config unset`
// that would have been the way out.
func TestSetRefusesAKeyThatCannotCoexistWithOneAlreadyThere(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		key      string
		wantName string
	}{
		{
			name:     "an existing scalar is the new key's table",
			src:      "emit = \"x\"\n",
			key:      "emit.claude",
			wantName: "emit",
		},
		{
			name:     "the new key is an existing key's table",
			src:      "review.cadence = 90\n",
			key:      "review",
			wantName: "review.cadence",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Decode([]byte(tt.src))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			changed, err := f.Set(tt.key, ptoml.Bool(true))
			if err == nil {
				out, _ := f.Encode()
				t.Fatalf("Set(%q) = %v, nil; want a refusal (it would have written %q)", tt.key, changed, out)
			}
			if !strings.Contains(err.Error(), tt.wantName) {
				t.Errorf("error %q does not name the key it collides with, %q", err, tt.wantName)
			}
			// The refusal leaves the file exactly as it was.
			got, encErr := f.Encode()
			if encErr != nil {
				t.Fatalf("Encode after a refused Set: %v", encErr)
			}
			if string(got) != tt.src {
				t.Errorf("a refused Set changed the file: %q, want %q", got, tt.src)
			}
		})
	}
}

// A key that cannot be written back is refused at the door, with a message
// naming it — rather than read happily and then lost, or duplicated, when
// the next `config set` rewrites the file.
func TestDecodeRefusesAKeyItCannotWriteBack(t *testing.T) {
	for _, src := range []string{
		"\"a.b\" = 1\n\n[a]\nb = 2\n", // two facts that flatten to one name
		"\"a.b\" = 1\n",
		"\"my key\" = 1\n",
		"\"café\" = 1\n",
	} {
		if _, err := Decode([]byte(src)); err == nil {
			t.Errorf("Decode(%q) = nil error, want a refusal", src)
		}
	}
}

// A float that is not a number would be written as Go spells it, which is
// not TOML, leaving a config.toml no reader can parse.
func TestNonFiniteFloatsSurviveEveryPath(t *testing.T) {
	// A hand-written one round-trips rather than corrupting the file, since
	// para may be rewriting the file for an unrelated key.
	f, err := Decode([]byte("some.knob = nan\n"))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := Decode(out); err != nil {
		t.Fatalf("para wrote a file it cannot read back: %q (%v)", out, err)
	}

	// But para's own knobs refuse one, because a threshold that is not a
	// number is a check that can never fire.
	spec, ok := Lookup("key-result.at-risk-pace")
	if !ok {
		t.Fatal("Lookup(key-result.at-risk-pace) not found")
	}
	for _, raw := range []string{"nan", "NaN", "inf", "+Inf", "-inf"} {
		if v, err := spec.Parse(raw); err == nil {
			t.Errorf("Parse(%q) = %v, want a refusal", raw, Format(v))
		}
	}
	if err := spec.Check(ptoml.Float64(math.NaN())); err == nil {
		t.Error("Check(NaN) = nil, want a refusal")
	}
	if err := spec.Check(ptoml.Float64(math.Inf(1))); err == nil {
		t.Error("Check(+Inf) = nil, want a refusal")
	}
}

// The no-op rule asks whether the file would change, which for floats is a
// question about bytes rather than about numbers.
func TestSetComparesFloatsByTheBytesTheyWrite(t *testing.T) {
	var f File
	mustSet(t, &f, "key-result.at-risk-pace", ptoml.Float64(0))
	if changed := mustSet(t, &f, "key-result.at-risk-pace", ptoml.Float64(math.Copysign(0, -1))); !changed {
		t.Error("setting -0.0 over 0.0 reported no change, but they write different bytes")
	}
	if changed := mustSet(t, &f, "key-result.at-risk-pace", ptoml.Float64(math.Copysign(0, -1))); changed {
		t.Error("setting -0.0 over -0.0 reported a change")
	}

	var g File
	mustSet(t, &g, "some.knob", ptoml.Float64(math.NaN()))
	if changed := mustSet(t, &g, "some.knob", ptoml.Float64(math.NaN())); changed {
		t.Error("setting NaN over NaN reported a change, which would rewrite the file on every run")
	}
}
