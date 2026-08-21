package tree_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

func TestResolveDotFromEntityDir(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	})

	loc, err := tree.ResolveDot(root, filepath.Join(root, "projects", "acme"))
	if err != nil {
		t.Fatalf("ResolveDot: %v", err)
	}
	if got, want := loc.String(), "projects.acme"; got != want {
		t.Errorf("ResolveDot = %q, want %q", got, want)
	}
}

func TestResolveDotWalksUpFromContent(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                    "schema = 1\n",
		"projects/.para/state.toml":          "name = \"Projects\"\n",
		"projects/acme/.para/state.toml":     "name = \"Acme\"\n",
		"projects/acme/design.md":            "notes\n",
		"projects/acme/scratch/nested/x.txt": "notes\n",
	})

	loc, err := tree.ResolveDot(root, filepath.Join(root, "projects", "acme", "scratch", "nested"))
	if err != nil {
		t.Fatalf("ResolveDot: %v", err)
	}
	if got, want := loc.String(), "projects.acme"; got != want {
		t.Errorf("ResolveDot = %q, want %q", got, want)
	}
}

func TestResolveDotFromInsideSkill(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml": "schema = 1\n",
		".agents/skills/para-signups-report/.para/state.toml": "name = \"Signups report\"\n",
		".agents/skills/para-signups-report/scripts/pull.sh":  "#!/bin/sh\n",
	})

	loc, err := tree.ResolveDot(root, filepath.Join(root, ".agents", "skills", "para-signups-report", "scripts"))
	if err != nil {
		t.Fatalf("ResolveDot: %v", err)
	}
	if got, want := loc.String(), "skills.signups-report"; got != want {
		t.Errorf("ResolveDot = %q, want %q", got, want)
	}
}

func TestResolveDotErrorsWhenNothingContainsIt(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml": "schema = 1\n",
	})

	_, err := tree.ResolveDot(root, root)
	if err == nil {
		t.Fatal("ResolveDot at the tree root should error: the root is not itself an entity or container")
	}
	var perr *paraerr.Error
	if !errors.As(err, &perr) || perr.Kind != paraerr.KindNotFound {
		t.Errorf("ResolveDot error = %v, want KindNotFound", err)
	}
}

func TestResolveDotErrorsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{".para/tree.toml": "schema = 1\n"})
	outside := t.TempDir()

	_, err := tree.ResolveDot(root, outside)
	if err == nil {
		t.Fatal("ResolveDot should error when startDir is not inside root")
	}
}

// TestResolveDotOrRootReportsTheRootRatherThanErroring is para-xbb: `show .`
// at the tree root treats the root as a legitimate answer, not a failure —
// ResolveDot's own error stays exactly as it was for every other caller.
func TestResolveDotOrRootReportsTheRootRatherThanErroring(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml": "schema = 1\n",
	})

	loc, atRoot, err := tree.ResolveDotOrRoot(root, root)
	if err != nil {
		t.Fatalf("ResolveDotOrRoot: %v", err)
	}
	if !atRoot {
		t.Errorf("atRoot = false, want true")
	}
	if loc != nil {
		t.Errorf("loc = %v, want nil", loc)
	}
}

// The same, from a subdirectory the root's own walk never planted a
// container in: nothing between it and the root satisfies "." either, and
// the tree is still the nearest thing that contains the working directory.
func TestResolveDotOrRootFromAnUntrackedSubdirectory(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":  "schema = 1\n",
		"scratch/notes.md": "notes\n",
	})

	_, atRoot, err := tree.ResolveDotOrRoot(root, filepath.Join(root, "scratch"))
	if err != nil {
		t.Fatalf("ResolveDotOrRoot: %v", err)
	}
	if !atRoot {
		t.Errorf("atRoot = false, want true")
	}
}

// ResolveDotOrRoot still finds a real container or entity before it ever
// reaches the root — the root is the fallback, not the first answer tried.
func TestResolveDotOrRootFindsAnEntityBeforeTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{
		".para/tree.toml":                "schema = 1\n",
		"projects/.para/state.toml":      "name = \"Projects\"\n",
		"projects/acme/.para/state.toml": "name = \"Acme\"\n",
	})

	loc, atRoot, err := tree.ResolveDotOrRoot(root, filepath.Join(root, "projects", "acme"))
	if err != nil {
		t.Fatalf("ResolveDotOrRoot: %v", err)
	}
	if atRoot {
		t.Fatal("atRoot = true, want false: projects.acme should have satisfied the walk")
	}
	if got, want := loc.String(), "projects.acme"; got != want {
		t.Errorf("ResolveDotOrRoot = %q, want %q", got, want)
	}
}

// ResolveDotOrRoot still refuses a startDir genuinely outside the tree —
// para-xbb only widens what counts as "found", not the tree's own boundary.
func TestResolveDotOrRootStillErrorsOutsideRoot(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, map[string]string{".para/tree.toml": "schema = 1\n"})
	outside := t.TempDir()

	_, _, err := tree.ResolveDotOrRoot(root, outside)
	if err == nil {
		t.Fatal("ResolveDotOrRoot should error when startDir is not inside root")
	}
}
