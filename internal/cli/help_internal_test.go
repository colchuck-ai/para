package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/kindmeta"
)

// The `--help` review (implementation plan, Phase 15, task 5) as properties
// rather than as a reading. What a human has to check is that the prose agrees
// with §13–§23; what a test can check is that the prose exists, that the usage
// line spells the command it belongs to, and that no flag's help is silently
// eaten by the library that prints it.

// walkTestCommands visits every command in a freshly built tree, using the
// production walk rather than a second copy of the same recursion.
func walkTestCommands(t *testing.T, visit func(*cobra.Command)) {
	t.Helper()
	walkCommands(newRootCmd(), visit)
}

// generated names the commands cobra contributes. Their help is cobra's prose,
// not para's, and holding it to para's shape would be asserting on a library.
var generated = map[string]bool{"completion": true, "help": true, "bash": true, "zsh": true, "fish": true, "powershell": true}

func TestEveryCommandHasShortAndLongHelp(t *testing.T) {
	walkTestCommands(t, func(cmd *cobra.Command) {
		if generated[cmd.Name()] {
			return
		}
		if strings.TrimSpace(cmd.Short) == "" {
			t.Errorf("%q has no Short", cmd.CommandPath())
		}
		if strings.TrimSpace(cmd.Long) == "" {
			t.Errorf("%q has no Long — every verb owes §13 an explanation of what it does", cmd.CommandPath())
		}
	})
}

func TestEveryUseLineStartsWithItsCommandName(t *testing.T) {
	walkTestCommands(t, func(cmd *cobra.Command) {
		if name, _, _ := strings.Cut(cmd.Use, " "); name != cmd.Name() {
			t.Errorf("%q has Use %q, which does not start with the command's name", cmd.CommandPath(), cmd.Use)
		}
	})
}

// TestFlagUsageCarriesNoBackquotes is the one that found something.
//
// pflag reads a back-quoted word in a usage string as the *name of the flag's
// value* and strips it out of the text — so `--tags "a boolean tag expression:
// `+"`not`"+` → …"` printed as `--tags not   a boolean tag expression: not → …`,
// naming the value "not" and losing the emphasis it was reaching for. The
// convention is invisible until it fires, so it is asserted rather than
// remembered.
func TestFlagUsageCarriesNoBackquotes(t *testing.T) {
	walkTestCommands(t, func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if strings.Contains(f.Usage, "`") {
				t.Errorf("%q's --%s usage contains a backquote, which pflag reads as the value's name: %q",
					cmd.CommandPath(), f.Name, f.Usage)
			}
		})
	})
}

// TestAddAndSetHelpListTheNouns is R27: `para add --help` and `para set
// --help` list the nouns, since neither shows its per-noun flags until one
// is named. unset is checked too — it dispatches on the noun exactly the
// same way (P19.5) — though R27's own text names only add and set.
func TestAddAndSetHelpListTheNouns(t *testing.T) {
	cmds := map[string]*cobra.Command{
		"add":   newAddCmd(),
		"set":   newSetCmd(),
		"unset": newUnsetCmd(),
	}
	// Derived from kindmeta.AddressableKindWords() rather than a second
	// hand-written list: TestAddDispatchesOnNoun already holds that
	// function's seven nouns to a hardcoded, independently-checked list, so
	// drift between kindmeta and *that* test is caught there — this test's
	// own job is narrower, whether the help text agrees with what the command
	// actually computes.
	wantLine := "Nouns: " + strings.Join(kindmeta.AddressableKindWords(), ", ") + "."
	for name, cmd := range cmds {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(cmd.Long, wantLine) {
				t.Errorf("%s --help does not contain %q; Long = %q", name, wantLine, cmd.Long)
			}
		})
	}
}

// TestRootHelpUnchangedInShape confirms R27 only touches add's and set's
// own help — the root's still lists the nineteen top-level commands and
// says nothing about nouns, which belong to the two commands that dispatch
// on one.
func TestRootHelpUnchangedInShape(t *testing.T) {
	root := newRootCmd()
	if strings.Contains(root.Long, "Nouns:") {
		t.Errorf("root --help mentions nouns; that belongs to add/set/unset's own help, not the root's")
	}
	for _, name := range []string{"add", "set", "show", "config"} {
		found := false
		for _, cmd := range root.Commands() {
			if cmd.Name() == name {
				found = true
			}
		}
		if !found {
			t.Errorf("root --help's command tree has no %q", name)
		}
	}
}

// TestRootHelpDescribesTheNounAndChainGrammar is the one place Phase 22's
// sweep missed: the root's own prose, unlike every subcommand's, was never
// transcribed off the old locator model. It must speak R1's model — a noun
// and a short id-chain — rather than a single dotted locator string, and it
// must not still carry a worked example in the old plural-bucket form.
func TestRootHelpDescribesTheNounAndChainGrammar(t *testing.T) {
	root := newRootCmd()
	if !strings.Contains(root.Long, "a noun and a short id-chain") {
		t.Errorf("root --help does not describe the noun+chain address model; Long = %q", root.Long)
	}
	if strings.Contains(root.Long, "projects.acme") || strings.Contains(root.Long, "every locator in the tree") {
		t.Errorf("root --help still carries an old-grammar locator example; Long = %q", root.Long)
	}
}

// TestEveryVerbInTheDesignIsACommand holds §13's list, which is the command
// surface itself: eighteen verbs and config's four shapes.
func TestEveryVerbInTheDesignIsACommand(t *testing.T) {
	verbs := []string{
		"init", "add", "show", "list", "set", "unset", "move", "remove", "archive",
		"unarchive", "note", "measure", "log", "activity", "review", "rebuild", "path", "doctor", "config",
	}
	have := map[string]bool{}
	for _, cmd := range newRootCmd().Commands() {
		have[cmd.Name()] = true
	}
	for _, verb := range verbs {
		if !have[verb] {
			t.Errorf("§13's %q is not a command", verb)
		}
	}

	var config *cobra.Command
	for _, cmd := range newRootCmd().Commands() {
		if cmd.Name() == "config" {
			config = cmd
		}
	}
	if config == nil {
		t.Fatal("config is not a command")
	}
	shapes := map[string]bool{}
	for _, sub := range config.Commands() {
		shapes[sub.Name()] = true
	}
	for _, shape := range []string{"set", "unset", "list", "show"} {
		if !shapes[shape] {
			t.Errorf("§22's `config %s` is not a subcommand", shape)
		}
	}
}
