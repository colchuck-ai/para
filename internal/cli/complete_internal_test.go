package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
)

// The completion's pure parts, at the unit altitude §0.1 asks for. What a
// candidate set contains against a real tree is testdata/script/completion.txtar's
// job; these are the functions that decide *what could be offered* without
// reading a filesystem at all.

func TestWithPrefixKeepsOnlyWhatExtendsWhatIsTyped(t *testing.T) {
	all := []string{"project", "project.acme", "project.acme.q1", "area", "area.health"}
	cases := []struct {
		toComplete string
		want       string
	}{
		{"", "project,project.acme,project.acme.q1,area,area.health"},
		{"pro", "project,project.acme,project.acme.q1"},
		{"project.acme.", "project.acme.q1"},
		{"area.health", "area.health"},
		{"nothing", ""},
	}
	for _, c := range cases {
		if got := strings.Join(withPrefix(all, c.toComplete), ","); got != c.want {
			t.Errorf("withPrefix(_, %q) = %q, want %q", c.toComplete, got, c.want)
		}
	}
	// The order given is the order returned: the callers sort before filtering,
	// and a filter that reordered would undo that.
	if got := withPrefix(all, ""); strings.Join(got, ",") != strings.Join(all, ",") {
		t.Errorf("withPrefix reordered its input: %v", got)
	}
}

func TestSplitLastComma(t *testing.T) {
	cases := []struct {
		in       string
		wantHead string
		wantLast string
	}{
		{"", "", ""},
		{"project.acme", "", "project.acme"},
		{"project.acme,area.g", "project.acme,", "area.g"},
		{"project.acme,area.growth,", "project.acme,area.growth,", ""},
	}
	for _, c := range cases {
		head, last := splitLastComma(c.in)
		if head != c.wantHead || last != c.wantLast {
			t.Errorf("splitLastComma(%q) = %q, %q; want %q, %q", c.in, head, last, c.wantHead, c.wantLast)
		}
	}
}

// TestAddParentIsRestNestingStructure is task P21.3's own rule stated as a
// table: only the nouns that nest (R3) have a parent to complete, and each
// nests under the noun R3's table actually names.
func TestAddParentIsNestingStructure(t *testing.T) {
	cases := []struct {
		kind kindmeta.Kind
		want kindmeta.Kind
		has  bool
	}{
		{kindmeta.KindProject, kindmeta.KindUnknown, false},
		{kindmeta.KindSkill, kindmeta.KindUnknown, false},
		{kindmeta.KindArea, kindmeta.KindArea, true},
		{kindmeta.KindResource, kindmeta.KindResource, true},
		{kindmeta.KindObjective, kindmeta.KindProject, true},
		{kindmeta.KindKeyResult, kindmeta.KindObjective, true},
	}
	for _, c := range cases {
		got, ok := addParent[c.kind]
		if ok != c.has || (ok && got != c.want) {
			t.Errorf("addParent[%s] = %s, %v; want %s, %v", c.kind, got, ok, c.want, c.has)
		}
	}
}

func TestPrefixDots(t *testing.T) {
	out, dir := prefixDots([]string{"acme", "acme-migration"}, "acme")
	if strings.Join(out, ",") != "acme.,acme-migration." {
		t.Errorf("prefixDots = %v", out)
	}
	if dir != cobra.ShellCompDirectiveNoSpace|noFiles {
		t.Errorf("prefixDots directive = %v", dir)
	}
}

// TestNounSubcommandFindsExactlyTheSixAddableNouns is R15 read off the
// dispatch tree rather than off a list: add never registers a `container`
// subcommand, so nounSubcommand must fail to find one, and must find every
// one of the six it does register.
func TestNounSubcommandFindsExactlyTheSixAddableNouns(t *testing.T) {
	add := commandNamed(t, newRootCmd(), "add")
	for _, kind := range kindmeta.AllKinds() {
		if nounSubcommand(add, kind) == nil {
			t.Errorf("add has no %q subcommand", kind.String())
		}
	}
	if got := nounSubcommand(add, kindmeta.KindContainer); got != nil {
		t.Errorf("add has a %q subcommand: %v", "container", got.Name())
	}
}

// TestEveryCommandCompletesItsArguments is the property the script test
// cannot state: that no command that takes its own positional argument was
// left out of registerCompletions. A verb with no ValidArgsFunction falls
// back to the shell's filename completion, which is always wrong here (R12
// has one path argument, `init`'s). Parent commands that only dispatch to
// subcommands (add, set, unset, config) need none of their own: cobra
// completes subcommand names natively.
func TestEveryCommandCompletesItsArguments(t *testing.T) {
	walkTestCommands(t, func(cmd *cobra.Command) {
		if generated[cmd.Name()] || len(cmd.Commands()) > 0 {
			return
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%q has no argument completion, so the shell will offer filenames", cmd.CommandPath())
		}
	})
}

// TestNoFlagFallsBackToFilenames covers denyFileCompletion: every flag on every
// command answers something, even if that something is "nothing".
func TestNoFlagFallsBackToFilenames(t *testing.T) {
	walkCommands(newRootCmd(), func(cmd *cobra.Command) {
		if generated[cmd.Name()] {
			return
		}
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if _, ok := cmd.GetFlagCompletionFunc(f.Name); !ok {
				t.Errorf("%q's --%s has no completion, so the shell will offer filenames", cmd.CommandPath(), f.Name)
			}
		})
	})
}

// TestFieldValueCompletionIsPerNoun is task P21.8: the value vocabulary for
// add's and set's own field flags comes from the kind the subcommand is
// already dispatched to, with no special case for the one field (a
// key-result's status) whose closed vocabulary genuinely varies by kind.
func TestFieldValueCompletionIsPerNoun(t *testing.T) {
	root := newRootCmd()
	kr := nounSubcommand(commandNamed(t, root, "add"), address.KeyResult)
	if kr == nil {
		t.Fatal("no add key-result subcommand")
	}
	fn, ok := kr.GetFlagCompletionFunc("status")
	if !ok {
		t.Fatal("add key-result --status has no completion")
	}
	got, _ := fn(kr, nil, "")
	if strings.Join(got, ",") != "dropped" {
		t.Errorf("add key-result --status completes %v, want only [dropped]", got)
	}

	// area has no status field at all (§15), so add never registers the flag
	// — the kind-specific vocabulary is never even reachable, rather than
	// reachable and empty.
	area := nounSubcommand(commandNamed(t, root, "add"), address.Area)
	if area.Flags().Lookup("status") != nil {
		t.Error("add area registers --status, which §15 gives it no field for")
	}
}

func commandNamed(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	t.Fatalf("no %q command", name)
	return nil
}
