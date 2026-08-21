package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// execSet/execUnset run args against `set`/`unset` attached to a stand-in
// parent, the same technique execAdd uses and for the same reason: cobra's
// dispatch to a noun subcommand, or its fallthrough to the parent's own
// RunE for an unmatched word, only behaves the way production does once the
// command genuinely has a parent (cmd.HasParent() == true).
func execSet(args []string) (string, error) {
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newSetCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"set"}, args...))
	err := root.Execute()
	return out.String(), err
}

func execUnset(args []string) (string, error) {
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newUnsetCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"unset"}, args...))
	err := root.Execute()
	return out.String(), err
}

func TestSetDispatchesOnNoun(t *testing.T) {
	names := map[string]bool{}
	for _, sub := range newSetCmd().Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"project", "area", "resource", "objective", "key-result", "skill"} {
		if !names[want] {
			t.Errorf("set has no %q subcommand", want)
		}
	}
	if names["container"] {
		t.Error(`set has a "container" subcommand; a container has nothing you would want to set`)
	}
	if len(names) != 6 {
		t.Errorf("set has %d subcommands, want exactly 6: %v", len(names), names)
	}
}

func TestUnsetDispatchesOnNoun(t *testing.T) {
	names := map[string]bool{}
	for _, sub := range newUnsetCmd().Commands() {
		names[sub.Name()] = true
	}
	for _, want := range []string{"project", "area", "resource", "objective", "key-result", "skill"} {
		if !names[want] {
			t.Errorf("unset has no %q subcommand", want)
		}
	}
	if names["container"] {
		t.Error(`unset has a "container" subcommand`)
	}
}

func setSubcommand(t *testing.T, kind kindmeta.Kind) *cobra.Command {
	t.Helper()
	for _, sub := range newSetCmd().Commands() {
		if sub.Name() == kind.String() {
			return sub
		}
	}
	t.Fatalf("set has no %q subcommand", kind)
	return nil
}

// TestSetSkillHelpShowsExactlyFiveFlags is the task's own acceptance
// example.
func TestSetSkillHelpShowsExactlyFiveFlags(t *testing.T) {
	got := flagNames(setSubcommand(t, kindmeta.KindSkill))
	want := []string{"name", "description", "tags", "created", "scope"}
	for _, f := range want {
		if !got[f] {
			t.Errorf("set skill: missing --%s", f)
		}
	}
	wantSet := map[string]bool{}
	for _, f := range want {
		wantSet[f] = true
	}
	for _, f := range allFieldNames {
		if !wantSet[f] && got[f] {
			t.Errorf("set skill: has --%s, want it absent", f)
		}
	}
}

// TestSetKeyResultExcludesTypeButAddIncludesIt is why set and add cannot
// share one registration unconditionally: type is RequiredFixed for
// key-result, so add (creation) offers it and set (mutation) must not.
func TestSetKeyResultExcludesTypeButAddIncludesIt(t *testing.T) {
	setFlags := flagNames(setSubcommand(t, kindmeta.KindKeyResult))
	if setFlags["type"] {
		t.Error("set key-result has --type, which is fixed at creation and can never be changed")
	}
	for _, f := range []string{"name", "description", "status", "due", "tags", "created", "start", "target"} {
		if !setFlags[f] {
			t.Errorf("set key-result: missing --%s", f)
		}
	}

	addFlags := flagNames(findAddSubcommand(t, "key-result"))
	if !addFlags["type"] {
		t.Error("add key-result has no --type, but type is RequiredFixed and add is creation")
	}
}

func TestSetInapplicableFlagIsAnUnknownFlagError(t *testing.T) {
	_, err := execSet([]string{"project", "acme", "--type", "ratio"})
	if err == nil {
		t.Fatal("set project --type: want an error, got none")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("set project --type: error = %q, want an unknown-flag parse error", err.Error())
	}
}

