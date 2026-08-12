package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/view"
)

// newListCmd implements `para list [<locator>] [filters]` (§16.2).
func newListCmd() *cobra.Command {
	var read readFlags
	var filter filterFlags
	var archived archivedFlag

	cmd := &cobra.Command{
		Use:   "list [<locator>]",
		Short: "list the entities beneath a locator, at any depth",
		Long: "List entities beneath the given locator, defaulting to the whole tree.\n\n" +
			"Containers are transparent: `objectives` and `key-results` are never rows and\n" +
			"are always traversed through. `archive/` is not traversed unless you name it —\n" +
			"archived things are not hidden, they are somewhere else. Items whose status is\n" +
			"terminal are withheld unless --all, and the count of them is reported.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openRead(cmd)
			if err != nil {
				return err
			}
			var scope locator.Locator
			if len(args) == 1 {
				if scope, err = resolveLocatorArg(env.Root, cwd, args[0]); err != nil {
					return err
				}
			}
			opts, err := filter.options(scope)
			if err != nil {
				return err
			}
			res, err := query.List(env, opts)
			if err != nil {
				return err
			}
			if read.json {
				return writeJSON(cmd.OutOrStdout(), listOutputOf(env, res))
			}
			printList(cmd.OutOrStdout(), env, res, read)
			return nil
		},
	}
	read.register(cmd)
	filter.register(cmd)
	archived.register(cmd)
	return cmd
}

// printList is §26's shape: one row per entity, then the count line, then the
// terminal-hiding line when anything was withheld.
func printList(out io.Writer, env *view.Env, res query.Result, read readFlags) {
	// `list` prints no timestamp, only an age — but an age is a count of days on
	// somebody's calendar, so `--local` reaches it here even though there is no
	// instant on the row to convert (§16.2.1).
	zone := read.zone(env)
	var t table
	for _, e := range res.Entities {
		t.add(e.Locator.String(), e.Kind.String(), statusCell(e), ago(env.DaysSinceIn(e.Attention, zone)))
	}
	t.write(out)

	// §17's count line, always, because "showing 3 of 3" is the answer to "is
	// that everything" and a reader cannot tell a complete list from a
	// truncated one by looking at it.
	fmt.Fprintf(out, "showing %d of %d\n", len(res.Entities), res.Total)

	// A second number with a second cause and a second fix (§16.2). Folding it
	// into the first would leave one count explaining neither.
	if res.Hidden > 0 {
		fmt.Fprintf(out, "%d hidden (%s) — --all to include\n",
			res.Hidden, strings.Join(res.HiddenStatuses, ", "))
	}
}

// statusCell is the effective status, or a dash for the kinds that have none —
// an area and a resource, whose archival state is their location (§1.6).
func statusCell(e view.Entity) string {
	if e.EffectiveStatus == "" {
		return dash
	}
	return e.EffectiveStatus
}

// listOutput is `list --json`: the rows, and the three counts that explain them.
//
// `total` and `shown` are §23's, and `hidden` is beside them rather than folded
// into either, for the same reason the human output keeps them apart.
type listOutput struct {
	Total          int          `json:"total"`
	Shown          int          `json:"shown"`
	Hidden         int          `json:"hidden"`
	HiddenStatuses []string     `json:"hidden-statuses"`
	Entities       []entityJSON `json:"entities"`
}

func listOutputOf(env *view.Env, res query.Result) listOutput {
	out := listOutput{
		Total:          res.Total,
		Shown:          len(res.Entities),
		Hidden:         res.Hidden,
		HiddenStatuses: res.HiddenStatuses,
		Entities:       make([]entityJSON, 0, len(res.Entities)),
	}
	if out.HiddenStatuses == nil {
		out.HiddenStatuses = []string{}
	}
	for _, e := range res.Entities {
		out.Entities = append(out.Entities, newEntityJSON(env, e))
	}
	return out
}
