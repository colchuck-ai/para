package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// chdirToTestTree points the process cwd at a freshly built para tree —
// built through the real `init` command rather than a hand-planted
// fixture, so every bucket exists exactly as it would for a user — and
// restores the original cwd on cleanup. Some refusals — R17's bucket
// refusal among them — only fire once tree.Find has succeeded, since
// openEnv runs before the chain is parsed (the same order every other
// mutating command already uses).
func chdirToTestTree(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	initCmd := newInitCmd()
	var out bytes.Buffer
	initCmd.SetOut(&out)
	initCmd.SetErr(&out)
	initCmd.SetArgs([]string{"--name", "Test"})
	if err := initCmd.Execute(); err != nil {
		t.Fatalf("init: %v (%s)", err, out.String())
	}
}

// allFieldNames is every §15 field flag name, for checking that a per-noun
// subcommand carries exactly the ones kindmeta.Has(kind, field) says it
// should and none of the others.
var allFieldNames = []string{
	"name", "description", "status", "priority", "due",
	"tags", "created", "type", "start", "target", "scope",
}

func findAddSubcommand(t *testing.T, noun string) *cobra.Command {
	t.Helper()
	for _, sub := range newAddCmd().Commands() {
		if sub.Name() == noun {
			return sub
		}
	}
	t.Fatalf("add has no %q subcommand", noun)
	return nil
}

// TestNounDispatchShortTextUsesTheRightArticle is a regression test for a
// review finding: add's, set's, and unset's per-noun Short text built "a "
// + kind.String() by hand instead of withArticle, so `para add --help`
// printed "create a area" and "create a objective" — exactly the typo
// withArticle exists to avoid.
func TestNounDispatchShortTextUsesTheRightArticle(t *testing.T) {
	dispatchers := map[string]*cobra.Command{
		"add":   newAddCmd(),
		"set":   newSetCmd(),
		"unset": newUnsetCmd(),
	}
	for verb, parent := range dispatchers {
		for _, sub := range parent.Commands() {
			if strings.Contains(sub.Short, "a area") || strings.Contains(sub.Short, "a objective") {
				t.Errorf("%s %s: Short = %q, wrong article", verb, sub.Name(), sub.Short)
			}
		}
	}
}

func flagNames(cmd *cobra.Command) map[string]bool {
	out := map[string]bool{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		out[f.Name] = true
	})
	return out
}

// assertAddFields checks a per-noun add subcommand carries exactly the
// fields listed in want and no other §15 field — the property task P19.3
// exists for: the matrix, not eleven flags on every command, decides.
func assertAddFields(t *testing.T, noun string, want []string) {
	t.Helper()
	got := flagNames(findAddSubcommand(t, noun))
	wantSet := map[string]bool{}
	for _, f := range want {
		wantSet[f] = true
		if !got[f] {
			t.Errorf("add %s: missing --%s", noun, f)
		}
	}
	for _, f := range allFieldNames {
		if !wantSet[f] && got[f] {
			t.Errorf("add %s: has --%s, want it absent (not in %s's §15 row)", noun, f, noun)
		}
	}
}

// TestAddDispatchesOnNoun is R3/R15: one subcommand per addressable noun,
// container refused because it is created eagerly by its parent and never
// directly.
func TestAddDispatchesOnNoun(t *testing.T) {
	cmd := newAddCmd()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"project", "area", "resource", "objective", "key-result", "skill"} {
		if !names[want] {
			t.Errorf("add has no %q subcommand", want)
		}
	}
	if names["container"] {
		t.Error(`add has a "container" subcommand; R15 refuses it`)
	}
	if len(names) != 6 {
		t.Errorf("add has %d subcommands, want exactly 6: %v", len(names), names)
	}
}

// TestAddProjectHelpShowsExactlySevenFields is the task's own acceptance
// example: a project has seven §15 fields, not eleven.
func TestAddProjectHelpShowsExactlySevenFields(t *testing.T) {
	assertAddFields(t, "project", []string{"name", "description", "status", "priority", "due", "tags", "created"})
}

// TestAddKeyResultHelpShowsExactlyNineFields is the task's other example:
// status is Derived rather than Optional but still offered at creation
// (only `set` excludes a RequiredFixed field the way it excludes nothing
// else), and scope belongs to skill alone.
func TestAddKeyResultHelpShowsExactlyNineFields(t *testing.T) {
	assertAddFields(t, "key-result", []string{
		"name", "description", "status", "due", "tags", "created", "type", "start", "target",
	})
}

// TestAddAreaAndResourceHaveNoStatusOrPriority is §15's row for the two
// kinds with no lifecycle at all — their archival state is their location
// (§1.6) — checked because it is the row most likely to accidentally offer
// something section 15 does not give it once fields stop being registered
// unconditionally.
func TestAddAreaAndResourceHaveNoStatusOrPriority(t *testing.T) {
	assertAddFields(t, "area", []string{"name", "description", "priority", "tags", "created"})
	assertAddFields(t, "resource", []string{"name", "description", "tags", "created"})
}

