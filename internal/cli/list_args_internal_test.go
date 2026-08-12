package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// TestParseListArgs is R19's lookahead table, tested as a table: all five
// rows, plus the refusals R19/R20 name — a chain in the filter position,
// container as a filter, and four arguments — and the one shape R19's table
// leaves implicit: a noun with no bucket form (objective, key-result) named
// in the second, "bucket scope" position.
func TestParseListArgs(t *testing.T) {
	root := writeAddrFixture(t)
	cwd := root

	cases := []struct {
		name      string
		args      []string
		wantKind  kindmeta.Kind
		wantScope string // locator.String(), "" for the whole tree
		wantErr   string
	}{
		{
			name:     "no args is the whole tree",
			args:     nil,
			wantKind: kindmeta.KindUnknown,
		},
		{
			name:     "one noun, a bucket kind, is that kind tree-wide",
			args:     []string{"project"},
			wantKind: kindmeta.KindProject,
		},
		{
			name:     "one noun, a non-bucket kind, is that kind tree-wide",
			args:     []string{"key-result"},
			wantKind: kindmeta.KindKeyResult,
		},
		{
			name:      "noun noun is a filter plus a bucket scope",
			args:      []string{"key-result", "project"},
			wantKind:  kindmeta.KindKeyResult,
			wantScope: "projects",
		},
		{
			name:      "noun chain is a scope, no filter",
			args:      []string{"project", "acme"},
			wantScope: "projects.acme",
		},
		{
			name:      "noun noun chain is a filter plus a scope",
			args:      []string{"key-result", "project", "acme"},
			wantKind:  kindmeta.KindKeyResult,
			wantScope: "projects.acme",
		},
		{
			name:    "a chain in the filter position is refused as not a noun",
			args:    []string{"acme.q1-growth"},
			wantErr: "not a noun",
		},
		{
			name:    "container alone is not a legal filter (R20)",
			args:    []string{"container"},
			wantErr: "not a legal filter",
		},
		{
			name:    "container as a filter with a bucket scope is still refused",
			args:    []string{"container", "project"},
			wantErr: "not a legal filter",
		},
		{
			name:    "container as a filter with a scope is still refused",
			args:    []string{"container", "project", "acme"},
			wantErr: "not a legal filter",
		},
		{
			name:    "four arguments is refused",
			args:    []string{"key-result", "project", "acme", "extra"},
			wantErr: "at most",
		},
		{
			name:    "a noun, a non-noun second argument, and a third is refused",
			args:    []string{"project", "acme", "extra"},
			wantErr: "not a noun",
		},
		{
			// objective has no bucket form (R4: exactly two segments, never
			// empty) — naming it as the second, "bucket scope" position falls
			// through to the same arity refusal a bare `list objective` on
			// its own chain would give, rather than a nonsensical bucket.
			name:    "a noun with no bucket form in the bucket-scope position is an arity refusal",
			args:    []string{"key-result", "objective"},
			wantErr: "objective takes exactly two segments",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseListArgs(root, cwd, c.args, false)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("parseListArgs(%v) = %+v, want error containing %q", c.args, got, c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("parseListArgs(%v) error = %q, want it to contain %q", c.args, err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseListArgs(%v): %v", c.args, err)
			}
			if got.Kind != c.wantKind {
				t.Errorf("parseListArgs(%v).Kind = %v, want %v", c.args, got.Kind, c.wantKind)
			}
			if got.Scope.String() != c.wantScope {
				t.Errorf("parseListArgs(%v).Scope = %q, want %q", c.args, got.Scope.String(), c.wantScope)
			}
		})
	}
}

// TestParseListArgsDot is R16's "." convenience, restored for `list`: it
// stands for a whole scope as one token, resolved by walking up from cwd to
// the nearest .para/state.toml, and it is refused with anything after it
// rather than guessed at, since `list` gives it no second, kind-filter slot.
func TestParseListArgsDot(t *testing.T) {
	root := writeAddrFixture(t)
	cwd := filepath.Join(root, "projects", "acme")

	got, err := parseListArgs(root, cwd, []string{"."}, false)
	if err != nil {
		t.Fatalf("parseListArgs(.): %v", err)
	}
	if got.Kind != kindmeta.KindUnknown {
		t.Errorf("parseListArgs(.).Kind = %v, want no filter", got.Kind)
	}
	if want := "projects.acme"; got.Scope.String() != want {
		t.Errorf("parseListArgs(.).Scope = %q, want %q", got.Scope.String(), want)
	}

	if _, err := parseListArgs(root, cwd, []string{".", "extra"}, false); err == nil {
		t.Fatal(`parseListArgs(., extra): want a refusal, got none`)
	} else if !strings.Contains(err.Error(), "takes no further argument") {
		t.Errorf("parseListArgs(., extra) error = %q, want it to contain %q", err.Error(), "takes no further argument")
	}

	// archived is ignored in the dot branch, the same way parseAddressArgs'
	// own dot branch ignores it: cwd already names an archived or live place,
	// not an independent qualifier layered on top of one.
	archivedGot, err := parseListArgs(root, cwd, []string{"."}, true)
	if err != nil {
		t.Fatalf("parseListArgs(., archived): %v", err)
	}
	if want := "projects.acme"; archivedGot.Scope.String() != want {
		t.Errorf("parseListArgs(., archived).Scope = %q, want %q (archived should be ignored)", archivedGot.Scope.String(), want)
	}
}

// TestParseListArgsArchived is R7 threaded through the two rows a chain
// carries archived on its own (chainToLocator does the work there), plus the
// two rows that never call chainToLocator at all and so need archived
// applied directly: no args, and one noun alone.
func TestParseListArgsArchived(t *testing.T) {
	root := writeAddrFixture(t)

	cases := []struct {
		name      string
		args      []string
		wantKind  kindmeta.Kind
		wantScope string
	}{
		{
			name:      "no args, archived, is the whole archive",
			args:      nil,
			wantScope: "archive",
		},
		{
			name:      "one noun, archived, redirects the tree-wide walk into the archive",
			args:      []string{"project"},
			wantKind:  kindmeta.KindProject,
			wantScope: "archive",
		},
		{
			name:      "noun and chain, archived, prepends the place as chainToLocator always has",
			args:      []string{"project", "acme"},
			wantScope: "archive.projects.acme",
		},
		{
			name:      "kind filter plus bucket scope, archived",
			args:      []string{"key-result", "project"},
			wantKind:  kindmeta.KindKeyResult,
			wantScope: "archive.projects",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseListArgs(root, root, c.args, true)
			if err != nil {
				t.Fatalf("parseListArgs(%v, archived): %v", c.args, err)
			}
			if got.Kind != c.wantKind {
				t.Errorf("parseListArgs(%v, archived).Kind = %v, want %v", c.args, got.Kind, c.wantKind)
			}
			if got.Scope.String() != c.wantScope {
				t.Errorf("parseListArgs(%v, archived).Scope = %q, want %q", c.args, got.Scope.String(), c.wantScope)
			}
		})
	}
}
