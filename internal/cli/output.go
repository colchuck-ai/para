package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/colchuck-ai/para/internal/mutate"
)

// wroteLabel and its continuation indent are §23's shape: "mutations print what
// they wrote, one line per file, because write-through touches four or five
// files and a user who cannot see that will not believe it".
const (
	wroteLabel  = "wrote  "
	wroteIndent = "       "
)

// printWrote renders the file list §23 requires: a labelled first line and the
// rest aligned under it. A mutation that wrote nothing prints nothing, which is
// how a no-op stays silent about files (§15).
func printWrote(out io.Writer, paths []string) {
	seen := map[string]bool{}
	label := wroteLabel
	for _, path := range paths {
		// A multi-field `set` appends several lines to one journal file, so the
		// same path can appear more than once in the write record. The record
		// is about operations; this list is about files.
		if seen[path] {
			continue
		}
		seen[path] = true
		fmt.Fprintf(out, "%s%s\n", label, path)
		label = wroteIndent
	}
}

// printResult prints a mutation's summary lines, then its file list, separated
// by a blank line — the layout §23's worked example uses.
func printResult(out io.Writer, summary []string, res mutate.Result) {
	for _, line := range summary {
		fmt.Fprintln(out, line)
	}
	if len(res.Wrote) == 0 {
		return
	}
	if len(summary) > 0 {
		fmt.Fprintln(out)
	}
	printWrote(out, res.Wrote)
}

// changeLines is §26's `set` output: one line per field that changed, naming
// the locator once and the transition in full.
//
//	projects.acme-migration  status in-progress → blocked
func changeLines(res mutate.Result) []string {
	var lines []string
	for _, c := range res.Changes {
		lines = append(lines, fmt.Sprintf("%s  %s %s", res.Locator, c.Field, transition(c.From, c.To)))
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
