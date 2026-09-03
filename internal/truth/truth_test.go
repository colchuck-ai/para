package truth_test

import (
	"os"
	"path/filepath"
	"strings"
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

// TestEncodeStateWritesAttentionAndSuppression pins §28.4's worked example
// shape: attention lands after every §15 field, and [suppression] is a
// separate table after a blank line.
func TestEncodeStateWritesAttentionAndSuppression(t *testing.T) {
	s := truth.State{
		Name:      "Acme migration",
		Status:    "in-progress",
		Priority:  "high",
		Due:       "2026-09-30",
		Tags:      []string{"kafka", "consumer"},
		Created:   "2026-01-01T16:15:00Z",
		Attention: "2026-08-20T10:00:00Z",
		Suppression: truth.Suppression{
			Until: "2027-03-01",
			Note:  "paused, resumes with Q1 relaunch",
		},
	}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	want := `name = "Acme migration"
status = "in-progress"
priority = "high"
due = "2026-09-30"
tags = ["kafka", "consumer"]
created = "2026-01-01T16:15:00Z"
attention = "2026-08-20T10:00:00Z"

[suppression]
until = "2027-03-01"
note = "paused, resumes with Q1 relaunch"
`
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}
}

// TestEncodeStateOmitsSuppressionWithEmptyUntil pins §28.4's "presence of the
// table is the signal": an unsuppressed entity (or one never suppressed)
// writes no [suppression] table at all, never one with an empty until.
func TestEncodeStateOmitsSuppressionWithEmptyUntil(t *testing.T) {
	s := truth.State{Name: "Acme migration", Attention: "2026-08-20T10:00:00Z"}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	if strings.Contains(string(got), "suppression") {
		t.Errorf("EncodeState wrote a [suppression] table with no active suppression: %s", got)
	}
}

// TestEncodeStateDoesNotDropAnOrphanedSuppressionNote is a data-loss
// regression: a hand-edited state.toml can decode to a Suppression with a
// Note but no Until (or vice versa), a shape the write-through derivation
// never produces but EncodeState must not silently discard the next time
// anything rewrites the file (§19: hand-editing truth is legal, and nothing
// in it is ever corrupted or lost).
func TestEncodeStateDoesNotDropAnOrphanedSuppressionNote(t *testing.T) {
	s := truth.State{Name: "Acme migration", Suppression: truth.Suppression{Note: "orphaned note, no until"}}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	want := "name = \"Acme migration\"\n\n[suppression]\nnote = \"orphaned note, no until\"\n"
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}

	back, err := truth.DecodeState(got)
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if back.Suppression.Note != "orphaned note, no until" || back.Suppression.Until != "" {
		t.Errorf("round trip = %+v, want the note preserved with no until", back.Suppression)
	}
}

// TestEncodeStateOmitsAttentionAndSuppressionWhenUncached is the pre-§28
// case: a State with neither field set writes neither key, matching a
// state.toml written before §28 existed.
func TestEncodeStateOmitsAttentionAndSuppressionWhenUncached(t *testing.T) {
	s := truth.State{Name: "Acme migration", Status: "in-progress"}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	want := "name = \"Acme migration\"\nstatus = \"in-progress\"\n"
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}
}

