package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// newPathCmd implements `para path <noun> [<chain>]` (R3, R25): the inverse
// of typing a noun and chain, printing one bare line shaped for $(…). R17's
// bucket applies here too: a bare noun prints the bucket's own path.
func newPathCmd() *cobra.Command {
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "path <noun> [<chain>]",
		Short: "print the filesystem path a noun and chain address",
		Long: "Print the absolute path a noun and a chain address, as one bare line and\n" +
			"nothing else — shaped for $(…), which is the whole reason it exists.\n\n" +
			"It is the inverse of the one thing every other command takes, so `cd \"$(para\n" +
			"path project acme)\"` and `para show .` are the two directions of the same\n" +
			"correspondence. It prints the path whether or not anything is there yet.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
			}
			root, err := tree.Find(cwd)
			if err != nil {
				return err
			}
			loc, _, err := parseAddressArgs(root, cwd, args, bucketArity, archived.value)
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
	archived.register(cmd)
	return cmd
}
