package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// newInitCmd is `para init [path]` (§13), the one mutating command that does
// not discover a tree — it makes one, so openEnv's root discovery would fail
// before it started.
func newInitCmd() *cobra.Command {
	var id mutate.Identity
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "create a para tree",
		Long: "Create a para tree: the root marker, the four buckets, the archive's\n" +
			"mirror of the live three, and .agents/ for skills and their derived rules.\n\n" +
			"With no path the current directory becomes the tree. Either way para\n" +
			"refuses to create a tree inside another one.\n\n" +
			"The tree is named after its directory unless --name says otherwise, and\n" +
			"init is the only chance to say: the root has no locator, so no `para set`\n" +
			"can reach it afterwards.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := initTarget(args)
			if err != nil {
				return err
			}
			res, err := mutate.NewEnv(target, clock.FromContext(cmd.Context())).Init(id)
			if err != nil {
				return err
			}
			printInit(cmd, target, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&id.Name, "name", "", "what the tree is called; defaults to the directory's name")
	cmd.Flags().StringVar(&id.Description, "description", "", "one line: what this tree holds")
	return cmd
}

// initTarget is the directory the tree will occupy, as an absolute path.
func initTarget(args []string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
	}
	if len(args) == 0 {
		return cwd, nil
	}
	if filepath.IsAbs(args[0]) {
		return filepath.Clean(args[0]), nil
	}
	return filepath.Join(cwd, args[0]), nil
}

// printInit is §23's file list under `init`'s own verb, plus §26's closing
// note about the Claude Code surface.
//
// The paths are relative to where the command was run rather than to the tree
// root, which is the one place para prints a path that way — and the reason is
// that `para init brain` creates a root the user is not standing in, so
// root-relative paths would print `README.md` for a file that is at
// `brain/README.md` from where they are looking.
//
// §26 compacts the list with brace notation and a "(8 files)" count. §23's "one
// line per file" wins, as it has since Phase 8: the braces are a document
// eliding, and a user who cannot see what was written will not believe it.
func printInit(cmd *cobra.Command, target string, res mutate.Result) {
	out := cmd.OutOrStdout()
	prefix := displayPrefix(target)

	paths := uniquePaths(res.Wrote)
	items := make([]string, 0, len(paths))
	for _, rel := range paths {
		items = append(items, prefix+rel)
	}
	printLabelled(out, "created", items)

	fmt.Fprintln(out)
	fmt.Fprintln(out, "no CLAUDE.md, no .claude/ — enable with `para config set emit.claude true`")
}

// displayPrefix is what every created path is printed under: the target as the
// user named it, or nothing when they named the directory they are standing in.
func displayPrefix(target string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(cwd, target)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel) + "/"
}
