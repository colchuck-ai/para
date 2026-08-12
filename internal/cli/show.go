package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/view"
)

// newShowCmd implements `para show <noun> [<chain>]` (R3, R17: a bare noun
// is the bucket).
func newShowCmd() *cobra.Command {
	var read readFlags
	var archived archivedFlag

	cmd := &cobra.Command{
		Use:   "show <noun> [<chain>]",
		Short: "print one thing: its stored fields, what is derived, its children, and the skills that reach it",
		Long: "Print the thing itself. It does not print the journal (`log`), the digest\n" +
			"(`activity`), or its siblings (`list`), and it is unaffected by the\n" +
			"terminal-status hiding `list` applies — you named the thing.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openRead(cmd)
			if err != nil {
				return err
			}
			loc, _, err := parseAddressArgs(env.Root, cwd, args, bucketArity, archived.value)
			if err != nil {
				return err
			}
			ent, err := env.Load(loc)
			if err != nil {
				return err
			}
			children, err := showChildren(env, ent)
			if err != nil {
				return err
			}
			skills, err := env.SkillsReaching(loc)
			if err != nil {
				return err
			}
			stale, isStale, err := env.Stale(ent)
			if err != nil {
				return err
			}

			s := shown{ent: ent, children: children, skills: skills, stale: stale, isStale: isStale}
			if read.json {
				return writeJSON(cmd.OutOrStdout(), showOutputOf(env, s))
			}
			printShow(cmd.OutOrStdout(), env, s, read)
			return nil
		},
	}
	read.register(cmd)
	archived.register(cmd)
	return cmd
}

// shown is everything `show` gathered, so that the human and JSON shapes are
// two renderings of one read rather than two reads.
type shown struct {
	ent      view.Entity
	children []view.Entity
	skills   []view.SkillReach
	stale    view.Threshold
	isStale  bool
}

// showChildren is the children summary, bounded by kind.
//
// The whole subtree for the project → objective → key-result chain, because
// §1.3 closes it at three levels and a project's objectives without their
// numbers say nothing — which is exactly what §16.1's example shows. Immediate
// children plus nothing else for areas and resources, which nest without limit;
// §16.1 already draws that line for siblings ("it does not print … its
// siblings (`list`)"), and `list <noun> <chain>` is how you see the rest.
func showChildren(env *view.Env, ent view.Entity) ([]view.Entity, error) {
	if ent.Kind == kindmeta.KindKeyResult || ent.Kind == kindmeta.KindSkill {
		return nil, nil
	}
	opts := query.Options{
		Scope: ent.Locator,
		// Terminal hiding is `list`'s, not `show`'s: a done objective is part
		// of what this project contains, and omitting it here would make the
		// summary disagree with the tree.
		Filter: query.Filter{All: true, Direct: !deepSummary(ent.Kind)},
	}
	res, err := query.List(env, opts)
	if err != nil {
		return nil, err
	}
	return res.Entities, nil
}

// deepSummary reports whether a kind's children summary reaches past the
// immediate generation — the bounded chain §1.3 closes at three levels.
func deepSummary(k kindmeta.Kind) bool {
	return k == kindmeta.KindProject || k == kindmeta.KindObjective || k == kindmeta.KindContainer
}

// showLabelWidth is the field-block label column. It is fixed rather than
// computed because the block is a list of known labels, and letting it shrink
// would make two `show` outputs of different things line up differently.
const showLabelWidth = 13

func printShow(out io.Writer, env *view.Env, s shown, read readFlags) {
	ent := s.ent
	zone := read.zone(env)

	// The header: what it is, then what it is called, then what it is for.
	// The noun and the chain print as their own columns (R23), the same way
	// `list`'s rows do, so the two never disagree about how an address looks.
	var head table
	head.add(ent.Kind.String(), entityChain(ent.Locator))
	head.write(out)
	if name := ent.Name(); name != "" {
		fmt.Fprintln(out, name)
	}
	if d := ent.State.Description; d != "" {
		fmt.Fprintf(out, "  %s\n", d)
	}

	fields := fieldBlock(env, s, zone)
	if len(fields) > 0 {
		fmt.Fprintln(out)
		for _, line := range fields {
			fmt.Fprintln(out, strings.TrimRight(line, " "))
		}
	}

	if len(s.children) > 0 {
		fmt.Fprintln(out)
		printChildren(out, ent.Locator, s.children)
	}

	if len(s.skills) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, strings.TrimRight(labelled("skills", skillsCell(s.skills)), " "))
	}
}

