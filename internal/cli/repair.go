package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/tree"
)

// newRebuildCmd implements `para rebuild [<locator>] [--dry-run]` (§21.1).
func newRebuildCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag

	cmd := &cobra.Command{
		Use:   "rebuild [<locator>]",
		Short: "regenerate every projection from truth",
		Long: "Regenerate every generated file under the locator — the whole tree by\n" +
			"default — from state.toml, tree.toml, and the journals. It is the answer\n" +
			"to a hand-edited generated file, a hand-mv that left frontmatter stale, a\n" +
			"merge that resolved truth and left the projections wrong, and a new para\n" +
			"version that renders a template differently.\n\n" +
			"It is idempotent, and it never reads a projection to produce one. The one\n" +
			"thing it takes from a generated file is the part of it you own: a\n" +
			"README.md's body, an AGENTS.md's prose outside the markers.\n\n" +
			"ACTIVITY.md is re-derived in full, from every rotated journal file, which\n" +
			"is the one thing a mutation never does.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root, cwd, err := findTree()
			if err != nil {
				return err
			}
			var scope locator.Locator
			if len(args) == 1 {
				if scope, err = resolveLocatorArg(root, cwd, args[0]); err != nil {
					return err
				}
			}

			res, runErr := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{Scope: scope, DryRun: dryRun})
			if runErr != nil {
				// What it managed to write before it stopped is what is on
				// disk, and a user who cannot see it cannot tell the tree's
				// state from the error alone. The summary line is left off:
				// "nothing to rewrite" beneath a failure would be a claim about
				// a tree the command did not finish looking at.
				printChanged(cmd.OutOrStdout(), res, dryRun)
				return runErr
			}
			printRebuild(cmd.OutOrStdout(), res, dryRun)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "list what would change without writing")
	archived.register(cmd)
	return cmd
}

// printRebuild is §26's shape: one line per file, labelled by whether it was
// written or only considered.
func printRebuild(out io.Writer, res rebuild.Result, dryRun bool) {
	printChanged(out, res, dryRun)
	if res.Empty() {
		// Saying nothing would leave "did it run" and "was there nothing to do"
		// looking identical, which for the command you run to repair a tree is
		// the wrong pair to conflate — the same argument `review` makes.
		// "places" rather than a bare count, because the number counts subjects
		// — the root, the containers, the entities, the skills — and not files.
		// A bare number beside a command that has just been listing files reads
		// as a file count, and it is off by a factor of three.
		fmt.Fprintf(out, "nothing to rewrite (%d places up to date)\n", res.Subjects)
	}
}

// printChanged is the file list alone: one line per file, labelled by whether
// it was written or only considered, and by what was done to it.
//
// A rebuild has three kinds of effect now, not one. Turning `emit.claude` off
// makes a rebuild's whole job a deletion (§6.1), and reporting that under
// "rewrote" would be reporting the opposite of what happened.
// Every line carries its own label rather than aligning under the first, which
// is where this differs from a mutation's file list (§23). A mutation's list is
// one verb applied to four or five files; a rebuild's is a long list of three
// verbs, and a reader scanning it for the deletions should not have to count
// back to the last label to find where they start.
func printChanged(out io.Writer, res rebuild.Result, dryRun bool) {
	printEach(out, tense("rewrote", "would rewrite", dryRun), res.Changed)
	printEach(out, tense("removed", "would remove", dryRun), res.Removed)
	for _, c := range res.Mirror {
		fmt.Fprintf(out, "%s  %s\n", tense(string(c.Verb), c.Verb.Would(), dryRun), mirrorLine(c))
	}
}

// tense picks the past tense or the conditional, so that a `--dry-run` never
// claims to have done something it only considered.
func tense(did, would string, dryRun bool) string {
	if dryRun {
		return would
	}
	return did
}

// newDoctorCmd implements `para doctor [<locator>]` (§21.2).
func newDoctorCmd() *cobra.Command {
	var read readFlags
	var archived archivedFlag

	cmd := &cobra.Command{
		Use:   "doctor [<locator>]",
		Short: "scan the tree deeply and report what is wrong",
		Long: "Walk every directory — including inside content, which the fast walk\n" +
			"never enters — and report what a read would get wrong: entities the walk\n" +
			"cannot reach, truth that will not parse, journals with bad lines, scopes\n" +
			"naming things that are gone, and generated files that differ from what\n" +
			"would be written now.\n\n" +
			"Read-only, and there is no --fix. `para rebuild` is the repair for the one\n" +
			"finding that has one; a --fix that repaired that and not the others would\n" +
			"teach you that doctor cleans up after itself, which for orphan, misplaced,\n" +
			"invalid, and scope-unresolved it cannot.\n\n" +
			"Exit 0 clean, 1 on any error, 2 when only advisories are present — so CI\n" +
			"can gate on 1 and ignore 2.",
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

			rep, err := doctor.Run(env, doctor.Options{Scope: scope})
			if err != nil {
				return err
			}
			if read.json {
				if err := writeJSON(cmd.OutOrStdout(), doctorOutputOf(rep)); err != nil {
					return err
				}
			} else {
				printDoctor(cmd.OutOrStdout(), rep)
			}
			if code := rep.ExitCode(); code != 0 {
				// The findings are the output; the code is how to read them
				// (§21.2). paraerr.Status carries the one without printing the
				// other.
				return paraerr.Status(code)
			}
			return nil
		},
	}

	// §23 lists doctor among the commands carrying --json. --local comes with
	// it from the shared flag set and has nothing to convert: a finding names a
	// file and a rule, and the one timestamp it can print is the `at` of a
	// journal line, which is quoted back exactly as the line stores it so that
	// the line can be found.
	read.register(cmd)
	archived.register(cmd)
	cmd.Flags().Lookup("local").Usage = "accepted for consistency; doctor prints no timestamp to convert"
	return cmd
}

