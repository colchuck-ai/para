package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/cli"
	"github.com/colchuck-ai/para/internal/clock"
)

// TestAddProjectEndToEnd is the one round trip proving the noun-dispatching
// rewrite (P19.3) is actually wired to mutate.Env.Add and not just to
// --help: a real tree, a real `add project acme`, and the directory it
// must have created.
func TestAddProjectEndToEnd(t *testing.T) {
	dir := t.TempDir()
	clk := clock.Fixed{At: time.Date(2026, time.August, 11, 12, 0, 0, 0, time.UTC)}

	var initOut, initErr bytes.Buffer
	if code := cli.Run(clk, []string{"init", dir, "--name", "Test"}, &initOut, &initErr); code != 0 {
		t.Fatalf("init: exit %d, stderr %q", code, initErr.String())
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	var addOut, addErr bytes.Buffer
	code := cli.Run(clk, []string{"add", "project", "acme", "--name", "Acme", "--description", "a project"}, &addOut, &addErr)
	if code != 0 {
		t.Fatalf("add project acme: exit %d, stderr %q", code, addErr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "projects", "acme", ".para", "state.toml")); err != nil {
		t.Errorf("projects/acme/.para/state.toml was not created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "projects", "acme", "objectives")); err != nil {
		t.Errorf("projects/acme/objectives was not created: %v", err)
	}

	// R24: the confirmation line names the entity in the dotted address
	// form, not the old plural-bucket locator string — a regression found
	// while implementing P19.8, since this test previously checked only
	// the exit code and the filesystem, never stdout.
	if got := addOut.String(); !strings.Contains(got, "project.acme") {
		t.Errorf("add project acme: stdout = %q, want it to contain %q", got, "project.acme")
	}
	if strings.Contains(addOut.String(), "projects.acme") {
		t.Errorf("add project acme: stdout = %q, still has the old plural-bucket form", addOut.String())
	}
}
