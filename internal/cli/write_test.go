package cli_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestSetDryRunWritesNothingAndMatchesTheRealRun is para-ato's `set` case of
// add_test.go's own property: the rehearsal's stdout is the real run's
// stdout plus the trailer, and the rehearsal itself writes nothing.
func TestSetDryRunWritesNothingAndMatchesTheRealRun(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "a project"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	statePath := filepath.Join(root, "projects", "acme", ".para", "state.toml")
	before := statOf(t, statePath)

	dryCode, dryOut, dryErr := run(t, root, "set", "project", "acme", "--status", "in-progress", "--dry-run")
	if dryCode != 0 {
		t.Fatalf("set --dry-run: exit %d, stderr %q", dryCode, dryErr)
	}
	if after := statOf(t, statePath); after != before {
		t.Errorf("set --dry-run rewrote %s", statePath)
	}
	if !strings.Contains(dryOut, "dry-run: nothing was written") {
		t.Errorf("set --dry-run: stdout = %q, want the dry-run trailer", dryOut)
	}

	realCode, realOut, realErr := run(t, root, "set", "project", "acme", "--status", "in-progress")
	if realCode != 0 {
		t.Fatalf("set: exit %d, stderr %q", realCode, realErr)
	}
	want := strings.TrimSuffix(dryOut, "dry-run: nothing was written\n")
	if realOut != want {
		t.Errorf("real run's stdout =\n%q\nwant (dry run minus its trailer)\n%q", realOut, want)
	}
}

// TestUnsetDryRunWritesNothingAndMatchesTheRealRun is set's twin for `unset`.
func TestUnsetDryRunWritesNothingAndMatchesTheRealRun(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "a project", "--due", "2026-12-31"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	statePath := filepath.Join(root, "projects", "acme", ".para", "state.toml")
	before := statOf(t, statePath)

	dryCode, dryOut, dryErr := run(t, root, "unset", "project", "acme", "due", "--dry-run")
	if dryCode != 0 {
		t.Fatalf("unset --dry-run: exit %d, stderr %q", dryCode, dryErr)
	}
	if after := statOf(t, statePath); after != before {
		t.Errorf("unset --dry-run rewrote %s", statePath)
	}
	if !strings.Contains(dryOut, "dry-run: nothing was written") {
		t.Errorf("unset --dry-run: stdout = %q, want the dry-run trailer", dryOut)
	}

	realCode, realOut, realErr := run(t, root, "unset", "project", "acme", "due")
	if realCode != 0 {
		t.Fatalf("unset: exit %d, stderr %q", realCode, realErr)
	}
	want := strings.TrimSuffix(dryOut, "dry-run: nothing was written\n")
	if realOut != want {
		t.Errorf("real run's stdout =\n%q\nwant (dry run minus its trailer)\n%q", realOut, want)
	}
}

// TestNoteDryRunWritesNothingAndMatchesTheRealRun is set's twin for `note`.
func TestNoteDryRunWritesNothingAndMatchesTheRealRun(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":      "",
		"areas/.para/state.toml": "name = \"Areas\"\n",
	})
	if code, _, stderr := run(t, root, "add", "area", "health", "--name", "Health", "--description", "Staying in one piece."); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	activityPath := filepath.Join(root, "areas", "health", "ACTIVITY.md")
	before := statOf(t, activityPath)

	dryCode, dryOut, dryErr := run(t, root, "note", "area", "health", "swapped the tempo block for intervals", "--dry-run")
	if dryCode != 0 {
		t.Fatalf("note --dry-run: exit %d, stderr %q", dryCode, dryErr)
	}
	if after := statOf(t, activityPath); after != before {
		t.Errorf("note --dry-run rewrote %s", activityPath)
	}
	if !strings.Contains(dryOut, "dry-run: nothing was written") {
		t.Errorf("note --dry-run: stdout = %q, want the dry-run trailer", dryOut)
	}

	realCode, realOut, realErr := run(t, root, "note", "area", "health", "swapped the tempo block for intervals")
	if realCode != 0 {
		t.Fatalf("note: exit %d, stderr %q", realCode, realErr)
	}
	want := strings.TrimSuffix(dryOut, "dry-run: nothing was written\n")
	if realOut != want {
		t.Errorf("real run's stdout =\n%q\nwant (dry run minus its trailer)\n%q", realOut, want)
	}
}
