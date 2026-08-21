package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// dryRunLine closes a rehearsal. §19 gives `--dry-run` to exactly the operations
// that touch more than one entity's worth of bytes, and the point of one is to be
// certain nothing happened — which is worth a line of its own rather than left to
// the absence of a `wrote` block.
const dryRunLine = "dry-run: nothing was written"

// newMoveCmd implements `para move <noun> <from-chain> <to-chain>` (R14):
// the noun spoken once, because §18.3 makes move same-kind only — a second
// noun could only ever repeat the first or be a refusal, so there is no
// spelling left for "change kind" to parse as (R14's decision log).
//
// Neither chain is "."-resolvable: unlike show's or set's noun-taking
// argument, move's chains have no single-token slot "." could occupy
// without contradicting "noun spoken once" — the destination was never
// "."-resolvable to begin with (it names something that must not exist),
// and losing it from the source is what one shared noun costs.
func newMoveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "move <noun> <from-chain> <to-chain>",
		Short: "move an entity, with its subtree",
		Long: "Move an entity to another place in the tree.\n\n" +
			"One rename, plus the work a hand-`mv` cannot do: README frontmatter\n" +
			"throughout the subtree, both parents' journals, and every skill scope\n" +
			"entry naming the old locator or anything beneath it.\n\n" +
			"Same-kind only — the noun is given once, for both chains — and it\n" +
			"will not cross the archive boundary in either direction: --archived\n" +
			"means both ends are archived, and that is `para archive` and\n" +
			"`para unarchive`.",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, _, err := openEnv(cmd)
			if err != nil {
				return err
			}
			noun, fromChain, toChain := args[0], args[1], args[2]
			if err := checkArchivedSide(env.Root, archived.value, noun, fromChain); err != nil {
				return err
			}
			src, err := chainToLocator(noun, fromChain, archived.value, false)
			if err != nil {
				return err
			}
			dst, err := chainToLocator(noun, toChain, archived.value, false)
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

// checkArchivedSide is R9's own worked example: if the side --archived
// claims for the source does not exist but the other side does, the flag
// almost certainly has the wrong value, and saying so by name —
// "project.acme-migration is live" — is friendlier than PlanMove's own
// "does not exist", which would be equally true of a source that is simply
// missing on both sides. It answers nothing when neither side resolves or
// when the claimed side is actually there, leaving those cases to
// PlanMove's own checks.
func checkArchivedSide(root string, archived bool, noun, chain string) error {
	claimed, err := chainToLocator(noun, chain, archived, false)
	if err != nil {
		return nil
	}
	if exists, err := tree.Exists(root, claimed); err != nil || exists {
		return nil
	}
	other, err := chainToLocator(noun, chain, !archived, false)
	if err != nil {
		return nil
	}
	otherExists, err := tree.Exists(root, other)
	if err != nil || !otherExists {
		return nil
	}
	addr, err := address.String(other)
	if err != nil {
		addr = other.String()
	}
	side := "live"
	if !archived {
		side = "archived"
	}
	return paraerr.Newf(paraerr.KindValidation, "--archived means both ends are archived; %s is %s", addr, side)
}

// newArchiveCmd implements `para archive <noun> <chain>` (R3, entity-only
// per R17). "." works: the source is live by definition (that is what
// this command refuses --archived for), so it may be where you stand.
func newArchiveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "archive <noun> <chain>",
		Short: "move an entity into the archive",
		Long: "Archive an entity: `projects/acme` becomes `archive/projects/acme`.\n\n" +
			"The whole subtree goes in one move, and a live ancestor that stays\n" +
			"behind gets a stub — a bare directory recording where the thing came\n" +
			"from. No field changes: status says how something ended, the archive\n" +
			"says where it lives.",
		// One arg when "." stands in for the whole noun-and-chain pair
		// (R16), two otherwise.
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := archived.check("the source is live by definition"); err != nil {
				return err
			}
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, _, err := parseAddressArgs(env.Root, cwd, args, entityArity, false)
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

