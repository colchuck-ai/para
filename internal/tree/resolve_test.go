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

// TestParentExistsForSkill: a skill's parent is never a precondition.
//
// .agents/skills/ has no .para/ of its own, so the general "parent holds a
// state.toml" rule does not apply (§1.4's exception) — and neither does the
// directory's own existence. It used to: this returned false until the directory
// was there, which made `para add skills.x` refuse on any fresh clone of a tree
// with no skills in it, since git does not carry an empty directory (§18.1) and
// nothing else would create one. The directory is para's own and the write path
// makes it on the way past.
func TestParentExistsForSkill(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml": "schema = 1\n",
	})

	for _, when := range []string{"before .agents/skills/ exists", "once it does"} {
		got, err := tree.ParentExists(root, mustParse(t, "skills.signups-report"))
		if err != nil {
			t.Fatalf("ParentExists %s: %v", when, err)
		}
		if !got {
			t.Errorf("ParentExists(skills.signups-report) = false %s, want true", when)
		}
		writeFixture(t, root, map[string]string{".agents/skills/.keep": ""})
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