// TestAddSkillHasScopeAndNothingElseNoOtherKindHas confirms scope is
// skill's alone.
func TestAddSkillHasScope(t *testing.T) {
	assertAddFields(t, "skill", []string{"name", "description", "tags", "created", "scope"})
}

// TestAddInapplicableFlagIsAnUnknownFlagError is the task's third example:
// --type on a project must be a parse-time refusal, not the runtime one
// mutate.Add used to produce when every flag was registered on every kind.
func TestAddInapplicableFlagIsAnUnknownFlagError(t *testing.T) {
	// Built standalone, not fetched from newAddCmd()'s tree: cobra's Execute
	// always redirects to Root() when a command has a parent, so a command
	// still attached to `add` cannot be exercised on its own args this way.
	cmd := newAddNounCmd(kindmeta.KindProject)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"acme", "--type", "ratio", "--name", "Acme", "--description", "x"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("add project --type: want an error, got none")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("add project --type: error = %q, want an unknown-flag parse error", err.Error())
	}
}

// TestAddRefusesArchived confirms P19.2's refusal survives the noun-dispatch
// restructuring: --archived is registered (and refused) on each per-noun
// subcommand, not just on the now-childless top-level `add`.
func TestAddRefusesArchived(t *testing.T) {
	cmd := newAddNounCmd(kindmeta.KindArea)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"health", "--archived", "--name", "Health", "--description", "x"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("add area --archived: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "nothing is created under archive/") {
		t.Errorf("add area --archived: error = %q, want the named refusal", err.Error())
	}
}

// execAdd runs args against `add` attached to a real root, the way
// `para add …` actually dispatches. Cobra's own "unknown command" check
// (legacyArgs) behaves differently depending on cmd.HasParent() — calling
// Execute() on a bare, unattached newAddCmd() makes cobra treat `add` as if
// it were the root itself and produce cobra's own "unknown command" error
// before ever reaching add's RunE, which is not what happens when `add` is
// where it really lives, one level under `para`. Attaching it to a minimal
// stand-in root reproduces the real HasParent()==true dispatch.
func execAdd(t *testing.T, args []string) (string, error) {
	t.Helper()
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"add"}, args...))
	err := root.Execute()
	return out.String(), err
}

// TestAddContainerIsRefused is R15, exercised through the real dispatch
// path: only once `add` is attached under a parent does cobra fall through
// to `add`'s own RunE for an unmatched first argument — exactly what
// `para add container ...` does — instead of the "unknown command" cobra
// gives an unparented command that fails to match a subcommand.
func TestAddContainerIsRefused(t *testing.T) {
	_, err := execAdd(t, []string{"container", "acme.objectives"})
	if err == nil {
		t.Fatal("add container: want a refusal, got none (a silent no-op is indistinguishable from success)")
	}
	if !strings.Contains(err.Error(), "container is refused") {
		t.Errorf("add container: error = %q, want it to name container's own refusal", err.Error())
	}
}

// TestAddUnknownNounIsRefused is the general case TestAddContainerIsRefused
// is one instance of: any word that is not one of the six add offers must
// be a named refusal, not cobra's default "print help, exit 0" for an
// unmatched subcommand.
func TestAddUnknownNounIsRefused(t *testing.T) {
	_, err := execAdd(t, []string{"bogus-noun", "acme"})
	if err == nil {
		t.Fatal("add bogus-noun: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), `"bogus-noun" is not a noun add takes`) {
		t.Errorf("add bogus-noun: error = %q, want it to name the bad word", err.Error())
	}
}

// TestAddWithNoArgsShowsHelp confirms the fix for the two tests above did
// not turn a bare `para add` into a refusal too — R18's "no args is the
// root" does not apply to add (add always needs a noun and a chain), but
// showing help and exiting 0 is still the right behavior for "you gave add
// nothing to do", matching every other cobra parent-dispatch command.
func TestAddWithNoArgsShowsHelp(t *testing.T) {
	out, err := execAdd(t, []string{})
	if err != nil {
		t.Fatalf("add (no args): want help and exit 0, got error %v", err)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("add (no args): output = %q, want help text", out)
	}
}

// TestAddBareNounRefusedAsABucket is R17 applied to add's new shape: with
// the noun now the subcommand name, the chain is the only positional
// argument left, and omitting it must still be refused by name rather than
// treated as "nothing to do" or a generic cobra arity error.
func TestAddBareNounRefusedAsABucket(t *testing.T) {
	chdirToTestTree(t)
	cmd := newAddNounCmd(kindmeta.KindProject)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--name", "Acme", "--description", "x"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("add project (no chain): want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "project names a bucket, not an entity") {
		t.Errorf("add project (no chain): error = %q, want R17's named refusal", err.Error())
	}
}
