package cli

import (
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/view"
)

// newLogCmd implements `para log <locator>` (§16.3).
func newLogCmd() *cobra.Command {
	var read readFlags
	var kind string
	var limit int
	var reverse bool

	cmd := &cobra.Command{
		Use:   "log <locator>",
		Short: "print one entity's raw journal, newest first",
		Long: "Print the raw journal for one entity — the JSONL, rendered as lines, newest\n" +
			"first, matching ACTIVITY.md's direction so the two never read in opposite\n" +
			"orders. It reads every rotated file for that entity, in order. It is the one\n" +
			"command that does.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openRead(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			ent, err := env.Load(loc)
			if err != nil {
				return err
			}
			events, err := journal.ReadAll(truth.LogsDir(ent.Dir))
			if err != nil {
				return err
			}
			events, err = filterKind(events, kind)
			if err != nil {
				return err
			}

			// Newest first (§16.3), unless --reverse asks for chronological.
			// ReadAll returns them by `at`, oldest first (§3.1).
			if !reverse {
				slices.Reverse(events)
			}
			if limit > 0 && len(events) > limit {
				events = events[:limit]
			}

			if read.json {
				// §16.3: "--json to get the events back unchanged". The events
				// are the journal's own shape, so they are marshalled as they
				// were decoded rather than reshaped into a payload of their own.
				return writeJSON(cmd.OutOrStdout(), events)
			}
			printLog(cmd.OutOrStdout(), env, events, read)
			return nil
		},
	}
	read.register(cmd)
	cmd.Flags().StringVar(&kind, "kind", "", "only events of this kind: change, measurement, note, child")
	cmd.Flags().IntVar(&limit, "limit", 0, "print at most this many")
	cmd.Flags().BoolVar(&reverse, "reverse", false, "chronological rather than newest first")
	return cmd
}

// eventKinds is §3.1's four, in the order the error message lists them.
var eventKinds = []journal.Kind{
	journal.KindChange, journal.KindMeasurement, journal.KindNote, journal.KindChild,
}

func filterKind(events []journal.Event, kind string) ([]journal.Event, error) {
	if kind == "" {
		return events, nil
	}
	if !slices.Contains(eventKinds, journal.Kind(kind)) {
		names := make([]string, len(eventKinds))
		for i, k := range eventKinds {
			names[i] = string(k)
		}
		return nil, paraerr.Newf(paraerr.KindValidation,
			"%q is not an event kind (one of %s)", kind, strings.Join(names, ", "))
	}
	var out []journal.Event
	for _, e := range events {
		if e.Kind == journal.Kind(kind) {
			out = append(out, e)
		}
	}
	return out, nil
}

// printLog renders the journal as lines: the instant, the kind, and what the
// event says.
//
// The instant is printed in full rather than as a day, which is the difference
// between `log` and `activity`: a digest folds a day's events into a summary,
// and the journal is the record of moments the fold was made from.
func printLog(out io.Writer, env *view.Env, events []journal.Event, read readFlags) {
	zone := read.zone(env)
	var t table
	for _, e := range events {
		t.add(instant(e.At, zone), string(e.Kind), logDetail(e), logNote(e))
	}
	t.write(out)
}

// logDetail is the event's own content, per kind (§3.1's table).
func logDetail(e journal.Event) string {
	switch e.Kind {
	case journal.KindChange:
		return e.Field + " " + transition(e.From, e.To)
	case journal.KindMeasurement:
		return e.Value
	case journal.KindNote:
		// A note's whole content is its note field, which the next column
		// carries; repeating it here would print it twice.
		return ""
	case journal.KindChild:
		return childDetail(e)
	default:
		return ""
	}
}

func childDetail(e journal.Event) string {
	detail := string(e.Op) + " " + e.Child
	if e.Op == journal.ChildOpMoved && e.From != "" && e.To != "" {
		detail += " " + transition(e.From, e.To)
	}
	return detail
}

// logNote is the note any event kind may carry (§3.1), flattened to one line so
// a multi-line note does not break the column layout.
func logNote(e journal.Event) string {
	return strings.Join(strings.Fields(e.Note), " ")
}
