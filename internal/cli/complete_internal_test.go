package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
)

// The completion's pure parts, at the unit altitude §0.1 asks for. What a
// candidate set contains against a real tree is testdata/script/completion.txtar's
// job; these are the three functions that decide *what could be offered* without
// reading a filesystem at all.

func TestWithPrefixKeepsOnlyWhatExtendsWhatIsTyped(t *testing.T) {
	all := []string{"projects", "projects.acme", "projects.acme.objectives", "areas", "areas.health"}
	cases := []struct {
		toComplete string
		want       string
	}{
		{"", "projects,projects.acme,projects.acme.objectives,areas,areas.health"},
		{"pro", "projects,projects.acme,projects.acme.objectives"},
		{"projects.acme.", "projects.acme.objectives"},
		{"areas.health", "areas.health"},
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

// TestCanNest is `add`'s question — may a new id be created directly beneath
// this locator — answered by §1.3's derivation rather than by a list here.
func TestCanNest(t *testing.T) {
	cases := []struct {
		loc  locator.Locator
		want bool
	}{
		{locator.Locator{"projects"}, true},
		{locator.Locator{"areas"}, true},
		{locator.Locator{"resources"}, true},
		{locator.Locator{"skills"}, true},
		{locator.Locator{"areas", "health"}, true},   // areas nest
		{locator.Locator{"resources", "rust"}, true}, // so do resources
		{locator.Locator{"projects", "acme"}, false}, // a project's children are objectives/
		{locator.Locator{"skills", "commit"}, false}, // skills are one level (§1.3)
		{locator.Locator{"projects", "acme", "objectives"}, true},
		{locator.Locator{"projects", "acme", "objectives", "q1", "key-results"}, true},
		{locator.Locator{"archive"}, false},  // things arrive by `archive` (§1.6)
		{locator.Locator{"nonsense"}, false}, // not a bucket
		{nil, false},                         // the root has no locator to nest under
	}
	for _, c := range cases {
		if got := canNest(c.loc); got != c.want {
			t.Errorf("canNest(%q) = %v, want %v", c.loc, got, c.want)
		}
	}
}

func TestKindOfArgReadsTheLocatorAlone(t *testing.T) {
	cases := []struct {
		arg  string
		kind kindmeta.Kind
		ok   bool
	}{
		{"projects.acme", kindmeta.KindProject, true},
		{"areas.health.training", kindmeta.KindArea, true},
		{"skills.commit-style", kindmeta.KindSkill, true},
		{"projects.acme.objectives.q1.key-results.signups", kindmeta.KindKeyResult, true},
		{"archive.projects.acme", kindmeta.KindProject, true},     // archival never changes the kind
		{"projects.acme.objectives", kindmeta.KindUnknown, false}, // a container has no locator of its own
		{"", kindmeta.KindUnknown, false},
		{".", kindmeta.KindUnknown, false}, // resolving it would mean reading the filesystem
		{"nonsense", kindmeta.KindUnknown, false},
		{"projects..acme", kindmeta.KindUnknown, false},
	}
	for _, c := range cases {
		kind, ok := kindOfArg(c.arg)
		if kind != c.kind || ok != c.ok {
			t.Errorf("kindOfArg(%q) = %v, %v; want %v, %v", c.arg, kind, ok, c.kind, c.ok)
		}
	}
}

// TestCompletionsAreRegisteredForEveryLocatorTakingCommand is the property the
// script test cannot state: that no command was left out of registerCompletions.
// A verb with no ValidArgsFunction falls back to the shell's filename
// completion, which is always wrong here (§13 has one path argument, `init`'s).
func TestEveryCommandCompletesItsArguments(t *testing.T) {
	root := newRootCmd()
	for _, cmd := range root.Commands() {
		if generated[cmd.Name()] || cmd.Name() == "config" {
			continue
		}
		if cmd.ValidArgsFunction == nil {
			t.Errorf("%q has no argument completion, so the shell will offer filenames", cmd.Name())
		}
	}
	for _, sub := range commandNamed(t, root, "config").Commands() {
		if generated[sub.Name()] {
			continue
		}
		if sub.ValidArgsFunction == nil {
			t.Errorf("config %q has no argument completion", sub.Name())
		}
	}
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
