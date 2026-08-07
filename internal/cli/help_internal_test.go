package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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