// TestDecodeState_AttentionAndSuppressionAbsentIsNotYetCached is §28.4's
// backward-compatibility case: a pre-§28 state.toml has neither key, and
// decoding it must leave both fields at their zero value rather than
// synthesizing false/zero — the read path is what interprets that as "not
// yet cached" and falls back to a live journal derivation.
func TestDecodeState_AttentionAndSuppressionAbsentIsNotYetCached(t *testing.T) {
	s, err := truth.DecodeState([]byte("name = \"Acme migration\"\nstatus = \"in-progress\"\n"))
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if s.Attention != "" {
		t.Errorf("Attention = %q, want empty (not yet cached)", s.Attention)
	}
	if s.Suppression != (truth.Suppression{}) {
		t.Errorf("Suppression = %+v, want the zero value (not yet cached)", s.Suppression)
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
		"attention and suppression": `name = "Acme migration"
status = "in-progress"
created = "2026-01-01T08:15:00-08:00"
attention = "2026-08-20T10:00:00Z"

[suppression]
until = "2027-03-01"
note = "paused, resumes with Q1 relaunch"
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

// TestAttentionAndSuppressionAreNotSettableFields pins §28.4's "only ever
// written by the derivation step, never by set/unset": neither is a
// kindmeta.Field, so they cannot appear in the set of fields `set`/`unset`
// validate against, and State.SetField — the one function those verbs call —
// cannot reach either struct field even if handed the bare key by name.
func TestAttentionAndSuppressionAreNotSettableFields(t *testing.T) {
	for _, f := range kindmeta.AllFields() {
		if string(f) == "attention" || string(f) == "suppression" {
			t.Errorf("kindmeta.AllFields() contains %q, want attention/suppression unreachable from set/unset", f)
		}
	}
	s := truth.State{}
	s.SetField(kindmeta.Field("attention"), "2026-01-01T00:00:00Z")
	if s.Attention != "" {
		t.Errorf("SetField(%q, ...) wrote through to State.Attention, want it unreachable", "attention")
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

func TestEncodeStateWritesKindFirst(t *testing.T) {
	// kind leads the file (§8.3): it is the one field that has to be readable
	// before anything else in the file means anything, since `type` and
	// `start` are interpretable only once you know you are reading a
	// key-result.
	s := truth.State{
		Kind:    "key-result",
		Name:    "Weekly signups",
		Type:    "ratio",
		Created: "2026-01-01T08:15:00-08:00",
	}

	got, err := truth.EncodeState(s)
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}

	want := `kind = "key-result"
name = "Weekly signups"
created = "2026-01-01T08:15:00-08:00"
type = "ratio"
`
	if string(got) != want {
		t.Errorf("EncodeState =\n%s\nwant\n%s", got, want)
	}
}

func TestStateKindRoundTrips(t *testing.T) {
	for _, k := range kindmeta.AllStorableKinds() {
		data, err := truth.EncodeState(truth.State{Kind: k.String(), Name: "x"})
		if err != nil {
			t.Fatalf("EncodeState(%s): %v", k, err)
		}
		got, err := truth.DecodeState(data)
		if err != nil {
			t.Fatalf("DecodeState(%s): %v", k, err)
		}
		if got.Kind != k.String() {
			t.Errorf("round-trip kind = %q, want %q", got.Kind, k.String())
		}
	}
}

func TestDecodeStateAbsentKindIsEmpty(t *testing.T) {
	// A state.toml written before §30 has no kind key at all. That must decode
	// to "" — distinguishable from any legal value — rather than to a word
	// that would read as a real answer.
	s, err := truth.DecodeState([]byte("name = \"Health\"\n"))
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if s.Kind != "" {
		t.Errorf("Kind = %q, want \"\" for an absent key", s.Kind)
	}
}

func TestEncodeStatePreservesUnrecognisedKind(t *testing.T) {
	// EncodeState's contract is to lose nothing it is handed (§19), the same
	// reason a Suppression Note with no Until still round-trips. A hand-edited
	// `kind = "banana"` is doctor's finding to report, not this function's to
	// silently delete — deleting it would turn a visible error into an absent
	// key, which is a different and quieter one.
	data, err := truth.EncodeState(truth.State{Kind: "banana", Name: "x"})
	if err != nil {
		t.Fatalf("EncodeState: %v", err)
	}
	if !strings.Contains(string(data), `kind = "banana"`) {
		t.Errorf("EncodeState dropped an unrecognised kind:\n%s", data)
	}
	got, err := truth.DecodeState(data)
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if got.Kind != "banana" {
		t.Errorf("Kind = %q, want it preserved as \"banana\"", got.Kind)
	}
}

func TestKindIsNotAFieldSetOrUnsetCanReach(t *testing.T) {
	// There is no --kind flag (§0 principle 1): an address carries a noun and
	// the noun is the kind. set and unset find a field by kindmeta.Field, so
	// kind's absence from that vocabulary is what puts it out of their reach —
	// the same device Attention and Suppression use.
	for _, f := range kindmeta.AllFields() {
		if string(f) == "kind" {
			t.Fatal("kind became a kindmeta.Field, which puts it in reach of set/unset")
		}
	}
}
