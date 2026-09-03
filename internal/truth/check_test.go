package truth_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/truth"
)

// TestCheck walks §15's rules over stored states — the set doctor's `invalid`
// finding reports (§10). Each case names the fields it expects to be faulted,
// so a rule that fires on the wrong field fails as loudly as one that does not
// fire at all.
func TestCheck(t *testing.T) {
	cases := []struct {
		name  string
		kind  kindmeta.Kind
		state truth.State
		want  []kindmeta.Field
	}{
		{
			name:  "a complete project passes",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Status: "in-progress", Priority: "high", Due: "2026-09-30", Created: "2026-01-01T00:00:00Z"},
		},
		{
			name:  "a complete key-result passes",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "ratio", Start: "480/9000", Target: "2000/12000"},
		},
		{
			name:  "a complete skill passes",
			kind:  kindmeta.KindSkill,
			state: truth.State{Name: "Report", Description: "when asked for signups", Scope: []string{"project"}},
		},
		{
			name:  "a complete container passes",
			kind:  kindmeta.KindContainer,
			state: truth.State{Name: "Objectives", Description: "What acme is trying to move."},
		},
		{
			// para-6g7: a link's type is an opaque tag, not key-result's
			// closed measurement grammar — "jira-epic" must not be refused
			// the way it would be as a key-result's type.
			name:  "a complete link passes",
			kind:  kindmeta.KindLink,
			state: truth.State{Type: "jira-epic", Ref: "PROJ-123", Direction: "output"},
		},
		{
			name:  "a link direction outside the closed set",
			kind:  kindmeta.KindLink,
			state: truth.State{Type: "jira-epic", Ref: "PROJ-123", Direction: "sideways"},
			want:  []kindmeta.Field{kindmeta.FieldDirection},
		},
		{
			// para-nd3's breaking change: "both" was a legal link direction
			// in v0.5.0 (para-6g7) and no longer is — removed so every link
			// has exactly one attention clock and one link.stale-after key
			// applies to it, with no exception in the model. A state.toml
			// carried over from before this change must be refused the same
			// way any other unrecognised value already is, not silently
			// accepted.
			name:  "a link direction of the removed value both",
			kind:  kindmeta.KindLink,
			state: truth.State{Type: "jira-epic", Ref: "PROJ-123", Direction: "both"},
			want:  []kindmeta.Field{kindmeta.FieldDirection},
		},
		{
			name:  "a project with no name or description",
			kind:  kindmeta.KindProject,
			state: truth.State{Status: "planned"},
			want:  []kindmeta.Field{kindmeta.FieldName, kindmeta.FieldDescription},
		},
		{
			// §10 names this one specially: a skill with no description "would
			// be inert", since the description is the whole of when to use it.
			name:  "a skill with no description",
			kind:  kindmeta.KindSkill,
			state: truth.State{Name: "Report"},
			want:  []kindmeta.Field{kindmeta.FieldDescription},
		},
		{
			name:  "a key-result with no type or target",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups"},
			want:  []kindmeta.Field{kindmeta.FieldType, kindmeta.FieldTarget},
		},
		{
			name:  "a status the kind does not admit",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Status: "shipped"},
			want:  []kindmeta.Field{kindmeta.FieldStatus},
		},
		{
			// §4.3: a key-result's status is derived, except for the one
			// settable value carved out of it.
			name:  "a key-result status that is not dropped",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "number", Target: "100", Status: "achieved"},
			want:  []kindmeta.Field{kindmeta.FieldStatus},
		},
		{
			name:  "a field the kind does not have at all",
			kind:  kindmeta.KindArea,
			state: truth.State{Name: "Health", Description: "Staying alive.", Status: "in-progress", Due: "2026-09-30"},
			want:  []kindmeta.Field{kindmeta.FieldStatus, kindmeta.FieldDue},
		},
		{
			name:  "a priority outside the closed set",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Priority: "urgent"},
			want:  []kindmeta.Field{kindmeta.FieldPriority},
		},
		{
			name:  "a key-result type outside the grammar",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "percentage", Target: "100"},
			want:  []kindmeta.Field{kindmeta.FieldType},
		},
		{
			name:  "a target that contradicts the type",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "boolean", Target: "2000/12000"},
			want:  []kindmeta.Field{kindmeta.FieldTarget},
		},
		{
			name:  "a start that contradicts the type",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "number", Start: "yes", Target: "100"},
			want:  []kindmeta.Field{kindmeta.FieldStart},
		},
		{
			name:  "a start equal to its target",
			kind:  kindmeta.KindKeyResult,
			state: truth.State{Name: "Signups", Type: "number", Start: "100", Target: "100"},
			want:  []kindmeta.Field{kindmeta.FieldStart},
		},
		{
			// The Phase 10/11 debt: view treats a created it cannot parse as
			// absent, on the grounds that this is doctor's to report.
			name:  "a created that will not parse",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Created: "last tuesday"},
			want:  []kindmeta.Field{kindmeta.FieldCreated},
		},
		{
			name:  "a due that will not parse",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Due: "soon"},
			want:  []kindmeta.Field{kindmeta.FieldDue},
		},
		{
			name:  "a created after its due",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Created: "2026-10-01T00:00:00Z", Due: "2026-09-30"},
			want:  []kindmeta.Field{kindmeta.FieldCreated},
		},
		{
			name:  "a tag that is an operator in a --tags expression",
			kind:  kindmeta.KindProject,
			state: truth.State{Name: "Acme", Description: "Rebuild.", Tags: []string{"and"}},
			want:  []kindmeta.Field{kindmeta.FieldTags},
		},
		{
			name:  "a scope entry in the old plural locator form is not a noun",
			kind:  kindmeta.KindSkill,
			state: truth.State{Name: "Report", Description: "when asked", Scope: []string{"projects.acme"}},
			want:  []kindmeta.Field{kindmeta.FieldScope},
		},
		{
			name:  "a scope entry with the wrong arity for its noun",
			kind:  kindmeta.KindSkill,
			state: truth.State{Name: "Report", Description: "when asked", Scope: []string{"objective.acme"}},
			want:  []kindmeta.Field{kindmeta.FieldScope},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truth.Check(c.kind, c.state)
			var fields []kindmeta.Field
			for _, p := range got {
				fields = append(fields, p.Field)
				if strings.TrimSpace(p.Msg) == "" {
					t.Errorf("problem on %q has no message", p.Field)
				}
			}
			if len(fields) != len(c.want) {
				t.Fatalf("Check faulted %v, want %v", fields, c.want)
			}
			for i := range fields {
				if fields[i] != c.want[i] {
					t.Fatalf("Check faulted %v, want %v", fields, c.want)
				}
			}
		})
	}
}

