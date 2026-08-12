package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// execRead runs args against a fresh read command. None of the seven
// commands this task rewires (show, log, path, activity, review, rebuild,
// doctor) dispatch on a subcommand, so there is no HasParent()/Root()
// gotcha here either.
func execRead(cmd *cobra.Command, args []string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

// bucketArityReadCmds is show's, log's, and path's shape (R12): the noun is
// required, but alone it names R17's bucket — never the tree root.
func bucketArityReadCmds() map[string]func() *cobra.Command {
	return map[string]func() *cobra.Command{
		"show": newShowCmd,
		"log":  newLogCmd,
		"path": newPathCmd,
	}
}

// scopeArityReadCmds is activity's, review's, rebuild's, and doctor's shape
// (R12): the noun itself may be left off, naming the tree root (R18).
func scopeArityReadCmds() map[string]func() *cobra.Command {
	return map[string]func() *cobra.Command{
		"activity": newActivityCmd,
		"review":   newReviewCmd,
		"rebuild":  newRebuildCmd,
		"doctor":   newDoctorCmd,
	}
}

func allReadCmds() map[string]func() *cobra.Command {
	out := map[string]func() *cobra.Command{}
	for name, ctor := range bucketArityReadCmds() {
		out[name] = ctor
	}
	for name, ctor := range scopeArityReadCmds() {
		out[name] = ctor
	}
	return out
}

// TestReadCommandsAcceptNounAndChain is R12's scope form, across all seven.
func TestReadCommandsAcceptNounAndChain(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	for name, ctor := range allReadCmds() {
		t.Run(name, func(t *testing.T) {
			out, err := execRead(ctor(), []string{"project", "acme"})
			if err != nil {
				t.Fatalf("%s project acme: %v (%s)", name, err, out)
			}
		})
	}
}

// TestReadCommandsAcceptBareNounAsBucket is R17: a noun with no chain names
// the bucket, accepted by all seven — the property that sets them apart
// from note/remove/archive/unarchive/add/set/unset, which refuse it.
func TestReadCommandsAcceptBareNounAsBucket(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	for name, ctor := range allReadCmds() {
		t.Run(name, func(t *testing.T) {
			out, err := execRead(ctor(), []string{"project"})
			if err != nil {
				t.Fatalf("%s project (bucket): %v (%s)", name, err, out)
			}
		})
	}
}

// TestShowLogPathRequireANoun is R12's other half for the bucket-arity
// three: unlike activity/review/rebuild/doctor, no arguments at all is a
// refusal here, not the tree root.
func TestShowLogPathRequireANoun(t *testing.T) {
	chdirToTestTree(t)

	for name, ctor := range bucketArityReadCmds() {
		t.Run(name, func(t *testing.T) {
			_, err := execRead(ctor(), nil)
			if err == nil {
				t.Fatalf("%s (no args): want a refusal, got none", name)
			}
			if !strings.Contains(err.Error(), "a noun is required") {
				t.Errorf("%s (no args) error = %q, want R18's named refusal", name, err.Error())
			}
		})
	}
}

// TestActivityReviewRebuildDoctorAcceptNoArgsAsRoot is R18: no arguments at
// all is the tree root for these four, unchanged from before nouns existed.
func TestActivityReviewRebuildDoctorAcceptNoArgsAsRoot(t *testing.T) {
	chdirToTestTree(t)

	for name, ctor := range scopeArityReadCmds() {
		t.Run(name, func(t *testing.T) {
			_, err := execRead(ctor(), nil)
			if err != nil {
				t.Errorf("%s (no args, the root): %v", name, err)
			}
		})
	}
}

// TestReadCommandsAcceptDot is R16 across all seven: standing inside the
// entity's own directory is worth as much as typing its noun and chain.
func TestReadCommandsAcceptDot(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(filepath.Join(cwd, "projects", "acme")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	for name, ctor := range allReadCmds() {
		t.Run(name, func(t *testing.T) {
			out, err := execRead(ctor(), []string{"."})
			if err != nil {
				t.Fatalf("%s .: %v (%s)", name, err, out)
			}
		})
	}
}

// TestShowOutputUsesTheDottedAddress is a regression test found while
// implementing this task: show's own header line and its skills-reaching
// line both printed the old plural-bucket locator form directly, a gap
// Phase 18's sweep missed since it covered --json (already correct) but not
// the human-readable path.
func TestShowOutputUsesTheDottedAddress(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	out, err := execRead(newShowCmd(), []string{"project", "acme"})
	if err != nil {
		t.Fatalf("show project acme: %v (%s)", err, out)
	}
	if !strings.Contains(out, "project.acme") {
		t.Errorf("show output = %q, want it to contain %q", out, "project.acme")
	}
	if strings.Contains(out, "projects.acme") {
		t.Errorf("show output = %q, still has the old plural-bucket form", out)
	}
}

// TestShowSkillsCellUsesTheDottedAddress covers the other half of the
// same regression: skillsCell's own "from %s" clause, which
// TestShowOutputUsesTheDottedAddress does not reach at all since it sets up
// no skill.
func TestShowSkillsCellUsesTheDottedAddress(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	var addOut bytes.Buffer
	root.SetOut(&addOut)
	root.SetErr(&addOut)
	root.SetArgs([]string{
		"add", "skill", "signups-report",
		"--name", "Signups report", "--description", "x", "--scope", "project.acme",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("add skill signups-report: %v (%s)", err, addOut.String())
	}

	out, err := execRead(newShowCmd(), []string{"project", "acme"})
	if err != nil {
		t.Fatalf("show project acme: %v (%s)", err, out)
	}
	if !strings.Contains(out, "signups-report (from skill.signups-report") {
		t.Errorf("show output = %q, want the skills line to name the skill's dotted address", out)
	}
	if strings.Contains(out, "skills.signups-report") {
		t.Errorf("show output = %q, still has the old plural-bucket form", out)
	}
}

// TestReviewOutputUsesTheDottedAddress is the same regression, found in
// review's item rows.
func TestReviewOutputUsesTheDottedAddress(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")
	execSetMust(t, []string{"project", "acme", "--status", "blocked", "--note", "waiting"})

	out, err := execRead(newReviewCmd(), []string{"--blocked"})
	if err != nil {
		t.Fatalf("review --blocked: %v (%s)", err, out)
	}
	if !strings.Contains(out, "project.acme") {
		t.Errorf("review output = %q, want it to contain %q", out, "project.acme")
	}
	if strings.Contains(out, "projects.acme") {
		t.Errorf("review output = %q, still has the old plural-bucket form", out)
	}
}

// execSetMust runs execSet and fails the test on error, for setup steps
// this file's own tests depend on rather than are testing.
func execSetMust(t *testing.T, args []string) {
	t.Helper()
	out, err := execSet(args)
	if err != nil {
		t.Fatalf("set %v: %v (%s)", args, err, out)
	}
}
