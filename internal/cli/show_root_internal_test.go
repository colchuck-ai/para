package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestShowDotAtTheRootSummarisesTheTree is para-xbb: the root has
// .para/tree.toml and is plainly a thing para tracks, but §14's own "."
// walk never satisfied there before this — the first thing a new user
// tries in a fresh tree failed with a message implying the tree was not
// set up. "." at the root now summarises the tree instead of erroring:
// its name, created date, config, and its container children — the four
// buckets tree.toml's own child events name (§8.1).
func TestShowDotAtTheRootSummarisesTheTree(t *testing.T) {
	chdirToTestTree(t)

	out, err := execShow([]string{"."})
	if err != nil {
		t.Fatalf("show .: %v (%s)", err, out)
	}
	for _, want := range []string{"Test", "created", "containers", "projects", "areas", "resources", "archive"} {
		if !strings.Contains(out, want) {
			t.Errorf("show . output does not contain %q; got:\n%s", want, out)
		}
	}
}

// TestShowDotAtTheRootStillErrorsOutsideAnyTree confirms para-xbb widens
// what "." resolves to inside a tree, not whether a tree was found at all —
// tree.Find's own refusal still fires first.
func TestShowDotAtTheRootStillErrorsOutsideAnyTree(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_, err = execShow([]string{"."})
	if err == nil {
		t.Fatal("show . outside any tree: want an error, got none")
	}
	if !strings.Contains(err.Error(), "no para tree found") {
		t.Errorf("error = %q, want it to name the missing tree", err.Error())
	}
}

// TestShowDotAtTheRootFromAnUntrackedSubdirectory is the same fallback one
// level down: a directory nothing tracks still has the tree as its nearest
// container.
func TestShowDotAtTheRootFromAnUntrackedSubdirectory(t *testing.T) {
	chdirToTestTree(t)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("scratch", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("scratch"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := execShow([]string{"."})
	if err != nil {
		t.Fatalf("show . from an untracked subdirectory: %v (%s)", err, out)
	}
	if !strings.Contains(out, "Test") || !strings.Contains(out, "containers") {
		t.Errorf("show . output = %q, want the tree summary", out)
	}
}

// TestShowDotAtTheRootJSON is the --json shape: the tree's own identity
// fields, which no locator-addressed command can otherwise reach (§8.1's
// root has no locator of its own), plus its four containers rendered the
// same entityJSON shape every other `show --json` child uses.
func TestShowDotAtTheRootJSON(t *testing.T) {
	chdirToTestTree(t)

	out, err := execShow([]string{".", "--json"})
	if err != nil {
		t.Fatalf("show . --json: %v (%s)", err, out)
	}
	var got struct {
		Name       string `json:"name"`
		Created    string `json:"created"`
		Containers []struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"containers"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("json.Unmarshal: %v; out = %s", err, out)
	}
	if got.Name != "Test" {
		t.Errorf("Name = %q, want %q", got.Name, "Test")
	}
	if got.Created == "" {
		t.Error("Created is empty, want the tree's own created timestamp")
	}
	if len(got.Containers) != 4 {
		t.Fatalf("Containers = %d, want 4; got %+v", len(got.Containers), got.Containers)
	}
	// ID is the bucket's own on-disk name ("projects"); the entityJSON
	// Locator field is instead the singular addressable noun ("project") —
	// R26's disagreement branch, since a bucket's Kind is container but its
	// address is the bare noun word (R17).
	want := map[string]bool{"projects": false, "areas": false, "resources": false, "archive": false}
	for _, c := range got.Containers {
		if c.Kind != "container" {
			t.Errorf("Containers[%s].Kind = %q, want %q", c.ID, c.Kind, "container")
		}
		if _, ok := want[c.ID]; !ok {
			t.Errorf("unexpected container id %q", c.ID)
			continue
		}
		want[c.ID] = true
	}
	for id, seen := range want {
		if !seen {
			t.Errorf("container %q missing from output", id)
		}
	}
}