// TestCheckScopeMessageNamesTheNoun is P18.1's acceptance criterion: a
// malformed scope entry is refused with a message naming the noun, not the
// old "is not a locator" wording.
func TestCheckScopeMessageNamesTheNoun(t *testing.T) {
	cases := []struct {
		name  string
		entry string
		want  string
	}{
		{
			name:  "old plural form names the legal nouns",
			entry: "projects.acme",
			want:  "is not a noun",
		},
		{
			name:  "wrong arity names the noun that rejected it",
			entry: "objective.acme",
			want:  "objective takes exactly two segments",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truth.Check(kindmeta.KindSkill, truth.State{
				Name: "Report", Description: "when asked", Scope: []string{c.entry},
			})
			if len(got) != 1 {
				t.Fatalf("Check faulted %d problems, want 1", len(got))
			}
			if strings.Contains(got[0].Msg, "is not a locator") {
				t.Errorf("Msg = %q, still names a locator rather than a noun", got[0].Msg)
			}
			if !strings.Contains(got[0].Msg, c.want) {
				t.Errorf("Msg = %q, want it to contain %q", got[0].Msg, c.want)
			}
		})
	}
}

// TestCheckOrderIsTheFieldMatrixOrder pins the report order to §15's rows, so
// two runs over one broken state name the same problem first (§0.2).
func TestCheckOrderIsTheFieldMatrixOrder(t *testing.T) {
	st := truth.State{Status: "shipped", Priority: "urgent", Created: "never"}
	got := truth.Check(kindmeta.KindProject, st)

	want := []kindmeta.Field{
		kindmeta.FieldName, kindmeta.FieldDescription,
		kindmeta.FieldStatus, kindmeta.FieldPriority, kindmeta.FieldCreated,
	}
	if len(got) != len(want) {
		t.Fatalf("Check returned %d problems, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Field != want[i] {
			t.Fatalf("problem %d is %q, want %q", i, got[i].Field, want[i])
		}
	}
}

