package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/review"
	"github.com/colchuck-ai/para/internal/view"
)

// newReviewCmd implements `para review [<locator>]` (§20).
func newReviewCmd() *cobra.Command {
	var read readFlags
	var all bool
	var limit int
	selected := map[review.Group]*bool{}

	cmd := &cobra.Command{
		Use:   "review [<locator>]",
		Short: "list what is worth looking at, grouped by reason",
		Long: "Group what needs attention by why it needs it: stale, blocked, overdue,\n" +
			"behind, and skills nobody has touched. Naming no group runs all five.\n\n" +
			"Ordering within a group is by distance past the threshold, which is why\n" +
			"there is no --sort. Terminal items and archived things are excluded unless\n" +
			"--all. It always exits 0: having work is not a failure, and a command that\n" +
			"fails whenever you have work is a command you stop running.",
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
			if limit < 0 {
				return paraerr.Newf(paraerr.KindValidation, "--limit cannot be negative")
			}

			var only []review.Group
			for _, g := range review.Groups() {
				if *selected[g] {
					only = append(only, g)
				}
			}

			res, err := review.Run(env, review.Options{
				Scope: scope, Only: only, All: all, Limit: limit,
			})
			if err != nil {
				return err
			}
			if read.json {
				return writeJSON(cmd.OutOrStdout(), reviewOutputOf(env, res))
			}
			printReview(cmd.OutOrStdout(), res)
			return nil
		},
	}

	for _, g := range review.Groups() {
		selected[g] = cmd.Flags().Bool(string(g), false, groupHelp[g])
	}
	cmd.Flags().BoolVar(&all, "all", false, "include terminal-status items and archived things")
	cmd.Flags().IntVar(&limit, "limit", 0, "print at most this many in each group")

	// §16.2.1 names `review` among the commands carrying `--local`, so it is
	// here — and it has nothing to convert, which is the honest outcome rather
	// than an oversight. Every number `review` prints is a comparison against a
	// committed threshold, and §7's answer to "is this stale" must not depend on
	// which continent asked (the argument §3.5 makes for the generated files,
	// and the one `show --local` already follows by keeping its staleness
	// verdict in UTC). The only date on a row is a `due`, which is never
	// converted at all: it is a day somebody chose, not an instant something
	// happened at (§15.1).
	read.register(cmd)
	return cmd
}

// groupHelp is §20's table as help text, one row per flag.
var groupHelp = map[review.Group]string{
	review.GroupStale:   "no note or measurement within <kind>.stale-after",
	review.GroupBlocked: "status is blocked — no timer, blocked is always listed",
	review.GroupOverdue: "open and past due",
	review.GroupBehind:  "a key-result whose pace is below key-result.at-risk-pace",
	review.GroupSkills:  "a skill untouched for longer than review.cadence",
}

// printReview is §26's shape: a heading naming the group and its count, then one
// row per finding, indented.
//
// One table across every group, not one per group, so the columns line up down
// the whole output — which is how §26 prints it, and what makes two groups'
// numbers comparable at a glance.
func printReview(out io.Writer, res review.Result) {
	if len(res.Sections) == 0 {
		// A command that prints nothing at all leaves "did it run?" and "is
		// there nothing?" looking identical, which for the command you run
		// every morning is the wrong pair to conflate.
		fmt.Fprintln(out, "nothing to review")
		return
	}

	t := table{indent: "  "}
	for _, s := range res.Sections {
		t.head(sectionHeading(s))
		for _, item := range s.Items {
			t.add(item.Entity.Locator.String(), measureCell(s.Group, item), thresholdCell(s.Group, item))
		}
	}
	t.write(out)
}

// sectionHeading is §26's `stale (3)`, and names both counts when --limit cut
// the group short — §17's count line applied where the counts live.
func sectionHeading(s review.Section) string {
	if len(s.Items) < s.Total {
		return fmt.Sprintf("%s (%d of %d)", s.Group, len(s.Items), s.Total)
	}
	return fmt.Sprintf("%s (%d)", s.Group, s.Total)
}

