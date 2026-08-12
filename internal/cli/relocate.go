package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// dryRunLine closes a rehearsal. §19 gives `--dry-run` to exactly the operations
// that touch more than one entity's worth of bytes, and the point of one is to be
// certain nothing happened — which is worth a line of its own rather than left to
// the absence of a `wrote` block.
const dryRunLine = "dry-run: nothing was written"

func newMoveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "move <from> <to>",
		Short: "move an entity, with its subtree",
		Long: "Move an entity to another place in the tree.\n\n" +
			"One rename, plus the work a hand-`mv` cannot do: README frontmatter\n" +
			"throughout the subtree, both parents' journals, and every skill scope\n" +
			"entry naming the old locator or anything beneath it.\n\n" +
			"Same-kind only, and it will not cross the archive boundary in either\n" +
			"direction — that is `para archive` and `para unarchive`.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			src, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			// The destination is deliberately not "."-resolvable: "." names
			// something that exists (§14), and a destination must not.
			dst, err := locator.Parse(args[1])
			if err != nil {
				return err
			}
			plan, err := env.PlanMove(src, dst)
			if err != nil {
				return err
			}
			return runRelocation(cmd, plan, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "rehearse: report what would happen and write nothing")
	archived.register(cmd)
	return cmd
}

func newArchiveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "archive <locator>",
		Short: "move an entity into the archive",
		Long: "Archive an entity: `projects/acme` becomes `archive/projects/acme`.\n\n" +
			"The whole subtree goes in one move, and a live ancestor that stays\n" +
			"behind gets a stub — a bare directory recording where the thing came\n" +
			"from. No field changes: status says how something ended, the archive\n" +
			"says where it lives.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := archived.check("the source is live by definition"); err != nil {
				return err
			}
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			plan, err := env.PlanArchive(loc)
			if err != nil {
				return err
			}
			return runRelocation(cmd, plan, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "rehearse: report what would happen and write nothing")
	archived.registerRefused(cmd, "the source is live by definition")
	return cmd
}

func newUnarchiveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "unarchive <locator>",
		Short: "bring an entity back out of the archive",
		Long: "Unarchive an entity, and cascade upward.\n\n" +
			"Every archived ancestor it needs comes back as a live entity; one that\n" +
			"is already live is adopted rather than duplicated. An archived sibling\n" +
			"stays archived, and the ancestor's archive directory is left behind as\n" +
			"a stub to record its ancestry — the mirror of what `archive` leaves.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := archived.check("the source is archived by definition"); err != nil {
				return err
			}
			env, _, err := openEnv(cmd)
			if err != nil {
				return err
			}
			// Not "."-resolvable either: "." resolves to the directory you are
			// standing in, and you cannot stand in the thing you are pulling out.
			loc, err := locator.Parse(args[0])
			if err != nil {
				return err
			}
			plan, err := env.PlanUnarchive(loc)
			if err != nil {
				return err
			}
			return runRelocation(cmd, plan, dryRun)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "rehearse: report what would happen and write nothing")
	archived.registerRefused(cmd, "the source is archived by definition")
	return cmd
}

// runRelocation reports the plan, then either applies it or stops. A rehearsal
// and a real run print the same summary because they are reporting the same
// planned value (§19).
func runRelocation(cmd *cobra.Command, plan *mutate.Relocation, dryRun bool) error {
	out := cmd.OutOrStdout()
	lines := relocationLines(plan)
	if dryRun {
		for _, line := range lines {
			fmt.Fprintln(out, line)
		}
		fmt.Fprintln(out, dryRunLine)
		return nil
	}
	res, err := plan.Apply()
	if err != nil {
		return err
	}
	printResult(out, lines, res)
	return nil
}

// relocationLines is §26's shape for the three relocating verbs: the transition
// on the first line, and every consequence indented under it.
//
//	archived  areas.health → archive.areas.health   (3 descendants moved with it)
//	          stub archive/areas/health/ became the entity
func relocationLines(r *mutate.Relocation) []string {
	head := fmt.Sprintf("%s  %s → %s", r.Verb, r.From, r.To)
	if r.Descendants > 0 {
		head += fmt.Sprintf("   (%s moved with it)", count(r.Descendants, "descendant"))
	}
	lines := []string{head}

	indent := strings.Repeat(" ", len(r.Verb)+2)
	add := func(format string, args ...any) {
		lines = append(lines, indent+fmt.Sprintf(format, args...))
	}

	for _, stub := range r.StubsCreated {
		// The parenthetical names the live counterpart, which is the reason the
		// stub exists at all: the parent did not come along (§1.6).
		add("created stub %s (parent %s is live)", stubDir(stub), stub[1:])
	}
	if len(r.StubAdopted) > 0 {
		add("stub %s became the entity", stubDir(r.StubAdopted))
	}
	for _, loc := range r.Reinstated {
		add("reinstated %s", loc)
	}
	for _, stub := range r.StubsDemoted {
		add("%s became a stub", stubDir(stub))
	}
	for _, stub := range r.StubsRemoved {
		add("removed stub %s", stubDir(stub))
	}
	for _, rw := range r.ScopeRewrites {
		add("rewrote %s in %s", count(rw.Entries, "scope entry"), rw.Skill)
	}
	return lines
}

