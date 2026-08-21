package view

import (
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
)

func mustLoc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// TestReaches is §5.2's "an entry covers its locator and everything beneath
// it", exercised as a table over the stored dotted-address form (R24) rather
// than the internal Locator string a scope entry used to be.
func TestReaches(t *testing.T) {
	cases := []struct {
		name  string
		scope []string
		loc   string
		want  bool
		via   string
	}{
		{
			name:  "an empty scope is the whole tree",
			scope: nil,
			loc:   "areas.health",
			want:  true,
		},
		{
			name:  "an entry naming the entity itself",
			scope: []string{"area.health"},
			loc:   "areas.health",
			want:  true,
			via:   "area.health",
		},
		{
			name:  "an entry naming an ancestor",
			scope: []string{"area.health"},
			loc:   "areas.health.training",
			want:  true,
			via:   "area.health",
		},
		{
			// The regression case: a project's address does not string-prefix
			// its own objective's ("project.acme" is not a prefix of
			// "objective.acme.q1-growth" — the structural word "objectives"
			// the address form drops), yet a project-scoped skill must still
			// reach its objectives and key-results.
			name:  "a project entry reaches its own objective, crossing a noun boundary",
			scope: []string{"project.acme"},
			loc:   "projects.acme.objectives.q1-growth",
			want:  true,
			via:   "project.acme",
		},
		{
			name:  "a project entry reaches its own key-result",
			scope: []string{"project.acme"},
			loc:   "projects.acme.objectives.q1-growth.key-results.signups",
			want:  true,
			via:   "project.acme",
		},
		{
			name:  "a sibling whose name merely starts the same way does not match",
			scope: []string{"area.health"},
			loc:   "areas.health-and-safety",
			want:  false,
		},
		{
			name:  "an unrelated entity does not match",
			scope: []string{"area.fitness"},
			loc:   "areas.health",
			want:  false,
		},
		{
			name:  "a bucket entry reaches everything of that noun",
			scope: []string{"project"},
			loc:   "projects.acme",
			want:  true,
			via:   "project",
		},
		{
			name:  "an entry that does not parse as an address is skipped, not matched",
			scope: []string{"Not An Address"},
			loc:   "areas.health",
			want:  false,
		},
		{
			name:  "the old plural-locator form no longer parses and so does not match",
			scope: []string{"areas.health"},
			loc:   "areas.health",
			want:  false,
		},
		{
			name:  "the first covering entry wins",
			scope: []string{"area.health", "area.health.training"},
			loc:   "areas.health.training",
			want:  true,
			via:   "area.health",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := reaches(c.scope, mustLoc(t, c.loc))
			if ok != c.want {
				t.Fatalf("reaches(%v, %q) ok = %v, want %v", c.scope, c.loc, ok, c.want)
			}
			if ok && got.Via != c.via {
				t.Errorf("Via = %q, want %q", got.Via, c.via)
			}
			if !ok && got.Via != "" {
				t.Errorf("Via = %q on a non-match, want empty", got.Via)
			}
		})
	}
}
