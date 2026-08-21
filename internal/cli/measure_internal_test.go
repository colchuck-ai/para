package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execMeasure runs args against a fresh `measure` command in the cwd
// chdirToTestTree set up. measure never dispatches on a noun (R13: it takes
// no noun at all), so — like move — there is no HasParent()/Root() gotcha.
func execMeasure(args []string) (string, error) {
	cmd := newMeasureCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// mustAddKeyResultChain adds a project, an objective under it, and a
// key-result under that, through the real add dispatch — the ancestor
// chain measure's own key-result chain (R13's bare chain) needs to exist.
func mustAddKeyResultChain(t *testing.T, projectID, objectiveID, keyResultID string) {
	t.Helper()
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	run := func(args []string) {
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("add %v: %v (%s)", args, err, out.String())
		}
	}
	run([]string{"add", "project", projectID, "--name", projectID, "--description", "x"})
	run([]string{"add", "objective", projectID + "." + objectiveID, "--name", objectiveID, "--description", "x"})
	run([]string{
		"add", "key-result", projectID + "." + objectiveID + "." + keyResultID,
		"--name", keyResultID, "--type", "number", "--target", "100",
	})
}

// TestMeasureBareChainResolvesToKeyResult is R13's own shape: no noun at
// all, just the chain and the value.
func TestMeasureBareChainResolvesToKeyResult(t *testing.T) {
	chdirToTestTree(t)
	mustAddKeyResultChain(t, "acme", "q1-growth", "signups")

	out, err := execMeasure([]string{"acme.q1-growth.signups", "42"})
	if err != nil {
		t.Fatalf("measure acme.q1-growth.signups 42: %v (%s)", err, out)
	}
	if !strings.Contains(out, "measured signups = 42") {
		t.Errorf("measure output = %q, want it to report the reading", out)
	}
}

// TestMeasureWrongArityChainRefusedByName is the task's other example: a
// chain of the wrong arity is refused by name — key-result always takes
// exactly three segments (R4), and a two-segment chain names an objective's
// shape, not a key-result's.
func TestMeasureWrongArityChainRefusedByName(t *testing.T) {
	chdirToTestTree(t)

	_, err := execMeasure([]string{"acme.q1-growth", "42"})
	if err == nil {
		t.Fatal("measure acme.q1-growth 42: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "key-result takes exactly three segments") {
		t.Errorf("error = %q, want the named arity refusal", err.Error())
	}
}

// TestMeasureDotResolvesWhenCwdIsTheKeyResult is R16's "." carried into the
// one bare-chain command: standing inside a key-result's own directory,
// "." is worth as much as typing its chain out.
func TestMeasureDotResolvesWhenCwdIsTheKeyResult(t *testing.T) {
	chdirToTestTree(t)
	mustAddKeyResultChain(t, "acme", "q1-growth", "signups")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	krDir := filepath.Join(cwd, "projects", "acme", "objectives", "q1-growth", "key-results", "signups")
	if err := os.Chdir(krDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := execMeasure([]string{".", "7"})
	if err != nil {
		t.Fatalf("measure . 7: %v (%s)", err, out)
	}
	if !strings.Contains(out, "measured signups = 7") {
		t.Errorf("measure output = %q, want it to report the reading", out)
	}
}

// TestMeasureDotWrongKindRefused mirrors set's own kind-mismatch check:
// "." resolving to something other than a key-result is refused by name,
// not silently measured as if it were one.
func TestMeasureDotWrongKindRefused(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(cwd, "projects", "acme")
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	_, err = execMeasure([]string{".", "7"})
	if err == nil {
		t.Fatal("measure . (cwd is a project): want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "not a key-result") {
		t.Errorf("error = %q, want it to name the kind mismatch", err.Error())
	}
}