// TestUnknownStateKeys_AttentionAndSuppressionAreKnown pins §28.4: attention
// and [suppression] are a legitimate part of state.toml's schema, generated
// rather than typed, so doctor's `invalid` finding must not flag a
// suppressed or attention-cached entity's state.toml as carrying stray keys.
func TestUnknownStateKeys_AttentionAndSuppressionAreKnown(t *testing.T) {
	data := []byte(`name = "Acme migration"
attention = "2026-08-20T10:00:00Z"

[suppression]
until = "2027-03-01"
note = "paused, resumes with Q1 relaunch"
`)
	got, err := truth.UnknownStateKeys(data)
	if err != nil {
		t.Fatalf("UnknownStateKeys: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("UnknownStateKeys = %v, want none", got)
	}
}

// TestUnknownStateKeys_StillCatchesATypo proves the widened known-key set
// does not swallow a genuine stray key.
func TestUnknownStateKeys_StillCatchesATypo(t *testing.T) {
	got, err := truth.UnknownStateKeys([]byte("name = \"X\"\ndescripton = \"typo\"\n"))
	if err != nil {
		t.Fatalf("UnknownStateKeys: %v", err)
	}
	if len(got) != 1 || got[0] != "descripton" {
		t.Errorf("UnknownStateKeys = %v, want [descripton]", got)
	}
}

// TestCheckTree covers the root, whose identity lives in tree.toml rather than
// state.toml (§8.1) and whose required fields are therefore its own.
func TestCheckTree(t *testing.T) {
	cases := []struct {
		name string
		tree truth.Tree
		want int
	}{
		{"a complete tree.toml passes", truth.Tree{Schema: 1, Name: "brain", Created: "2026-01-01T00:00:00Z"}, 0},
		{"no schema", truth.Tree{Name: "brain"}, 1},
		{"no name", truth.Tree{Schema: 1}, 1},
		{"a schema from the future", truth.Tree{Schema: 99, Name: "brain"}, 1},
		{"a created that will not parse", truth.Tree{Schema: 1, Name: "brain", Created: "whenever"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := truth.CheckTree(c.tree); len(got) != c.want {
				t.Fatalf("CheckTree returned %d problems, want %d: %v", len(got), c.want, got)
			}
		})
	}
}

func TestUnknownStateKeysAcceptsKind(t *testing.T) {
	// kind is not a §15 field — no kindmeta.Field names it — but it is a
	// legitimate part of state.toml's schema as of §30. Flagging it would make
	// every entity in a migrated tree read as `invalid`.
	got, err := truth.UnknownStateKeys([]byte("kind = \"area\"\nname = \"Health\"\n"))
	if err != nil {
		t.Fatalf("UnknownStateKeys: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("UnknownStateKeys = %v, want none", got)
	}
}

func TestCheckRejectsUnrecognisedKind(t *testing.T) {
	// A hand-edited or newer-para value outside the vocabulary. It is reported
	// against no field, because kind is not a §15 field: it is a rule about
	// the state as a whole.
	probs := truth.Check(kindmeta.KindArea, truth.State{Kind: "banana", Name: "Health"})
	var found *truth.Problem
	for i := range probs {
		if strings.Contains(probs[i].Msg, "banana") {
			found = &probs[i]
		}
	}
	if found == nil {
		t.Fatalf("Check did not fault kind = \"banana\"; got %v", probs)
	}
	if found.Field != "" {
		t.Errorf("Problem.Field = %q, want \"\" — kind is not a §15 field", found.Field)
	}
	if !strings.Contains(found.Msg, "kind") {
		t.Errorf("Problem.Msg = %q, want it to name the key", found.Msg)
	}
}

func TestCheckAcceptsEveryKindWordAsItsOwnStoredKind(t *testing.T) {
	// Every word ParseKind accepts must be storable, including container —
	// a container has its own state.toml (§8.2) and so its own stored kind.
	//
	// Each word is checked against its OWN kind rather than against a fixed
	// one. Passing kindmeta.KindArea for all of them would assert that
	// `kind = "skill"` under an area is clean, which pins the absence of
	// §10's `misplaced` check (§30.4, para-sxt.9) as though it were intended.
	for _, k := range kindmeta.AllStorableKinds() {
		probs := truth.Check(k, truth.State{Kind: k.String(), Name: "Health"})
		for _, p := range probs {
			if strings.Contains(p.Msg, "kind") {
				t.Errorf("Check faulted a legal stored kind %q: %s", k.String(), p.Msg)
			}
		}
	}
}

func TestMistypedStateKeys(t *testing.T) {
	// A known key holding the wrong TOML type is invisible to every typed
	// accessor on ptoml.Document — they report a mismatch as absence — so
	// without this check `kind = 5` decodes to no kind and the file reads
	// clean. Before kind became a known key it was caught as an unknown one;
	// this is the coverage that keeps.
	cases := []struct {
		name string
		toml string
		want []string
	}{
		{"kind as int", "kind = 5\nname = \"x\"\n", []string{"kind"}},
		{"kind as bool", "kind = true\n", []string{"kind"}},
		{"kind as array", "kind = [\"area\"]\n", []string{"kind"}},
		// Every TOML type, not just the ones ptoml.Value can represent. A
		// bare date is the likeliest hand-edit of `due` or `created`, and a
		// number inside an array the likeliest of `tags` — both were silent
		// while presence was probed with Value instead of Keys.
		{"kind as bare date", "kind = 2026-01-01\n", []string{"kind"}},
		{"kind as datetime", "kind = 2026-01-01T00:00:00Z\n", []string{"kind"}},
		{"kind as int array", "kind = [1]\n", []string{"kind"}},
		{"due as bare date", "due = 2026-06-01\n", []string{"due"}},
		{"created as bare datetime", "created = 2026-03-05T17:00:00Z\n", []string{"created"}},
		{"tags as int array", "tags = [1, 2]\n", []string{"tags"}},
		{"scope as int array", "scope = [1]\n", []string{"scope"}},
		{"mixed array", "tags = [\"a\", 1]\n", []string{"tags"}},
		{"name as int", "kind = \"area\"\nname = 5\n", []string{"name"}},
		{"tags as string", "tags = \"growth\"\n", []string{"tags"}},
		{"attention as int", "attention = 5\n", []string{"attention"}},
		{"well typed", "kind = \"area\"\nname = \"x\"\ntags = [\"a\"]\n", nil},
		{"absent keys are not mistyped", "name = \"x\"\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := truth.MistypedStateKeys([]byte(c.toml))
			if err != nil {
				t.Fatalf("MistypedStateKeys: %v", err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("MistypedStateKeys = %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("MistypedStateKeys[%d] = %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestMistypedKindIsNotAlsoAnUnknownKey(t *testing.T) {
	// The two checks must not double-report: kind is a known key whatever it
	// holds, so a mistyped one is exactly one finding, not two.
	data := []byte("kind = 5\n")
	unknown, err := truth.UnknownStateKeys(data)
	if err != nil {
		t.Fatalf("UnknownStateKeys: %v", err)
	}
	if len(unknown) != 0 {
		t.Errorf("UnknownStateKeys = %v, want none — kind is a known key", unknown)
	}
}
