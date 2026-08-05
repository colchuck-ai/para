// Package cli assembles para's cobra command tree.
package cli

import (
	"context"
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
	return paraerr.ExitCode(err)
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "para",
		Short:         "para manages a PARA-method tree of projects, areas, and resources",
		Version:       version.String(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("para {{.Version}}\n")
	root.AddCommand(newPathCmd())
	return root
}
