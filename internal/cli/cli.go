// Package cli assembles para's cobra command tree.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/version"
)

// Run parses and executes args against the root command, threading clk
// through cmd.Context() (§0.2) so every subsystem reads the clock from there
// rather than calling time.Now directly, and returns the process exit code.
func Run(clk clock.Clock, args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetContext(clock.WithContext(context.Background(), clk))

	err := root.Execute()
	if err != nil && !paraerr.IsStatus(err) {
		// §26 spells every refusal "error: …", lower case, one line. Cobra's own
		// "Error: …" is silenced above so that the one spelling in the design is
		// the one the user sees.
		fmt.Fprintf(stderr, "error: %s\n", err)
	}
	return paraerr.ExitCode(err)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "para",
		Short: "para manages a PARA-method tree of projects, areas, and resources",
		Long: "para keeps a PARA-method tree — projects, areas, resources, archive — as\n" +
			"plain directories and files: readable without this tool, editable with your\n" +
			"editor, and safe to commit.\n\n" +
			"Everything is addressed by a locator, which is the path with dots for\n" +
			"slashes: `projects.acme.objectives.q1-growth.key-results.signups`. Every\n" +
			"command takes one and every command prints them the same way, so anything you\n" +
			"read pastes into anything you type. `.` means the thing containing the\n" +
			"working directory.\n\n" +
			"What para stores is `.para/state.toml` and an append-only journal beside it.\n" +
			"Everything else — README frontmatter, ACTIVITY.md, MEASUREMENTS.csv, the\n" +
			"rule files — is generated from those on every write, and `para doctor` says\n" +
			"when it has drifted while `para rebuild` puts it back.\n\n" +
			"Start with `para init`. Shell completion knows every locator in the tree:\n" +
			"see `para completion --help`.",
		Version:       version.String(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("para {{.Version}}\n")
	root.AddCommand(newInitCmd())
	root.AddCommand(newPathCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newActivityCmd())
	root.AddCommand(newReviewCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newSetCmd())
	root.AddCommand(newUnsetCmd())
	root.AddCommand(newNoteCmd())
	root.AddCommand(newMeasureCmd())
	root.AddCommand(newMoveCmd())
	root.AddCommand(newArchiveCmd())
	root.AddCommand(newUnarchiveCmd())
	root.AddCommand(newRemoveCmd())
	root.AddCommand(newRebuildCmd())
	root.AddCommand(newDoctorCmd())
	registerCompletions(root)
	return root
}
