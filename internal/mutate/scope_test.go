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
			scope: []string{"areas.health.training", "projects"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health.training", "areas.fitness.training")}
			},
			want:    []string{"areas.fitness.training", "projects"},
			changed: 1,
		},
		{
			name:  "anything beneath it",
			scope: []string{"areas.health.training.tempo"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health", "areas.wellbeing")}
			},
			want:    []string{"areas.wellbeing.training.tempo"},
			changed: 1,
		},
		{
			name:  "a sibling whose name merely starts the same way",
			scope: []string{"areas.health-and-safety"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health", "areas.wellbeing")}
			},
			want:    []string{"areas.health-and-safety"},
			changed: 0,
		},
		{
			name: "an ancestor that carried no subtree matches only itself",
			// unarchive reinstating areas.health while nutrition stayed archived
			// (§1.6): the ancestor moved, its other children did not.
			scope: []string{"archive.areas.health", "archive.areas.health.nutrition"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{exactMove(t, "archive.areas.health", "areas.health")}
			},
			want:    []string{"areas.health", "archive.areas.health.nutrition"},
			changed: 1,
		},
		{
			name:  "a rewrite landing on an entry the list already holds",
			scope: []string{"areas.fitness.training", "areas.health.training"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{subtreeMove(t, "areas.health.training", "areas.fitness.training")}
			},
			// Two identical entries would render the same clause twice (§5.3).
			want:    []string{"areas.fitness.training"},
			changed: 1,
		},
		{
			name:  "the most specific move wins",
			scope: []string{"archive.areas.health.training"},
			moves: func(t *testing.T) []entityMove {
				return []entityMove{
					subtreeMove(t, "archive.areas.health.training", "areas.health.training"),
					exactMove(t, "archive.areas.health", "areas.health"),
				}
			},
			want:    []string{"areas.health.training"},
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