// TestSetContainerIsRefused deliberately gives no flags: the parent `set`
// command (unlike its per-noun subcommands) registers none, so a flag here
// would fail cobra's own parsing before ever reaching the container check —
// a different error than the one this test is proving.
func TestSetContainerIsRefused(t *testing.T) {
	_, err := execSet([]string{"container", "acme.objectives"})
	if err == nil {
		t.Fatal("set container: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "container is refused") {
		t.Errorf("set container: error = %q, want it to name container's own refusal", err.Error())
	}
}

func TestUnsetContainerIsRefused(t *testing.T) {
	_, err := execUnset([]string{"container", "acme.objectives", "tags"})
	if err == nil {
		t.Fatal("unset container: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "container is refused") {
		t.Errorf("unset container: error = %q, want it to name container's own refusal", err.Error())
	}
}

func TestSetWithNoArgsShowsHelp(t *testing.T) {
	out, err := execSet(nil)
	if err != nil {
		t.Fatalf("set (no args): want help and exit 0, got error %v", err)
	}
	if !strings.Contains(out, "Available Commands") {
		t.Errorf("set (no args): output = %q, want help text", out)
	}
}

// TestUnsetOfAnInapplicableFieldIsStillNamedByMutate is a regression check
// that the noun-dispatch rewiring did not lose the field-name validation
// mutate.Unset already does: `unset project acme type` is refused because a
// project has no type field at all, exactly as before dispatch existed —
// this task deliberately does not duplicate that rule at the CLI layer.
func TestUnsetOfAnInapplicableFieldIsStillNamedByMutate(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	_, err := execUnset([]string{"project", "acme", "type"})
	if err == nil {
		t.Fatal("unset project acme type: want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "type") {
		t.Errorf("unset project acme type: error = %q, want it to name the field", err.Error())
	}
}

// mustAddViaCLI creates an entity through the real add dispatch, for tests
// that need one to exist before exercising set/unset against it.
func mustAddViaCLI(t *testing.T, noun, id string) {
	t.Helper()
	root := &cobra.Command{Use: "para"}
	root.AddCommand(newAddCmd())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"add", noun, id, "--name", id, "--description", "x"})
	if err := root.Execute(); err != nil {
		t.Fatalf("add %s %s: %v (output: %s)", noun, id, err, out.String())
	}
}

// TestSetOutputUsesTheDottedAddress is a regression test found while
// implementing P19.8: output.go's changeLines (shared by set and unset)
// printed res.Locator raw — the old plural-bucket form — rather than
// through entityLocatorString, unnoticed because no existing test checked
// set's stdout content, only that the command succeeded.
func TestSetOutputUsesTheDottedAddress(t *testing.T) {
	chdirToTestTree(t)
	mustAddViaCLI(t, "project", "acme")

	out, err := execSet([]string{"project", "acme", "--description", "updated"})
	if err != nil {
		t.Fatalf("set project acme --description updated: %v (%s)", err, out)
	}
	if !strings.Contains(out, "project.acme") {
		t.Errorf("set output = %q, want it to contain %q", out, "project.acme")
	}
	if strings.Contains(out, "projects.acme") {
		t.Errorf("set output = %q, still has the old plural-bucket form", out)
	}
}

// TestSetDotResolvesAgainstCwdAndChecksKind is R16's "." carried into a
// noun-dispatched command: the noun is fixed by the subcommand, so "."
// cannot occupy an argument slot the way it does for show/note — it can
// only be checked against what it resolves to.
func TestSetDotResolvesAgainstCwdAndChecksKind(t *testing.T) {
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

	// The right noun: "." resolves to the project itself.
	if _, err := execSet([]string{"project", ".", "--description", "updated"}); err != nil {
		t.Errorf("set project .: %v", err)
	}

	// The wrong noun: "." still resolves to the project, and key-result
	// must refuse rather than silently act on a mismatched kind.
	_, err = execSet([]string{"key-result", ".", "--description", "updated"})
	if err == nil {
		t.Fatal("set key-result . (cwd is a project): want a refusal, got none")
	}
	if !strings.Contains(err.Error(), "not") {
		t.Errorf("set key-result . error = %q, want it to name the kind mismatch", err.Error())
	}
}
