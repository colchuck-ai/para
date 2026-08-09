package kindmeta

import (
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
)

// TestIsContainer is §1.2's container list: the four buckets, the three
// archived mirrors, objectives/ under a project, key-results/ under an
// objective, and nothing else.
func TestIsContainer(t *testing.T) {
	yes := []string{
		"projects", "areas", "resources", "archive",
		"archive.projects", "archive.areas", "archive.resources",
		"projects.acme.objectives",
		"projects.acme.objectives.q1.key-results",
		"archive.projects.acme.objectives",
		"archive.projects.acme.objectives.q1.key-results",
	}
	no := []string{
		// A reserved word used as an id: §10's `collision`, and the case a
		// name-only test would wave through as a container.
		"projects.skills",
		"projects.logs",
		"areas.health.objectives",
		"resources.archive",
		// A container in a position that has none: §10's `misplaced`.
		"projects.acme.key-results",
		"projects.acme.objectives.q1.objectives",
		"archive.archive",
		"archive.skills",
		// Entities, which are not containers however deep.
		"projects.acme", "areas.health.training", "skills.report",
		// The second root is not a directory in the tree (§1.4).
		"skills",
	}

	for _, s := range yes {
		if !IsContainer(mustParse(t, s)) {
			t.Errorf("IsContainer(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if IsContainer(mustParse(t, s)) {
			t.Errorf("IsContainer(%q) = true, want false", s)
		}
	}
	if IsContainer(locator.Locator{}) {
		t.Error("IsContainer(empty) = true, want false — the root is the tree, not a container")
	}
}
