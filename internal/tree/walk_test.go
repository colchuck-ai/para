package tree_test

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/tree"
)

// writeFixture materializes a tree from a map of "/"-separated relative
// paths to file content, creating parent directories as needed. A path
// ending in "/" is a directory with no file written.
func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if content == "" && rel[len(rel)-1] == '/' {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func collectLocators(t *testing.T, root string) []string {
	t.Helper()
	var got []string
	err := tree.Walk(root, func(n tree.Node) error {
		got = append(got, n.Locator.String())
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sort.Strings(got)
	return got
}

func TestWalkFindsEntitiesContainersAndSkills(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                                     "schema = 1\n",
		"projects/.para/state.toml":                           "name = \"Projects\"\n",
		"projects/acme/.para/state.toml":                      "name = \"Acme\"\n",
		"projects/acme/objectives/.para/state.toml":           "name = \"Objectives\"\n",
		"projects/acme/objectives/q1/.para/state.toml":        "name = \"Q1\"\n",
		"areas/.para/state.toml":                              "name = \"Areas\"\n",
		"areas/health/.para/state.toml":                       "name = \"Health\"\n",
		"areas/health/training/.para/state.toml":              "name = \"Training\"\n",
		".agents/skills/para-signups-report/.para/state.toml": "name = \"Signups report\"\n",
		".agents/rules/my-own-constraint.md":                  "# not para's\n",
	})

	got := collectLocators(t, root)
	want := []string{
		"areas",
		"areas.health",
		"areas.health.training",
		"projects",
		"projects.acme",
		"projects.acme.objectives",
		"projects.acme.objectives.q1",
		"skills.signups-report",
	}
	sort.Strings(want)
	assertStringSlicesEqual(t, got, want)
}

func TestWalkClassifiesContainersAndEntities(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                              "schema = 1\n",
		"projects/.para/state.toml":                    "name = \"Projects\"\n",
		"projects/acme/.para/state.toml":               "name = \"Acme\"\n",
		"projects/acme/objectives/.para/state.toml":    "name = \"Objectives\"\n",
		"projects/acme/objectives/q1/.para/state.toml": "name = \"Q1\"\n",
	})

	nodes := map[string]tree.Node{}
	if err := tree.Walk(root, func(n tree.Node) error {
		nodes[n.Locator.String()] = n
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if n := nodes["projects"]; !n.IsContainer || n.Kind != kindmeta.KindContainer {
		t.Errorf("projects node = %+v, want container", n)
	}
	if n := nodes["projects.acme"]; n.IsContainer || n.Kind != kindmeta.KindProject {
		t.Errorf("projects.acme node = %+v, want project entity", n)
	}
	if n := nodes["projects.acme.objectives"]; !n.IsContainer || n.Kind != kindmeta.KindContainer {
		t.Errorf("projects.acme.objectives node = %+v, want container", n)
	}
	if n := nodes["projects.acme.objectives.q1"]; n.IsContainer || n.Kind != kindmeta.KindObjective {
		t.Errorf("projects.acme.objectives.q1 node = %+v, want objective entity", n)
	}
}

func TestWalkIgnoresUntrackedContent(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":               "schema = 1\n",
		"areas/.para/state.toml":        "name = \"Areas\"\n",
		"areas/health/.para/state.toml": "name = \"Health\"\n",
		// content: an untracked directory, with a hand-planted state.toml
		// buried inside it. The walk's one known weakness (§8.5) is that
		// this stays invisible; only doctor's deep scan (a later phase)
		// finds it.
		"areas/health/scans/random-note.md":          "just a file\n",
		"areas/health/scans/buried/.para/state.toml": "name = \"Buried\"\n",
	})

	got := collectLocators(t, root)
	want := []string{"areas", "areas.health"}
	assertStringSlicesEqual(t, got, want)
}

func TestWalkNeverFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/real/.para/state.toml": "name = \"Real\"\n",
	})
	// A symlinked "entity" inside projects/, mimicking what a mirrored
	// skill could manufacture if the walk ever wandered into .claude/
	// (§6.1, §21.2). The walk must never descend into it.
	elsewhere := t.TempDir()
	writeFixture(t, elsewhere, map[string]string{
		".para/state.toml": "name = \"Phantom\"\n",
	})
	if err := os.Symlink(elsewhere, filepath.Join(root, "projects", "phantom")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	got := collectLocators(t, root)
	want := []string{"projects", "projects.real"}
	assertStringSlicesEqual(t, got, want)
}

func TestWalkRecognizesArchiveStubs(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                                "schema = 1\n",
		"archive/.para/state.toml":                       "name = \"Archive\"\n",
		"archive/areas/.para/state.toml":                 "name = \"Areas\"\n",
		"archive/areas/health/training/.para/state.toml": "name = \"Training\"\n",
	})
	// archive/areas/health/ itself has no .para/ at all — a stub (§1.6).

	var stub, entity *tree.Node
	if err := tree.Walk(root, func(n tree.Node) error {
		found := n
		switch found.Locator.String() {
		case "archive.areas.health":
			stub = &found
		case "archive.areas.health.training":
			entity = &found
		}
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}

	if stub == nil {
		t.Fatal("expected a stub node for archive.areas.health")
	}
	if !stub.Stub || !stub.Archived {
		t.Errorf("archive.areas.health node = %+v, want Stub=true Archived=true", *stub)
	}
	if entity == nil {
		t.Fatal("expected an entity node for archive.areas.health.training")
	}
	if entity.Stub || !entity.Archived || entity.Kind != kindmeta.KindArea {
		t.Errorf("archive.areas.health.training node = %+v, want a live archived area", *entity)
	}
}

// TestWalkIgnoresContentInsideAnArchivedEntity guards against treating
// ordinary content nested inside an already-classified archived entity as a
// stub: a stub is specifically the bare ancestry marker §1.6 describes, not
// "any bare directory under archive/". A project cannot nest at all, so a
// content directory under one is an unambiguous case.
func TestWalkIgnoresContentInsideAnArchivedEntity(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                                 "schema = 1\n",
		"archive/.para/state.toml":                        "name = \"Archive\"\n",
		"archive/projects/.para/state.toml":               "name = \"Projects\"\n",
		"archive/projects/old-migration/.para/state.toml": "name = \"Old migration\"\n",
		"archive/projects/old-migration/notes/plan.md":    "just a file\n",
	})

	got := collectLocators(t, root)
	want := []string{"archive", "archive.projects", "archive.projects.old-migration"}
	assertStringSlicesEqual(t, got, want)
}

// TestWalkIgnoresEmptyContentUnderAnArchivedArea guards the ambiguous case:
// areas nest to unlimited depth, so a bare content directory under an
// archived area is structurally identical to a genuine stub-in-waiting.
// The discriminator is downstream reality, not shape: if nothing real is
// ever found beneath it, it must not be reported as a stub.
func TestWalkIgnoresEmptyContentUnderAnArchivedArea(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                       "schema = 1\n",
		"archive/.para/state.toml":              "name = \"Archive\"\n",
		"archive/areas/.para/state.toml":        "name = \"Areas\"\n",
		"archive/areas/health/scans/reading.md": "just a file\n",
	})

	got := collectLocators(t, root)
	want := []string{"archive", "archive.areas"}
	assertStringSlicesEqual(t, got, want)
}

func assertStringSlicesEqual(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
