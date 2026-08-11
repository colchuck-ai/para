package mutate

import (
	"slices"
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

// TestRewriteScope is §5.4's whole obligation as a table: para owns the rename of
// every enumerated locator a relocation invalidates, and an entry covers its
// locator and everything beneath it (§5.2).
func TestRewriteScope(t *testing.T) {
	subtreeMove := func(t *testing.T, from, to string) entityMove {
		return entityMove{From: mustLoc(t, from), To: mustLoc(t, to), Subtree: true}
	}
	exactMove := func(t *testing.T, from, to string) entityMove {
		return entityMove{From: mustLoc(t, from), To: mustLoc(t, to)}
	}

	cases := []struct {
		name    string
		scope   []string
		moves   func(*testing.T) []entityMove
		want    []string
		changed int
	}{
		{
			name:  "the moved locator itself",
			scope: []string{"area.health.training", "project"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health.training", "areas.fitness.training")}
			},
			want:    []string{"area.fitness.training", "project"},
			changed: 1,
		},
		{
			name:  "anything beneath it",
			scope: []string{"area.health.training.tempo"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health", "areas.wellbeing")}
			},
			want:    []string{"area.wellbeing.training.tempo"},
			changed: 1,
		},
		{
			name:  "a sibling whose name merely starts the same way",
			scope: []string{"area.health-and-safety"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health", "areas.wellbeing")}
			},
			want:    []string{"area.health-and-safety"},
			changed: 0,
		},
		{
			name: "an ancestor that carried no subtree matches only itself",
			// unarchive reinstating areas.health while nutrition stayed archived
			// (§1.6): the ancestor moved, its other children did not.
			scope: []string{"archive.area.health", "archive.area.health.nutrition"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{exactMove(t, "archive.areas.health", "areas.health")}
			},
			want:    []string{"area.health", "archive.area.health.nutrition"},
			changed: 1,
		},
		{
			name:  "a rewrite landing on an entry the list already holds",
			scope: []string{"area.fitness.training", "area.health.training"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health.training", "areas.fitness.training")}
			},
			// Two identical entries would render the same clause twice (§5.3).
			want:    []string{"area.fitness.training"},
			changed: 1,
		},
		{
			name:  "the most specific move wins",
			scope: []string{"archive.area.health.training"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{
					subtreeMove(t, "archive.areas.health.training", "areas.health.training"),
					exactMove(t, "archive.areas.health", "areas.health"),
				}
			},
			want:    []string{"area.health.training"},
			changed: 1,
		},
		{
			// A project rename drags its objectives and key-results with it
			// (they nest under it on disk), but "project.acme" is not a
			// dotted-string prefix of "objective.acme.q1-growth" the way
			// "projects.acme" is a prefix of "projects.acme.objectives.q1-growth"
			// — the structural word "objectives" the address form drops. The
			// match has to happen in Locator space for this case to work at
			// all.
			name:  "a move crossing a noun boundary its own address does not show",
			scope: []string{"objective.acme.q1-growth"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "projects.acme", "projects.acme-migration")}
			},
			want:    []string{"objective.acme-migration.q1-growth"},
			changed: 1,
		},
		{
			name:  "no scope at all is the whole tree, and stays that way",
			scope: nil,
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health", "areas.wellbeing")}
			},
			want:    []string{},
			changed: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, changed := rewriteScope(tc.scope, tc.moves(t))
			if !slices.Equal(got, tc.want) {
				t.Errorf("entries: got %v, want %v", got, tc.want)
			}
			if changed != tc.changed {
				t.Errorf("changed: got %d, want %d", changed, tc.changed)
			}
		})
	}
}