// printDoctor is §26's shape: severity, finding, then the file and what is
// wrong with it. `clean` when there is nothing, because an empty report and a
// command that did not run must not look the same.
func printDoctor(out io.Writer, rep doctor.Report) {
	if rep.Clean() {
		fmt.Fprintln(out, "clean")
		return
	}
	t := table{}
	for _, f := range rep.Findings {
		t.add(f.Kind.Severity(), string(f.Kind), findingText(f))
	}
	t.write(out)
}

// findingText is the third column: what is at fault, then what is wrong with
// it. A journal finding carries its line, because §10 requires one to be
// "reported with file and line" and a file of thousands is not an address.
func findingText(f doctor.Finding) string {
	where := f.Path
	if where == "" {
		where = "the tree root"
	}
	if f.Line > 0 {
		where = fmt.Sprintf("%s:%d", where, f.Line)
	}
	return where + " " + f.Detail
}

// doctorOutput is `doctor --json`: the findings as data, and the three counts
// the exit code is computed from — so a caller reading the JSON never has to
// re-derive the verdict from the list.
type doctorOutput struct {
	Clean      bool            `json:"clean"`
	Errors     int             `json:"errors"`
	Advisories int             `json:"advisories"`
	Exit       int             `json:"exit"`
	Findings   []findingOutput `json:"findings"`
}

type findingOutput struct {
	Severity string `json:"severity"`
	Finding  string `json:"finding"`
	Path     string `json:"path"`
	// Line is a journal finding's line, and absent everywhere else.
	Line int `json:"line,omitempty"`
	// Locator names the entity at fault where the path addresses one, in the
	// dotted address form (R24). An orphan is precisely a directory whose
	// locator para cannot derive, so it is absent rather than guessed — and a
	// stub is the same absence for a different reason (R11): it has no kind,
	// so nothing can name it in a grammar whose first token is one.
	Locator string `json:"locator,omitempty"`
	Detail  string `json:"detail"`
}

// findingLocatorString is a Finding's Locator, in the dotted form, or "" for
// the empty Locator every locator-less finding carries — which includes one
// of `misplaced`'s two shapes (a path locator.FromPath itself refuses never
// gets a Locator at all). The other shape `misplaced` has, and `collision`
// always, sets Locator to precisely a position that derives no valid kind (a
// reserved word in an id position, or a position no shape admits), which is
// by construction a position that derives no address either — so the raw
// internal Locator string is shown there, the same way a stub (R11) has no
// noun to print because it has no kind at all.
func findingLocatorString(loc locator.Locator) string {
	s, err := address.String(loc)
	if err != nil {
		return loc.String()
	}
	return s
}

func doctorOutputOf(rep doctor.Report) doctorOutput {
	out := doctorOutput{
		Clean:      rep.Clean(),
		Errors:     rep.Errors(),
		Advisories: rep.Advisories(),
		Exit:       rep.ExitCode(),
		Findings:   make([]findingOutput, 0, len(rep.Findings)),
	}
	for _, f := range rep.Findings {
		out.Findings = append(out.Findings, findingOutput{
			Severity: f.Kind.Severity(),
			Finding:  string(f.Kind),
			Path:     f.Path,
			Line:     f.Line,
			Locator:  findingLocatorString(f.Locator),
			Detail:   f.Detail,
		})
	}
	return out
}

// findTree discovers the tree and the working directory "." resolves against
// (§14), for the two commands that are neither a read of entities nor a
// mutation of one: rebuild writes projections and reads no clock, so a
// view.Env would hand it two things it must not use.
func findTree() (root, cwd string, err error) {
	cwd, err = os.Getwd()
	if err != nil {
		return "", "", paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
	}
	root, err = tree.Find(cwd)
	if err != nil {
		return "", "", err
	}
	return root, cwd, nil
}
