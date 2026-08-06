package tree_test

import (
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/tree"
)

func mustParse(t *testing.T, s string) locator.Locator {
	t.Helper()
	loc, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return loc
}

func TestExists(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	})

	got, err := tree.Exists(root, mustParse(t, "projects.acme"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !got {
		t.Error("Exists(projects.acme) = false, want true")
	}

	got, err = tree.Exists(root, mustParse(t, "projects.missing"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if got {
		t.Error("Exists(projects.missing) = true, want false")
	}
}

func TestExistsForStub(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"archive/.para/state.toml":       "name = \"Archive\"\n",
		"archive/areas/.para/state.toml": "name = \"Areas\"\n",
		"archive/areas/health/notes.md":  "a bare stub, no .para/\n",
	})

	got, err := tree.Exists(root, mustParse(t, "archive.areas.health"))
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if got {
		t.Error("Exists(archive.areas.health) = true for a stub, want false — a stub is not an entity")
	}
}

func TestParentExistsForBucketLevelEntity(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":           "schema = 1\n",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})

	got, err := tree.ParentExists(root, mustParse(t, "projects.acme"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if !got {
		t.Error("ParentExists(projects.acme) = false, want true (projects/ bucket exists)")
	}
}

func TestParentExistsForObjectiveNeedsEagerContainer(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	})

	// The objectives/ container has not been eagerly created yet, so an
	// objective cannot legally be placed under projects.acme.
	got, err := tree.ParentExists(root, mustParse(t, "projects.acme.objectives.q1"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if got {
		t.Error("ParentExists(projects.acme.objectives.q1) = true before objectives/ exists, want false")
	}

	writeFixture(t, root, map[string]string{
		"projects/acme/objectives/.para/state.toml": "name = \"Objectives\"\n",
	})
	got, err = tree.ParentExists(root, mustParse(t, "projects.acme.objectives.q1"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if !got {
		t.Error("ParentExists(projects.acme.objectives.q1) = false once objectives/ exists, want true")
	}
}

func TestParentExistsForNestedArea(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":               "schema = 1\n",
		"areas/.para/state.toml":        "name = \"Areas\"\n",
		"areas/health/.para/state.toml": "name = \"Health\"\n",
	})

	got, err := tree.ParentExists(root, mustParse(t, "areas.health.training"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if !got {
		t.Error("ParentExists(areas.health.training) = false, want true (areas.health is a live entity)")
	}
}

func TestParentExistsForSkill(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml": "schema = 1\n",
	})

	// .agents/skills/ has no .para/ of its own — the general "parent holds
	// state.toml" rule does not apply to skills (§1.4's exception).
	got, err := tree.ParentExists(root, mustParse(t, "skills.signups-report"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if got {
		t.Error("ParentExists(skills.signups-report) = true before .agents/skills/ exists, want false")
	}

	writeFixture(t, root, map[string]string{
		".agents/skills/.keep": "",
	})
	got, err = tree.ParentExists(root, mustParse(t, "skills.signups-report"))
	if err != nil {
		t.Fatalf("ParentExists: %v", err)
	}
	if !got {
		t.Error("ParentExists(skills.signups-report) = false once .agents/skills/ exists, want true")
	}
}

// TestKindAt covers the classification the walk and every mutation share: a
// reserved last segment is a container, an id position derives its kind from
// §1.3, and the root has no kind of its own (§8.1).
func TestKindAt(t *testing.T) {
	tests := []struct {
		loc     string
		want    kindmeta.Kind
		wantErr bool
	}{
		{"", kindmeta.KindUnknown, false},
		{"projects", kindmeta.KindContainer, false},
		{"archive", kindmeta.KindContainer, false},
		{"archive.projects", kindmeta.KindContainer, false},
		{"projects.acme", kindmeta.KindProject, false},
		{"projects.acme.objectives", kindmeta.KindContainer, false},
		{"projects.acme.objectives.q1", kindmeta.KindObjective, false},
		{"projects.acme.objectives.q1.key-results", kindmeta.KindContainer, false},
		{"projects.acme.objectives.q1.key-results.signups", kindmeta.KindKeyResult, false},
		{"areas.health.training", kindmeta.KindArea, false},
		{"skills.signups-report", kindmeta.KindSkill, false},
		// A project cannot nest, so no position derives a kind (§1.3).
		{"projects.acme.nested", kindmeta.KindUnknown, true},
	}
	for _, tt := range tests {
		var loc locator.Locator
		if tt.loc != "" {
			var err error
			loc, err = locator.Parse(tt.loc)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.loc, err)
			}
		}
		got, err := tree.KindAt(loc)
		if (err != nil) != tt.wantErr {
			t.Errorf("KindAt(%q) error = %v, wantErr %v", tt.loc, err, tt.wantErr)
		}
		if got != tt.want {
			t.Errorf("KindAt(%q) = %v, want %v", tt.loc, got, tt.want)
		}
	}
}
