package address

import (
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
)

// TestIsBucket pins IsBucket's own list, moved here unchanged from kindmeta
// (task P17.6): the four §1.1 buckets, the three archived mirrors, the skill
// bucket (para-a3p: .agents/skills/ is a container now too, but has no
// archived mirror — skills cannot be archived), and nothing else — an
// entity, a container inside a bucket, and an archive of the archive all
// answer false.
func TestIsBucket(t *testing.T) {
	yes := []string{
		"projects", "areas", "resources", "archive", "archive.projects", "archive.areas", "archive.resources",
		"skills",
	}
	no := []string{
		"projects.acme", "areas.health", "areas.health.training",
		"projects.acme.objectives", "archive.archive", "archive.skills",
		// "archive" as a last segment paired with a non-archive first segment:
		// the archive-mirror clause requires loc[0] == "archive", not merely
		// that "archive" appears somewhere in the locator.
		"resources.archive",
	}
	for _, s := range yes {
		if !IsBucket(mustParseLoc(t, s)) {
			t.Errorf("IsBucket(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if IsBucket(mustParseLoc(t, s)) {
			t.Errorf("IsBucket(%q) = true, want false", s)
		}
	}
	if IsBucket(locator.Locator{}) {
		t.Error("IsBucket(empty) = true, want false")
	}
}

func mustParseLoc(t *testing.T, s string) locator.Locator {
	t.Helper()
	loc, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return loc
}
