package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/view"
)

// newActivityCmd implements `para activity [<locator>]` (§16.4).
func newActivityCmd() *cobra.Command {
	var read readFlags
	var recursive bool
	var since string

	cmd := &cobra.Command{
		Use:   "activity <locator>",
		Short: "print the digest — the same fold ACTIVITY.md contains",
		Long: "Print the digest for an entity or container. Without --recursive it is the\n" +
			"same fold ACTIVITY.md contains, which is the point: the two agree by\n" +
			"construction. --recursive merges the digests of everything beneath into one\n" +
			"chronology, each line labelled with the locator it came from. That is the only\n" +
			"rollup in the system, it is computed on demand, and nothing about it is\n" +
			"written to disk.",
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
			from, err := parseSince(since)
			if err != nil {
				return err
			}

			subjects, err := activitySubjects(env, loc, recursive)
			if err != nil {
				return err
			}
			rolled, err := rollup(env, subjects, from)
			if err != nil {
				return err
			}

			if read.json {
				return writeJSON(cmd.OutOrStdout(), rolled)
			}
			if !recursive && from == "" {
				// §16.4: "without --recursive it is just `cat` on a generated
				// file, and that is fine: it means the two agree by
				// construction". Re-derived rather than read, because the
				// journal is truth and ACTIVITY.md is a projection (§2.1) — a
				// read command sourcing the projection would report the drift
				// doctor exists to find.
				return printActivityFile(cmd.OutOrStdout(), env, subjects[0])
			}
			printRollup(cmd.OutOrStdout(), rolled, recursive)
			return nil
		},
	}
	read.register(cmd)
	cmd.Flags().BoolVar(&recursive, "recursive", false, "merge the digests of everything beneath into one chronology")
	cmd.Flags().StringVar(&since, "since", "", "only days on or after this date")
	return cmd
}

// parseSince validates --since and reduces it to the UTC day it names, which is
// what the digest's sections are keyed by (§3.5).
func parseSince(since string) (string, error) {
	if since == "" {
		return "", nil
	}
	t, err := ptime.ParseAt(since, time.UTC)
	if err != nil {
		return "", err
	}
	return t.UTC().Format("2006-01-02"), nil
}

// activitySubjects is the entity named, or it and everything beneath it.
//
// Containers are included in the recursive case and excluded from being rows in
// `list`, and that is not an inconsistency: a container has a journal of its own
// (§8.2) and §3.3's `child` events land in it, so "added objective q1-growth" —
// §26's own first line — is a container's event and would be missing otherwise.
func activitySubjects(env *view.Env, loc locator.Locator, recursive bool) ([]view.Entity, error) {
	self, err := env.Load(loc)
	if err != nil {
		return nil, err
	}
	if !recursive {
		return []view.Entity{self}, nil
	}
	res, err := query.List(env, query.Options{
		Scope:  loc,
		Filter: query.Filter{All: true},
	})
	if err != nil {
		return nil, err
	}
	subjects := append([]view.Entity{self}, res.Entities...)

	// query.List drops containers, which are exactly the subjects whose
	// journals hold the containment events, so they are collected separately.
	containers, err := activityContainers(env, loc)
	if err != nil {
		return nil, err
	}
	return append(subjects, containers...), nil
}

func activityContainers(env *view.Env, loc locator.Locator) ([]view.Entity, error) {
	nodes, err := treeSubtree(env, loc)
	if err != nil {
		return nil, err
	}
	var out []view.Entity
	for _, n := range nodes {
		if !n.IsContainer || len(n.Locator) == len(loc) {
			continue
		}
		state, err := truth.ReadState(n.Path)
		if err != nil {
			return nil, err
		}
		ent, err := env.Derive(n.Locator, kindmeta.KindContainer, n.Path, state)
		if err != nil {
			return nil, err
		}
		out = append(out, ent)
	}
	return out, nil
}

// digestLine is one line of the rollup: when, where, and what.
type digestLine struct {
	Day     string       `json:"day"`
	At      string       `json:"at,omitempty"`
	Locator string       `json:"locator"`
	Kind    journal.Kind `json:"kind,omitempty"`
	Text    string       `json:"text"`
}

// rollup folds every subject's journal into one chronology, newest first.
//
// This is the only aggregation in the system, it is computed on demand, and
// nothing about it is written to disk (§16.4, §3.2). The lines come from
// render.Digest, so they are the same lines ACTIVITY.md carries — which is what
// makes §16.4's "they agree by construction" a fact rather than a claim.
func rollup(env *view.Env, subjects []view.Entity, since string) ([]digestLine, error) {
	var out []digestLine
	for _, subject := range subjects {
		events, err := journal.ReadAll(truth.LogsDir(subject.Dir))
		if err != nil {
			return nil, err
		}
		days, err := render.Digest(render.In{
			Locator: subject.Locator,
			Kind:    subject.Kind,
			State:   subject.State,
			Events:  events,
		})
		if err != nil {
			return nil, err
		}
		for _, d := range days {
			if since != "" && d.Date < since {
				continue
			}
			for _, line := range d.Lines {
				out = append(out, digestLine{
					Day:     d.Date,
					At:      utcOrEmpty(line.At),
					Locator: subject.Locator.String(),
					Kind:    line.Kind,
					Text:    line.Text,
				})
			}
		}
	}

	// Newest first, and by `at` rather than by which subject was read (§3.1).
	// The `created` line has no instant, so it sorts to the end of its own day,
	// which is where it belongs: it is the first thing that happened.
	sortLines(out)
	return out, nil
}

// printActivityFile prints the digest exactly as ACTIVITY.md holds it, which is
// §16.4's whole claim about the unfiltered case.
func printActivityFile(out io.Writer, env *view.Env, subject view.Entity) error {
	events, err := journal.ReadAll(truth.LogsDir(subject.Dir))
	if err != nil {
		return err
	}
	// Full mode: Days empty re-derives every section from the whole history,
	// which is what rebuild writes and doctor compares against (§10, §21.1).
	data, err := render.Activity.Render(render.In{
		Locator: subject.Locator,
		Kind:    subject.Kind,
		State:   subject.State,
		Events:  events,
	})
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// printRollup is §26's three-column shape: the day, the locator the line came
// from, and the line. The locator column is dropped when there is only one
// subject, since repeating it on every row of a single entity's digest says
// nothing.
func printRollup(out io.Writer, lines []digestLine, recursive bool) {
	var t table
	for _, line := range lines {
		if recursive {
			t.add(line.Day, line.Locator, line.Text)
			continue
		}
		t.add(line.Day, line.Text)
	}
	t.write(out)
	if len(lines) == 0 {
		fmt.Fprintln(out, "no activity")
	}
}

// sortLines orders the rollup newest first: by day, then by instant within a
// day. Ties keep the order the subjects were read in, which is the §8.5 walk's
// own, so two events at one instant come out the same way twice.
func sortLines(lines []digestLine) {
	sort.SliceStable(lines, func(i, j int) bool {
		if lines[i].Day != lines[j].Day {
			return lines[i].Day > lines[j].Day
		}
		return lines[i].At > lines[j].At
	})
}

// treeSubtree is tree.Subtree, named here so activity.go states what it is
// asking for: the containers beneath a locator, whose journals hold the
// containment events §3.3 puts there.
func treeSubtree(env *view.Env, loc locator.Locator) ([]tree.Node, error) {
	return tree.Subtree(env.Root, loc)
}
