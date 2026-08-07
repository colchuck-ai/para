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
		Use:   "activity [<locator>]",
		Short: "print the digest — the same fold ACTIVITY.md contains",
		Long: "Print the digest for an entity, a container, or — naming nothing — the tree\n" +
			"root, which has an ACTIVITY.md like every other tracked directory and no\n" +
			"locator to address it by.\n\n" +
			"Without --recursive it is the same fold ACTIVITY.md contains, which is the\n" +
			"point: the two agree by construction. --recursive merges the digests of\n" +
			"everything beneath into one chronology, each line labelled with the locator it\n" +
			"came from. That is the only rollup in the system, it is computed on demand, and\n" +
			"nothing about it is written to disk.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openRead(cmd)
			if err != nil {
				return err
			}
			// No locator is the root, the same way it is for `list`, `rebuild`,
			// and `doctor`: absent means the whole tree, never the working
			// directory. `.` is the spelling for that (§14).
			var loc locator.Locator
			if len(args) == 1 {
				if loc, err = resolveLocatorArg(env.Root, cwd, args[0]); err != nil {
					return err
				}
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
			if !recursive {
				// §16.4: "without --recursive it is just `cat` on a generated
				// file, and that is fine: it means the two agree by
				// construction". Re-derived rather than read, because the
				// journal is truth and ACTIVITY.md is a projection (§2.1) — a
				// read command sourcing the projection would report the drift
				// doctor exists to find.
				//
				// `--since` narrows which days are shown and does not change
				// the shape: there are two output shapes, the file's and the
				// rollup's, and `--recursive` alone chooses between them.
				return printActivityFile(cmd.OutOrStdout(), subjects[0], from)
			}
			printRollup(cmd.OutOrStdout(), rolled)
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

// digestSubject is one thing whose digest is being read: an entity, a
// container, or the tree root.
//
// It exists rather than a view.Entity because the root is not one. The root has
// no locator (§1.4), no state.toml, and its identity lives in tree.toml
// instead (§8.1) — which is the same shape difference render.In already carries
// as the empty locator, so this type is exactly the fields the digest needs and
// nothing else.
type digestSubject struct {
	Locator locator.Locator
	Kind    kindmeta.Kind
	State   truth.State
	Tree    truth.Tree
	Dir     string
}

// in is the digest's view of the subject. The empty locator selects render's
// root shape, which reads Tree where every other shape reads State.
func (s digestSubject) in(events []journal.Event) render.In {
	return render.In{
		Locator: s.Locator,
		Kind:    s.Kind,
		State:   s.State,
		Tree:    s.Tree,
		Events:  events,
	}
}

// activitySubjects is the thing named, or it and everything beneath it.
//
// Containers are included in the recursive case and excluded from being rows in
// `list`, and that is not an inconsistency: a container has a journal of its own
// (§8.2) and §3.3's `child` events land in it, so "added objective q1-growth" —
// §26's own first line — is a container's event and would be missing otherwise.
func activitySubjects(env *view.Env, loc locator.Locator, recursive bool) ([]digestSubject, error) {
	self, err := loadSubject(env, loc)
	if err != nil {
		return nil, err
	}
	if !recursive {
		return []digestSubject{self}, nil
	}
	res, err := query.List(env, query.Options{
		Scope:  loc,
		Filter: query.Filter{All: true},
	})
	if err != nil {
		return nil, err
	}
	subjects := []digestSubject{self}
	for _, e := range res.Entities {
		subjects = append(subjects, entitySubject(e))
	}

	// query.List drops containers, which are exactly the subjects whose
	// journals hold the containment events, so they are collected separately.
	containers, err := activityContainers(env, loc)
	if err != nil {
		return nil, err
	}
	return append(subjects, containers...), nil
}

// loadSubject reads the named thing, or the root when nothing is named.
func loadSubject(env *view.Env, loc locator.Locator) (digestSubject, error) {
	if len(loc) == 0 {
		tr, err := truth.ReadTree(env.Root)
		if err != nil {
			return digestSubject{}, err
		}
		return digestSubject{Tree: tr, Dir: env.Root}, nil
	}
	ent, err := env.Load(loc)
	if err != nil {
		return digestSubject{}, err
	}
	return entitySubject(ent), nil
}

func entitySubject(e view.Entity) digestSubject {
	return digestSubject{Locator: e.Locator, Kind: e.Kind, State: e.State, Dir: e.Dir}
}

// activityContainers collects the containers beneath loc, whose journals hold
// the containment events no entity's journal carries.
//
// The archive rule is `list`'s, deliberately: a rollup that traversed archive/
// while the entity rows beside it did not would report a container's events
// from a place the same command refuses to list entities from (§16.2).
func activityContainers(env *view.Env, loc locator.Locator) ([]digestSubject, error) {
	nodes, err := containerNodes(env.Root, loc)
	if err != nil {
		return nil, err
	}
	var out []digestSubject
	for _, n := range nodes {
		if !n.IsContainer || len(n.Locator) == len(loc) {
			continue
		}
		if n.Archived && !loc.IsArchived() {
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
		out = append(out, entitySubject(ent))
	}
	return out, nil
}

// containerNodes is the walk beneath loc — the whole tree when loc is the root,
// which tree.Subtree refuses by definition since the root is not a subtree of
// itself.
func containerNodes(root string, loc locator.Locator) ([]tree.Node, error) {
	if len(loc) > 0 {
		return tree.Subtree(root, loc)
	}
	var nodes []tree.Node
	err := tree.Walk(root, func(n tree.Node) error {
		nodes = append(nodes, n)
		return nil
	})
	return nodes, err
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
func rollup(env *view.Env, subjects []digestSubject, since string) ([]digestLine, error) {
	var out []digestLine
	for _, subject := range subjects {
		events, err := journal.ReadAll(truth.LogsDir(subject.Dir))
		if err != nil {
			return nil, err
		}
		days, err := render.Digest(subject.in(events))
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
func printActivityFile(out io.Writer, subject digestSubject, since string) error {
	events, err := journal.ReadAll(truth.LogsDir(subject.Dir))
	if err != nil {
		return err
	}
	// Full mode: Days empty re-derives every section from the whole history,
	// which is what rebuild writes and doctor compares against (§10, §21.1).
	data, err := render.ActivityRenderer{Since: since}.Render(subject.in(events))
	if err != nil {
		return err
	}
	_, err = out.Write(data)
	return err
}

// printRollup is §26's three-column shape: the day, the locator the line came
// from, and the line. It is `--recursive`'s only, since the locator column is
// the whole reason the rollup is not just the file.
func printRollup(out io.Writer, lines []digestLine) {
	var t table
	for _, line := range lines {
		// The root's lines have no locator to label them with (§1.4), so they
		// print the dash every absent value prints as. `.` would be a spelling
		// that only pastes back from one directory, which is the opposite of
		// what §14 promises about what para prints.
		where := line.Locator
		if where == "" {
			where = dash
		}
		t.add(line.Day, where, line.Text)
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
