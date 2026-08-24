package cli_test

import (
	"strings"
	"testing"
)

// TestSuppressAndUnsuppressRoundTrip is §26/§28's worked example: suppress
// prints `until`, the state.toml cache picks it up, and unsuppress clears it.
func TestSuppressAndUnsuppressRoundTrip(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme-migration", "--name", "Acme migration", "--description", "Rebuild the consumer."); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}

	code, stdout, stderr := run(t, root, "suppress", "project", "acme-migration",
		"--until", "2027-03-01", "--note", "paused, resumes with Q1 relaunch")
	if code != 0 {
		t.Fatalf("suppress: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "suppressed  project.acme-migration  until 2027-03-01") {
		t.Errorf("suppress stdout = %q, want the §26 summary line", stdout)
	}
	if !strings.Contains(stdout, "wrote") || !strings.Contains(stdout, "state.toml") {
		t.Errorf("suppress stdout = %q, want state.toml among what it wrote", stdout)
	}

	code, stdout, stderr = run(t, root, "suppress", "project", "acme-migration")
	if code == 0 {
		t.Fatalf("suppress with no --until/--note: exit 0, want a validation error; stdout %q", stdout)
	}
	if !strings.Contains(stderr, "--until is required") {
		t.Errorf("suppress with no --until: stderr = %q, want it to name --until", stderr)
	}

	code, stdout, stderr = run(t, root, "unsuppress", "project", "acme-migration", "--note", "relaunch moved up")
	if code != 0 {
		t.Fatalf("unsuppress: exit %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "unsuppressed  project.acme-migration") {
		t.Errorf("unsuppress stdout = %q, want the §26 summary line", stdout)
	}
}

// TestSuppressRefusesContainer mirrors note's own CLI-level refusal (§13):
// typing the bare noun as a container is refused here even though a
// suppress event is structurally legal against any journal.
func TestSuppressRefusesContainer(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml": "",
	})
	code, _, stderr := run(t, root, "suppress", "container", "--until", "2027-03-01", "--note", "x")
	if code == 0 {
		t.Fatal("suppress container: exit 0, want a refusal")
	}
	if !strings.Contains(stderr, "container is refused") {
		t.Errorf("suppress container: stderr = %q, want the container refusal", stderr)
	}
}

// TestUnsuppressRequiresNoteAtTheCLI proves the CLI surfaces mutate.Unsuppress's
// own required-flag validation rather than swallowing it.
func TestUnsuppressRequiresNoteAtTheCLI(t *testing.T) {
	root := plantTree(t, map[string]string{
		".para/config.toml":         "",
		"projects/.para/state.toml": "name = \"Projects\"\n",
	})
	if code, _, stderr := run(t, root, "add", "project", "acme", "--name", "Acme", "--description", "a project"); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, stderr)
	}
	code, _, stderr := run(t, root, "unsuppress", "project", "acme")
	if code == 0 {
		t.Fatal("unsuppress with no --note: exit 0, want a validation error")
	}
	if !strings.Contains(stderr, "--note is required") {
		t.Errorf("unsuppress with no --note: stderr = %q, want it to name --note", stderr)
	}
}
