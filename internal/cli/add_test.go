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

// TestAddDryRunWritesNothingAndMatchesTheRealRun is para-uk5: a rehearsal must
// create nothing, and the file list it reports must be the one a real run —
// issued right after, against the same tree — actually produces, not merely a
// plausible-looking one.
func TestAddDryRunWritesNothingAndMatchesTheRealRun(t *testing.T) {
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

	args := []string{"add", "project", "acme", "--name", "Acme", "--description", "a project"}

	var dryOut, dryErr bytes.Buffer
	if code := cli.Run(clk, append(append([]string{}, args...), "--dry-run"), &dryOut, &dryErr); code != 0 {
		t.Fatalf("add --dry-run: exit %d, stderr %q", code, dryErr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, "projects", "acme")); !os.IsNotExist(err) {
		t.Fatalf("add --dry-run created projects/acme")
	}
	if !strings.Contains(dryOut.String(), "dry-run: nothing was written") {
		t.Errorf("add --dry-run: stdout = %q, want the dry-run trailer", dryOut.String())
	}
	if !strings.Contains(dryOut.String(), "wrote") {
		t.Errorf("add --dry-run: stdout = %q, want a wrote list", dryOut.String())
	}

	var realOut, realErr bytes.Buffer
	if code := cli.Run(clk, args, &realOut, &realErr); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, realErr.String())
	}

	// The rehearsal's output is the real run's output plus the trailer — same
	// summary line, same wrote list, in the same order.
	want := strings.TrimSuffix(dryOut.String(), "dry-run: nothing was written\n")
	if realOut.String() != want {
		t.Errorf("real run's stdout =\n%s\nwant (dry run minus its trailer)\n%s", realOut.String(), want)
	}
}

// TestAddSkillDryRunMatchesTheRealRunWithClaudeSurfaceOn is
// TestAddDryRunWritesNothingAndMatchesTheRealRun's counterpart for a skill with
// emit.claude on end to end through the CLI, not just internal/mutate: a
// rehearsed skill add reaches CLAUDE.md at all eight locations and
// `.claude/skills/`, and a rehearsal has to report that reach the same way a
// real run immediately after does.
func TestAddSkillDryRunMatchesTheRealRunWithClaudeSurfaceOn(t *testing.T) {
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

	var cfgOut, cfgErr bytes.Buffer
	if code := cli.Run(clk, []string{"config", "set", "emit.claude", "true"}, &cfgOut, &cfgErr); code != 0 {
		t.Fatalf("config set emit.claude: exit %d, stderr %q", code, cfgErr.String())
	}

	args := []string{"add", "skill", "commit-style", "--name", "Commit style", "--description", "when writing a commit message"}

	var dryOut, dryErr bytes.Buffer
	if code := cli.Run(clk, append(append([]string{}, args...), "--dry-run"), &dryOut, &dryErr); code != 0 {
		t.Fatalf("add --dry-run: exit %d, stderr %q", code, dryErr.String())
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "para-commit-style")); !os.IsNotExist(err) {
		t.Fatalf("add --dry-run created the skill")
	}
	if !strings.Contains(dryOut.String(), "CLAUDE.md") {
		t.Errorf("add skill --dry-run: stdout = %q, want it to report the CLAUDE.md rewrites", dryOut.String())
	}
	if !strings.Contains(dryOut.String(), "linked") {
		t.Errorf("add skill --dry-run: stdout = %q, want it to report the mirror link", dryOut.String())
	}

	var realOut, realErr bytes.Buffer
	if code := cli.Run(clk, args, &realOut, &realErr); code != 0 {
		t.Fatalf("add: exit %d, stderr %q", code, realErr.String())
	}

	want := strings.TrimSuffix(dryOut.String(), "dry-run: nothing was written\n")
	if realOut.String() != want {
		t.Errorf("real run's stdout =\n%s\nwant (dry run minus its trailer)\n%s", realOut.String(), want)
	}
}