// measureCell is the middle column: the value that put the item in the group,
// spelled in the group's own units.
func measureCell(g review.Group, item review.Item) string {
	switch g {
	case review.GroupBehind:
		return "pace " + number(item.Entity.KeyResult.Outlook.Pace)
	case review.GroupOverdue:
		return days(item.Days) + " over"
	default:
		return days(item.Days)
	}
}

// thresholdCell is the right column: what the middle one is past.
//
// The knob is named in full — `area.stale-after 30` where §26 writes
// `stale-after 30` — for the reason Phase 10 gave `show`, and more strongly
// here: one `stale` group mixes kinds, and an area and a project in the same
// list are reading two different keys. A line that does not say which knob fired
// leaves §7's chain invisible in exactly the place it decided something.
//
// Where the value came from is carried by --json rather than printed on every
// row: `show <locator>` is the command for one thing's provenance, and repeating
// a path down a column of twenty would bury the numbers.
func thresholdCell(g review.Group, item review.Item) string {
	if g == review.GroupOverdue {
		// A deadline is not a configured number; it is the entity's own stored
		// `due`, printed as typed (§15.1).
		return "due " + item.Entity.State.Due
	}
	if !item.Threshold.Found {
		return ""
	}
	return item.Threshold.Key + " " + exact(item.Threshold.Value)
}

// days spells §26's `61 days`, keeping the singular honest.
func days(n int) string {
	if n == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", n)
}

// reviewOutput is `review --json`: the sections, and §23's counts.
type reviewOutput struct {
	Total    int             `json:"total"`
	Shown    int             `json:"shown"`
	Sections []sectionOutput `json:"groups"`
}

type sectionOutput struct {
	Group string       `json:"group"`
	Total int          `json:"total"`
	Shown int          `json:"shown"`
	Items []itemOutput `json:"items"`
}

// itemOutput is one finding: the whole entity as every other read command
// reports it, plus what put it in this group.
type itemOutput struct {
	entityJSON
	// Days is the group's day count, absent for `behind`, whose measure is a
	// pace and is already on the key-result.
	Days *int `json:"days,omitempty"`
	// Over is the distance past the threshold — the value the group is ordered
	// by. Its units are the group's: days for the three timers and for
	// `blocked`, whose threshold is zero because it has none, and pace for
	// `behind`.
	Over float64 `json:"over"`
	// Threshold is the §7 knob that fired and the level that supplied it,
	// absent for the two groups measured against something else.
	Threshold *thresholdJSON `json:"threshold,omitempty"`
}

type thresholdJSON struct {
	Key   string  `json:"key"`
	Value float64 `json:"value"`
	// From is the config.toml that supplied it, root-relative, or empty when
	// the built-in default did (§7, §22).
	From string `json:"from"`
}

func reviewOutputOf(env *view.Env, res review.Result) reviewOutput {
	out := reviewOutput{
		Total:    res.Total,
		Shown:    res.Shown,
		Sections: make([]sectionOutput, 0, len(res.Sections)),
	}
	for _, s := range res.Sections {
		section := sectionOutput{
			Group: string(s.Group),
			Total: s.Total,
			Shown: len(s.Items),
			Items: make([]itemOutput, 0, len(s.Items)),
		}
		for _, item := range s.Items {
			row := itemOutput{entityJSON: newEntityJSON(env, item.Entity), Over: item.Over}
			if item.HasDays {
				n := item.Days
				row.Days = &n
			}
			if item.Threshold.Found {
				row.Threshold = &thresholdJSON{
					Key:   item.Threshold.Key,
					Value: item.Threshold.Value,
					From:  item.Threshold.From,
				}
			}
			section.Items = append(section.Items, row)
		}
		out.Sections = append(out.Sections, section)
	}
	return out
}