// fieldBlock is §16.1's stored fields and computed lines. Derived values
// announce themselves by being computed lines in the second column — "in 181
// days", "31 days ago", "stale" — since none of them is stored (§2.5).
func fieldBlock(env *view.Env, s shown, zone *time.Location) []string {
	ent := s.ent
	var lines []string
	add := func(label, value string, computed ...string) {
		if value == "" {
			return
		}
		lines = append(lines, labelled(label, value, computed...))
	}

	add("status", ent.EffectiveStatus)
	add("priority", ent.State.Priority)
	if ent.State.Due != "" {
		// `due` prints as stored, never converted: it is a date somebody chose
		// rather than an instant something happened at (§15.1), so rendering it
		// in another zone would move a deadline nobody moved.
		add("due", ent.State.Due, until(env.DaysUntilIn(ent.Deadline, zone)))
	}
	if len(ent.State.Tags) > 0 {
		add("tags", strings.Join(ent.State.Tags, ", "))
	}
	if len(ent.State.Scope) > 0 {
		add("scope", strings.Join(ent.State.Scope, ", "))
	} else if ent.Kind == kindmeta.KindSkill {
		// §5.2: omitting scope entirely means the whole tree, and absence
		// saying something is exactly the case worth printing.
		add("scope", "the whole tree")
	}
	add("type", ent.State.Type)
	add("created", day(ent.Created, zone))
	add("attention", day(ent.Attention, zone), ago(env.DaysSinceIn(ent.Attention, zone)))
	if s.isStale {
		// §16.1: "stale names where its threshold came from", because §7's
		// chain resolution is only defensible if it is visible.
		lines = append(lines, labelled("", staleCell(s.stale)))
	}
	if note := dormancy(ent); note != "" {
		// §16.1: show says when an entity is dormant, "because its own fields
		// do not explain why it stopped appearing in list".
		add("dormant", note)
	}
	if kr := ent.KeyResult; kr != nil {
		if reading := readingCell(kr); reading != "" {
			add("reading", reading)
		}
		add("progress", derivedCell(kr.Outlook.Progress, kr.Outlook.HasProgress))
		add("pace", derivedCell(kr.Outlook.Pace, kr.Outlook.HasPace))
	}
	return lines
}

func labelled(label, value string, computed ...string) string {
	var b strings.Builder
	b.WriteString(label)
	for b.Len() < showLabelWidth {
		b.WriteByte(' ')
	}
	b.WriteString(value)
	for _, c := range computed {
		if c == "" {
			continue
		}
		for b.Len() < showLabelWidth+18 {
			b.WriteByte(' ')
		}
		b.WriteString(c)
	}
	return b.String()
}

// staleCell is the threshold and the file it came from (§7, §16.1).
func staleCell(th view.Threshold) string {
	from := th.From
	if from == "" {
		from = "the built-in default"
	} else {
		from = "from " + from
	}
	return fmt.Sprintf("stale (%s %s, %s)", th.Key, exact(th.Value), from)
}

// dormancy names what is quieting the entity, since neither answer is in its
// own fields (§1.6, §1.7).
func dormancy(e view.Entity) string {
	switch {
	case len(e.DormantUnder) > 0 && e.Archived:
		return fmt.Sprintf("archived, and under %s (%s)", entityLocatorString(e.DormantUnder), e.EffectiveStatus)
	case len(e.DormantUnder) > 0:
		return fmt.Sprintf("under %s (%s)", entityLocatorString(e.DormantUnder), e.EffectiveStatus)
	case e.Archived:
		return "archived"
	default:
		return ""
	}
}

// readingCell is §16.1's "480/9000 → 880/11000 / 2000/12000": the baseline, the
// newest reading, and the target, each as written (§4.1 keeps a ratio's
// denominator, so none of these is reduced to a decimal).
func readingCell(kr *view.KeyResult) string {
	if !kr.HasTarget {
		return ""
	}
	current := dash
	if kr.HasCurrent {
		current = kr.Current.Raw
	}
	if kr.HasStart {
		return fmt.Sprintf("%s → %s / %s", kr.Start.Raw, current, kr.Target.Raw)
	}
	return fmt.Sprintf("%s / %s", current, kr.Target.Raw)
}

func derivedCell(v float64, has bool) string {
	if !has {
		return dash
	}
	return number(v)
}

// printChildren is §16.1's children summary: a heading naming what is beneath,
// then one line per child indented by how deep it is, with a key-result's
// numbers on a line of its own underneath.
func printChildren(out io.Writer, subject locator.Locator, children []view.Entity) {
	fmt.Fprintln(out, childrenHeading(children))

	var t table
	for _, c := range children {
		depth := query.ReaderDepth(c.Locator, subject)
		t.add(strings.Repeat("  ", depth)+c.ID(), c.Name(), statusCell(c))
		if kr := c.KeyResult; kr != nil {
			t.note(strings.TrimSpace(fmt.Sprintf("%s   progress %s   pace %s",
				readingCell(kr),
				derivedCell(kr.Outlook.Progress, kr.Outlook.HasProgress),
				derivedCell(kr.Outlook.Pace, kr.Outlook.HasPace))))
		}
	}
	t.write(out)
}

// childrenHeading names what the summary is a summary of — §16.1's `objectives`
// line.
//
// It is the plural of the shallowest child's kind, which is well defined
// because §1.3 gives every kind exactly one kind of child: a project holds
// objectives, an objective key-results, an area areas. So the heading is the
// container's name where there is one and reads the same way where there is not.
func childrenHeading(children []view.Entity) string {
	return children[0].Kind.String() + "s"
}

// skillsCell is §16.1's skills line: each entry names the skill and the scope
// entry that reached it, so "why is this rule in my context" is answerable
// without a second command (§13.1).
func skillsCell(skills []view.SkillReach) string {
	entries := make([]string, 0, len(skills))
	for _, s := range skills {
		via := "whole tree"
		if !s.WholeTree {
			via = "scope " + s.Via
		}
		entries = append(entries, fmt.Sprintf("%s (from %s, %s)", s.Locator[len(s.Locator)-1], entityLocatorString(s.Locator), via))
	}
	return strings.Join(entries, "\n"+strings.Repeat(" ", showLabelWidth))
}
