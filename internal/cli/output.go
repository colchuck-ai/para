package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/mutate"
)

// printLabelled renders §23's file-list shape: "mutations print what they
// wrote, one line per file, because write-through touches four or five files
// and a user who cannot see that will not believe it" — a labelled first line
// with the rest aligned under it. An empty list prints nothing, which is how a
// no-op stays silent about files (§15).
func printLabelled(out io.Writer, label string, items []string) {
	pad := strings.Repeat(" ", len(label))
	for i, item := range items {
		if i > 0 {
			label = pad
		}
		fmt.Fprintf(out, "%s  %s\n", label, item)
	}
}

// printEach is the other shape: every line carries the label. It is what a
// list mixing several verbs needs, which is `rebuild`'s case and not §23's.
func printEach(out io.Writer, label string, items []string) {
	for _, item := range items {
		fmt.Fprintf(out, "%s  %s\n", label, item)
	}
}

// printWrote is that list for the files a mutation wrote.
func printWrote(out io.Writer, paths []string) {
	printLabelled(out, "wrote", uniquePaths(paths))
}

// uniquePaths is a write record read as a list of files rather than of
// operations, keeping the order the first mention gave.
//
// A multi-field `set` appends several lines to one journal file, and `init`
// appends four to the root's, so the same path can appear more than once in the
// record. The record is about operations; every list para prints is about files.
func uniquePaths(paths []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	return out
}

// printEffects prints everything a mutation did to files: what it wrote, what
// it removed, and what it did to the `.claude/skills/` mirror.
//
// Three lists rather than one, because they need three different verbs.
// "wrote" beside a deleted CLAUDE.md would be a lie, and §26 gives the mirror
// its own word — `linked …/para-signups-report → ../../.agents/skills/…` — for
// the good reason that a link is the one thing para writes whose content is a
// path rather than bytes.
//
// Both verbs are reachable for CLAUDE.md and .gitattributes now that
// `emit.claude` and `emit.gitattributes` off shorten a block-scoped file
// rather than always deleting it (§6.1, §9): a block taken out of an otherwise
// non-empty file is a write, and one that empties the file is a removal.
func printEffects(out io.Writer, res mutate.Result) {
	printWrote(out, res.Wrote)
	printLabelled(out, "removed", res.Removed)
	printMirror(out, res.Mirror)
}

// printMirror prints one line per mirror change, runs of the same verb grouped
// under it the way §26's transcript groups two links.
func printMirror(out io.Writer, changes []mirror.Change) {
	for i := 0; i < len(changes); {
		verb := changes[i].Verb
		var items []string
		for ; i < len(changes) && changes[i].Verb == verb; i++ {
			items = append(items, mirrorLine(changes[i]))
		}
		printLabelled(out, string(verb), items)
	}
}

// mirrorLine is one mirror change: the entry, and for a link the target it now
// points at, because a link nobody can see the far end of is a link nobody can
// check.
func mirrorLine(c mirror.Change) string {
	if c.Target == "" {
		return c.Path
	}
	return c.Path + " → " + c.Target
}

// printResult prints a mutation's summary lines, then what it did to files,
// separated by a blank line — the layout §23's worked example uses.
func printResult(out io.Writer, summary []string, res mutate.Result) {
	for _, line := range summary {
		fmt.Fprintln(out, line)
	}
	if len(res.Wrote) == 0 && len(res.Removed) == 0 && len(res.Mirror) == 0 {
		return
	}
	if len(summary) > 0 {
		fmt.Fprintln(out)
	}
	printEffects(out, res)
}

// changeLines is §26's `set` output: one line per field that changed, naming
// the locator once and the transition in full.
//
//	project.acme-migration  status in-progress → blocked
func changeLines(res mutate.Result) []string {
	var lines []string
	loc := entityLocatorString(res.Locator)
	for _, c := range res.Changes {
		lines = append(lines, fmt.Sprintf("%s  %s %s", loc, c.Field, transition(c.From, c.To)))
	}
	return lines
}

// transition spells a field change. An absent side is spelled rather than left
// blank, since "due  → 2026-09-30" reads as a formatting bug.
func transition(from, to string) string {
	switch {
	case from == "":
		return "set to " + to
	case to == "":
		return "unset (was " + from + ")"
	default:
		return from + " → " + to
	}
}

// noChangeLine is §26's "no change (status already blocked); note recorded" —
// the answer a user needs when a command they expected to do something did
// nothing (§15, §23).
func noChangeLine(res mutate.Result) string {
	var already []string
	for _, n := range res.NoOps {
		if n.Value == "" {
			already = append(already, fmt.Sprintf("%s already unset", n.Field))
			continue
		}
		already = append(already, fmt.Sprintf("%s already %s", n.Field, n.Value))
	}

	line := "no change"
	if len(already) > 0 {
		line += " (" + strings.Join(already, ", ") + ")"
	}
	if res.NoteRecorded {
		line += "; note recorded"
	}
	return line
}