// stubDir is a stub's directory as §26 prints it: root-relative, with a trailing
// slash, because a stub is a directory and nothing else.
func stubDir(loc locator.Locator) string {
	path, err := loc.Path()
	if err != nil {
		return loc.String()
	}
	return path + "/"
}

// count pluralises an English noun, including the one noun here that does not
// take a bare "s".
func count(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	if strings.HasSuffix(noun, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(noun, "y"))
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func newRemoveCmd() *cobra.Command {
	var (
		dryRun    bool
		force     bool
		keepFiles bool
		archived  archivedFlag
	)
	cmd := &cobra.Command{
		Use:   "remove <locator>",
		Short: "delete an entity and everything beneath it",
		Long: "Delete an entity and its whole subtree.\n\n" +
			"It confirms first, naming what will go; --force skips the prompt and\n" +
			"--dry-run rehearses. It is the only interactive prompt para has.\n\n" +
			"--keep-files leaves your content and deletes para's footprint: every\n" +
			".para/, every ACTIVITY.md, every MEASUREMENTS.csv, and the frontmatter\n" +
			"block from every README.md — keeping the body.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			plan, err := env.PlanRemove(loc, keepFiles)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			for _, line := range removalLines(plan) {
				fmt.Fprintln(out, line)
			}
			if dryRun {
				for _, line := range removalDetail(plan) {
					fmt.Fprintln(out, line)
				}
				fmt.Fprintln(out, dryRunLine)
				return nil
			}
			if !force {
				ok, err := confirm(out, cmd.InOrStdin())
				if err != nil {
					return err
				}
				if !ok {
					return paraerr.New(paraerr.KindUsage, "aborted — nothing was removed")
				}
			}

			res, err := plan.Apply()
			if err != nil {
				return err
			}
			printResult(out, []string{fmt.Sprintf("removed  %s", plan.Locator)}, res)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "rehearse: report what would happen and write nothing")
	cmd.Flags().BoolVar(&force, "force", false, "skip the confirmation")
	cmd.Flags().BoolVar(&keepFiles, "keep-files", false, "keep your content; delete para's footprint throughout the subtree")
	archived.register(cmd)
	return cmd
}

// removalLines names the blast radius §19 requires the confirmation to name:
// what is going, how much of it there is, and whether your content survives.
func removalLines(r *mutate.Removal) []string {
	head := fmt.Sprintf("remove  %s  %s", r.Locator, r.Kind)
	if n := len(r.Descendants); n > 0 {
		head += fmt.Sprintf("   (%s)", count(n, "descendant"))
	}
	lines := []string{head}
	add := func(format string, args ...any) {
		lines = append(lines, "        "+fmt.Sprintf(format, args...))
	}

	if !r.KeepFiles {
		add("deletes %s and everything beneath it", dirOf(r))
	} else {
		add("deletes %s, keeping your content", count(len(r.Deletes), "para path"))
	}
	if n := len(r.Strips); n > 0 {
		add("strips the frontmatter from %s, keeping every body", count(n, "file"))
	}
	return lines
}

// removalDetail is the exhaustive list, which only a rehearsal prints: the
// confirmation is a summary a person reads before answering, and `--dry-run` is
// the place to be certain about every path.
func removalDetail(r *mutate.Removal) []string {
	var lines []string
	label := "        would delete  "
	for _, path := range r.Deletes {
		lines = append(lines, label+path)
		label = "                      "
	}
	label = "        would strip   "
	for _, path := range r.Strips {
		lines = append(lines, label+path)
		label = "                      "
	}
	return lines
}

// dirOf is the subject's own directory, root-relative with a trailing slash.
func dirOf(r *mutate.Removal) string {
	path, err := r.Locator.Path()
	if err != nil {
		return r.Locator.String()
	}
	return path + "/"
}

// confirm is para's one interactive prompt (§19). Anything but an explicit yes
// declines, including end of input: a `remove` piped from nothing must not read
// silence as consent.
func confirm(out io.Writer, in io.Reader) (bool, error) {
	fmt.Fprint(out, "continue? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}
