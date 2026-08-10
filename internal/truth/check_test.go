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