// newUnarchiveCmd implements `para unarchive <noun> <chain>` (R3,
// entity-only per R17). "." is refused (entityArityNoDot): it resolves
// against cwd, and you cannot stand in the thing being pulled out of the
// archive.
func newUnarchiveCmd() *cobra.Command {
	var dryRun bool
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "unarchive <noun> <chain>",
		Short: "bring an entity back out of the archive",
		Long: "Unarchive an entity, and cascade upward.\n\n" +
			"Every archived ancestor it needs comes back as a live entity; one that\n" +
			"is already live is adopted rather than duplicated. An archived sibling\n" +
			"stays archived, and the ancestor's archive directory is left behind as\n" +
			"a stub to record its ancestry — the mirror of what `archive` leaves.",
		// One arg admits "." through to parseAddressArgs so its own named
		// refusal fires ("." is not accepted here) instead of cobra's
		// generic arity error — entityArityNoDot refuses it regardless of
		// how many args follow, so allowing the count costs nothing.
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := archived.check("the source is archived by definition"); err != nil {
				return err
			}
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			// The chain itself names the live address (R10's "archive."
			// prefix is never typed) — Archived: true is what tells
			// chainToLocator to prepend it, the same way every other
			// archived-side address is built.
			loc, _, err := parseAddressArgs(env.Root, cwd, args, entityArityNoDot, true)
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
//	archived  area.health → archive.area.health   (3 descendants moved with it)
//	          stub archive/areas/health/ became the entity
func relocationLines(r *mutate.Relocation) []string {
	head := fmt.Sprintf("%s  %s → %s", r.Verb, entityLocatorString(r.From), entityLocatorString(r.To))
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
		add("created stub %s (parent %s is live)", stubDir(stub), entityLocatorString(stub[1:]))
	}
	if len(r.StubAdopted) > 0 {
		add("stub %s became the entity", stubDir(r.StubAdopted))
	}
	for _, loc := range r.Reinstated {
		add("reinstated %s", entityLocatorString(loc))
	}
	for _, stub := range r.StubsDemoted {
		add("%s became a stub", stubDir(stub))
	}
	for _, stub := range r.StubsRemoved {
		add("removed stub %s", stubDir(stub))
	}
	for _, rw := range r.ScopeRewrites {
		add("rewrote %s in %s", count(rw.Entries, "scope entry"), entityLocatorString(rw.Skill))
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

// newRemoveCmd implements `para remove <noun> <chain>` (R3, entity-only per
// R17). Container is not refused here specifically: Env.relocatable (via
// PlanRemove) already refuses it, the same way it does for move and
// archive/unarchive, so this command needs no CLI-level check of its own.
func newRemoveCmd() *cobra.Command {
	var (
		dryRun    bool
		force     bool
		keepFiles bool
		archived  archivedFlag
	)
	cmd := &cobra.Command{
		Use:   "remove <noun> <chain>",
		Short: "delete an entity and everything beneath it",
		Long: "Delete an entity and its whole subtree.\n\n" +
			"It confirms first, naming what will go; --force skips the prompt and\n" +
			"--dry-run rehearses. It is the only interactive prompt para has.\n\n" +
			"--keep-files leaves your content and deletes para's footprint: every\n" +
			".para/, every ACTIVITY.md, every MEASUREMENTS.csv, and the frontmatter\n" +
			"block from every README.md — keeping the body.",
		// One arg when "." stands in for the whole noun-and-chain pair
		// (R16), two otherwise.
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, _, err := parseAddressArgs(env.Root, cwd, args, entityArity, archived.value)
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
				res, err := plan.Apply(true)
				if err != nil {
					return err
				}
				// The exhaustive path list above is a rehearsal's own addition
				// (removalDetail's doc comment); res's own effects are the same
				// ones a real run would print — the parent's journal/ACTIVITY.md
				// write, and a skill's surface change — through the identical
				// apply()/syncSurface path a real run takes (para-ato).
				printEffects(out, res)
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

			res, err := plan.Apply(false)
			if err != nil {
				return err
			}
			printResult(out, []string{fmt.Sprintf("removed  %s", entityLocatorString(plan.Locator))}, res)
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
	head := fmt.Sprintf("remove  %s  %s", entityLocatorString(r.Locator), r.Kind)
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
