package truth_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/testutil"
	"github.com/colchuck-ai/para/internal/truth"
)

func TestEncodeStateWritesFieldsInMatrixOrder(t *testing.T) {
	// §15's row order, not the order the struct's fields happen to be
	// assigned in, and not map order (§0.2).
	s := truth.State{
		Target:      "2000/12000",
		Created:     "2026-01-01T08:15:00-08:00",
		Name:        "Weekly signups",
		Type:        "ratio",
		Start:       "480/9000",
		Due:         "2026-09-30",
		Tags:        []string{"growth", "funnel"},
		Description: "Move the top of the funnel.",
	}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	want := `name = "Weekly signups"
description = "Move the top of the funnel."
due = "2026-09-30"
tags = ["growth", "funnel"]
created = "2026-01-01T08:15:00-08:00"
type = "ratio"
start = "480/9000"
target = "2000/12000"
`
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeStateOmitsAbsentFields(t *testing.T) {
	// A container's state.toml is name, description, created and nothing
	// else (§8.2) — an empty string or empty slice is absence, not a value.
	s := truth.State{
		Name:        "Objectives",
		Description: "What acme-migration is trying to move.",
		Created:     "2026-01-01T08:15:00-08:00",
		Tags:        []string{},
		Scope:       []string{},
	}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	want := `name = "Objectives"
description = "What acme-migration is trying to move."
created = "2026-01-01T08:15:00-08:00"
`
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}
}

func TestEncodeStateIsByteStableAcrossCalls(t *testing.T) {
	s := truth.State{
		Name:  "Signups report",
		Scope: []string{"project.acme-migration", "area.growth"},
		Tags:  []string{"growth", "reporting"},
	}

	first, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	second, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("two encodes differ:\n%s\nvs\n%s", first, second)
	}
}

func TestStateRoundTripIsAFixedPoint(t *testing.T) {
	cases := map[string]string{
		"project": `name = "Acme migration"
description = "Rebuild the consumer."
status = "in-progress"
priority = "high"
due = "2026-09-30"
tags = ["kafka", "consumer"]
created = "2026-01-01T08:15:00-08:00"
`,
		"key-result": `name = "Weekly signups"
created = "2026-01-01T08:15:00-08:00"
type = "ratio"
start = "480/9000"
target = "2000/12000"
`,
		"skill": `name = "Signups report"
description = "when asked for the weekly signups number"
tags = ["growth"]
created = "2026-01-01T08:15:00-08:00"
scope = ["project.acme-migration", "area.growth"]
`,
		"container": `name = "Objectives"
description = "What acme-migration is trying to move."
created = "2026-01-01T08:15:00-08:00"
`,
	}

	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			testutil.AssertRoundTrip(t, []byte(data), truth.DecodeState, truth.EncodeState)
		})
	}
}

func TestDecodeStateRejectsInvalidTOML(t *testing.T) {
	if _, err := truth.DecodeState([]byte("name = \n")); err == nil {
		t.Error("DecodeState accepted invalid TOML, want an error")
	}
}

func TestDecodeStateIgnoresUnknownKeys(t *testing.T) {
	// A key para does not know is not para's to delete on the spot; doctor
	// reports the file as invalid (§10). Decoding must not fail on it.
	s, err := truth.DecodeState([]byte("name = \"X\"\nsomething-else = 1\n"))
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if s.Name != "X" {
		t.Errorf("Name = %q, want %q", s.Name, "X")
	}
}

func TestStateFieldRoundTripsThroughFieldAccessor(t *testing.T) {
	// Field/SetField are what set/unset and --sort address a state by name
	// (§15's "one spelling per field").
	s := truth.State{}
	for _, f := range kindmeta.AllFields() {
		if f == kindmeta.FieldTags || f == kindmeta.FieldScope {
			continue
		}
		s.SetField(f, "v-"+string(f))
	}
	for _, f := range kindmeta.AllFields() {
		if f == kindmeta.FieldTags || f == kindmeta.FieldScope {
			continue
		}
		if got := s.Field(f); got != "v-"+string(f) {
			t.Errorf("Field(%q) = %q, want %q", f, got, "v-"+string(f))
		}
	}
}

func TestEncodeTreeWritesTheRootMarkerInSchemaOrder(t *testing.T) {
	tr := truth.Tree{
		Schema:      1,
		ParaVersion: "0.4.0",
		Name:        "max's brain",
		Description: "Everything I am carrying.",
		Created:     "2026-01-01T08:00:00-08:00",
	}

	got, err := truth.EncodeTree(tr)
	if err != nil {
		t.Fatalf("EncodeTree: %v", err)
	}

	want := `schema = 1
para-version = "0.4.0"
name = "max's brain"
description = "Everything I am carrying."
created = "2026-01-01T08:00:00-08:00"
`
	if string(got) != want {
		t.Errorf("EncodeTree =\n%s\nwant\n%s", got, want)
	}
}

func TestTreeRoundTripIsAFixedPoint(t *testing.T) {
	data := []byte(`schema = 1
para-version = "0.4.0"
name = "max's brain"
description = "Everything I am carrying."
created = "2026-01-01T08:00:00-08:00"
`)
	testutil.AssertRoundTrip(t, data, truth.DecodeTree, truth.EncodeTree)
}

func TestReadStateReadsFromTheParaDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".para", "state.toml"), []byte("name = \"Health\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := truth.ReadState(dir)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if s.Name != "Health" {
		t.Errorf("Name = %q, want %q", s.Name, "Health")
	}
}

func TestReadStateOnAMissingFileIsNotFound(t *testing.T) {
	if _, err := truth.ReadState(t.TempDir()); err == nil {
		t.Error("ReadState on a directory with no .para/state.toml returned no error")
	}
}

func TestReadTreeReadsTheRootMarker(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".para", "tree.toml"), []byte("schema = 1\nname = \"brain\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tr, err := truth.ReadTree(dir)
	if err != nil {
		t.Fatalf("ReadTree: %v", err)
	}
	if tr.Schema != 1 || tr.Name != "brain" {
		t.Errorf("ReadTree = %+v, want schema 1 and name brain", tr)
	}
}

func TestStatePathAndTreePathNameTheUniformFilenames(t *testing.T) {
	// §8.4: state.toml and config.toml in every .para/, for every kind; the
	// single non-uniform file is tree.toml at the root.
	if got, want := truth.StatePath("x"), filepath.Join("x", ".para", "state.toml"); got != want {
		t.Errorf("StatePath = %q, want %q", got, want)
	}
	if got, want := truth.ConfigPath("x"), filepath.Join("x", ".para", "config.toml"); got != want {
		t.Errorf("ConfigPath = %q, want %q", got, want)
	}
	if got, want := truth.TreePath("x"), filepath.Join("x", ".para", "tree.toml"); got != want {
		t.Errorf("TreePath = %q, want %q", got, want)
	}
	if got, want := truth.LogsDir("x"), filepath.Join("x", ".para", "logs"); got != want {
		t.Errorf("LogsDir = %q, want %q", got, want)
	}
}
