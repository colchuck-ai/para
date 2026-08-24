package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestShowSuppressionFallsBackToTheJournalWhenCacheIsAbsent mirrors
// review_test's §28.4 upgrade case: show must print until and note from the
// journal when state.toml has not yet been backfilled with [suppression].
func TestShowSuppressionFallsBackToTheJournalWhenCacheIsAbsent(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "x"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, root, "suppress", "project", "acme",
		"--until", "2027-03-01", "--note", "paused, resumes with Q1 relaunch"); code != 0 {
		t.Fatalf("suppress: exit %d, stderr %q", code, stderr)
	}

	statePath := filepath.Join(root, "projects", "acme", ".para", "state.toml")
	current, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	without, _, ok := strings.Cut(string(current), "\n[suppression]")
	if !ok {
		t.Fatalf("state.toml has no [suppression] to strip:\n%s", current)
	}
	if err := os.WriteFile(statePath, []byte(without+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, root, "show", "project", "acme")
	if code != 0 {
		t.Fatalf("show: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "suppression") || !strings.Contains(stdout, "2027-03-01") {
		t.Errorf("show stdout = %q, want suppression until from the journal", stdout)
	}
	if !strings.Contains(stdout, "paused, resumes with Q1 relaunch") {
		t.Errorf("show stdout = %q, want the full note from the journal", stdout)
	}
}

// TestShowOmitsExpiredSuppression mirrors review's expiry case: a past until
// must not print a suppression line even when a stale cache table remains.
func TestShowOmitsExpiredSuppression(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "x"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, root, "suppress", "project", "acme",
		"--until", "2026-01-15", "--note", "already over"); code != 0 {
		t.Fatalf("suppress: exit %d, stderr %q", code, stderr)
	}

	code, stdout, stderr := run(t, root, "show", "project", "acme")
	if code != 0 {
		t.Fatalf("show: exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "suppression") {
		t.Errorf("show stdout = %q, want no suppression line after until passed", stdout)
	}
}

// TestShowSuppressionClearsAfterUnsuppress proves show re-reads the journal
// fold after unsuppress clears the active record.
func TestShowSuppressionClearsAfterUnsuppress(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "x"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, root, "suppress", "project", "acme",
		"--until", "2027-03-01", "--note", "paused"); code != 0 {
		t.Fatalf("suppress: exit %d, stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, root, "show", "project", "acme")
	if code != 0 {
		t.Fatalf("show after suppress: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "suppression") {
		t.Fatalf("show after suppress = %q, want a suppression line", stdout)
	}

	if code, _, stderr := run(t, root, "unsuppress", "project", "acme", "--note", "relaunch moved up"); code != 0 {
		t.Fatalf("unsuppress: exit %d, stderr %q", code, stderr)
	}
	code, stdout, stderr = run(t, root, "show", "project", "acme")
	if code != 0 {
		t.Fatalf("show after unsuppress: exit %d, stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "suppression") {
		t.Errorf("show after unsuppress = %q, want no suppression line", stdout)
	}
}

// TestShowJSONCarriesActiveSuppression is §23 parity: --json must carry the
// same derived suppression facts the human field block prints.
func TestShowJSONCarriesActiveSuppression(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "x"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, root, "suppress", "project", "acme",
		"--until", "2027-03-01", "--note", "paused, resumes with Q1 relaunch"); code != 0 {
		t.Fatalf("suppress: exit %d, stderr %q", code, stderr)
	}

	code, stdout, stderr := run(t, root, "show", "project", "acme", "--json")
	if code != 0 {
		t.Fatalf("show --json: exit %d, stderr %q", code, stderr)
	}
	for _, want := range []string{
		`"suppression-until": "2027-03-01"`,
		`"suppression-note": "paused, resumes with Q1 relaunch"`,
		`"suppression-days"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("show --json = %q, want %q", stdout, want)
		}
	}
}
