package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// newPathCmd implements `para path <locator>` (§14): the inverse of typing a
// locator, printing one bare line shaped for $(…).
func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <locator>",
		Short: "print the filesystem path a locator addresses",
		Long: "Print the absolute path a locator addresses, as one bare line and nothing\n" +
			"else — shaped for $(…), which is the whole reason it exists.\n\n" +
			"It is the inverse of the one thing every other command takes, so `cd \"$(para\n" +
			"path projects.acme)\"` and `para show .` are the two directions of the same\n" +
			"correspondence. It prints the path whether or not anything is there yet.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
			}
			root, err := tree.Find(cwd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(root, cwd, args[0])
			if err != nil {
				return err
			}
			abs, err := tree.ResolvePath(root, loc)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), abs)
			return nil
		},
	}
}

// resolveLocatorArg parses a command-line locator argument, honoring the
// "." convenience (§14): the entity or container containing the working
// directory.
func resolveLocatorArg(root, cwd, arg string) (locator.Locator, error) {
	if arg == "." {
		return tree.ResolveDot(root, cwd)
	}
	return locator.Parse(arg)
}
