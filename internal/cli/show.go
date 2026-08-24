package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/view"
)

// attentionSnippetLength is how much of a note's own text names it in show's
// attention line and review's stale group (§18.6, §20) — enough to recognise
// which note it was, not the whole thing; ACTIVITY.md remains where the full
// text lives.
const attentionSnippetLength = 50

// attentionSource names what set the attention clock — "measurement", or a
// note's own text truncated to attentionSnippetLength runes — so a reader can
// tell a note moved it from a measurement, and which note, without opening
// ACTIVITY.md. Empty when nothing has beaten `created` yet (AttentionKind is
// empty), which is also true of a container and of an entity kind that keeps
// no journal.
func attentionSource(ent view.Entity) string {
	switch ent.AttentionKind {
	case journal.KindMeasurement:
		return "measurement"
	case journal.KindNote:
		return "note: " + truncateRunes(ent.AttentionNote, attentionSnippetLength)
	default:
		return ""
	}
}

// truncateRunes shortens s to at most n runes, marking the cut with an
// ellipsis — by runes rather than bytes, so a multi-byte character never
// splits into invalid UTF-8.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

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
			// "." is checked before parseAddressArgs, because at the tree
			// root it names something parseAddressArgs' arity has no shape
			// for at all: not an entity, not a bucket, but the tree itself
			// (para-xbb) — the one address show can reach that no noun and
			// chain ever can, since the root has no locator of its own
			// (§8.1, §14).
			if len(args) > 0 && args[0] == "." {
				loc, atRoot, err := tree.ResolveDotOrRoot(env.Root, cwd)
				if err != nil {
					return err
				}
				if atRoot {
					return showRoot(cmd, env, read)
				}
				return showEntity(cmd, env, loc, read)
			}
			loc, _, err := parseAddressArgs(env.Root, cwd, args, bucketArity, archived.value)
			if err != nil {
				return err
			}
			return showEntity(cmd, env, loc, read)
		},
	}
	read.register(cmd)
	archived.register(cmd)
	return cmd
}

// showEntity is `show <noun> [<chain>]` once loc names a real entity or
// container — every case parseAddressArgs' arities cover, and dot-at-root's
// non-root outcome besides.
func showEntity(cmd *cobra.Command, env *view.Env, loc locator.Locator, read readFlags) error {
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
}

// rootContainerNames are the tree's own children (para-xbb): the four
// buckets §8.1's `init` journals as the root's four `child` events
// (mutate.rootPlan), in that same order. The skill bucket is deliberately
// not among them — it never joins that journaled set either (see
// mutate.bucketPlans' own reasoning) — so `show .` at the root names exactly
// what the root's own journal already claims as its children, not every
// container the tree happens to hold.
var rootContainerNames = []string{"projects", "areas", "resources", "archive"}

// showRoot is `show .` at the tree root (para-xbb): the root has
// .para/tree.toml and is plainly a thing para tracks, but it is not an
// entity or a container (§8.1) — the previous refusal, "no entity or
// container contains …", read as though the tree were unset up rather than
// simply not the kind of thing "." had ever been allowed to name. It is
// addressable here instead, the same "what is this tree" summary any other
// `show` gives: identity, then containers.
func showRoot(cmd *cobra.Command, env *view.Env, read readFlags) error {
	t, err := truth.ReadTree(env.Root)
	if err != nil {
		return err
	}
	containers, err := loadRootContainers(env)
	if err != nil {
		return err
	}
	configKeys, err := rootConfigKeyCount(env.Root)
	if err != nil {
		return err
	}

	if read.json {
		return writeJSON(cmd.OutOrStdout(), rootOutputOf(env, t, containers, configKeys))
	}
	printRoot(cmd.OutOrStdout(), t, containers, configKeys, read.zone(env))
	return nil
}

// loadRootContainers loads the four buckets rootContainerNames names, each
// exactly the way `show project` (the bare noun, R17's bucket) already
// loads one.
func loadRootContainers(env *view.Env) ([]view.Entity, error) {
	out := make([]view.Entity, 0, len(rootContainerNames))
	for _, name := range rootContainerNames {
		ent, err := env.Load(locator.Locator{name})
		if err != nil {
			return nil, err
		}
		out = append(out, ent)
	}
	return out, nil
}

// rootConfigKeyCount is how many keys the root's own config.toml sets —
// not the resolved chain `config list` walks (the root has no ancestor to
// resolve against), just what is actually written there, the same thing
// `show` prints for every other stored field: what is in the file, not a
// derivation over it.
func rootConfigKeyCount(root string) (int, error) {
	f, err := config.Read(truth.ConfigPath(root))
	if err != nil {
		return 0, err
	}
	return len(f.Keys()), nil
}

// printRoot is showRoot's human-readable shape: the tree's name and
// description the way any other `show` header prints them, its stored
// fields, then its containers — the same "children in summary" §16.1 gives
// every other kind, reusing printChildren so the two never format a child
// row two different ways.
func printRoot(out io.Writer, t truth.Tree, containers []view.Entity, configKeys int, zone *time.Location) {
	var head table
	head.add("tree", t.Name)
	head.write(out)
	if t.Description != "" {
		fmt.Fprintln(out, t.Description)
	}

	var fields []string
	if created, ok := ptime.StoredAt(t.Created); ok {
		fields = append(fields, labelled("created", day(created, zone)))
	}
	fields = append(fields, labelled("config", configSummary(configKeys)))
	fmt.Fprintln(out)
	for _, line := range fields {
		fmt.Fprintln(out, strings.TrimRight(line, " "))
	}

	if len(containers) > 0 {
		fmt.Fprintln(out)
		printChildren(out, nil, containers)
	}
}

// configSummary is showRoot's one-line answer to "is anything configured
// here", pointing at the command that has the real answer rather than
// reprinting `config list`'s own table inside `show`.
func configSummary(keys int) string {
	if keys == 0 {
		return "none set"
	}
	if keys == 1 {
		return "1 key set — see `para config list`"
	}
	return fmt.Sprintf("%d keys set — see `para config list`", keys)
}

// rootOutput is `show .` at the root, as --json reports it (para-xbb): the
// tree's own identity fields, which no locator-addressed --json shape can
// otherwise carry (§8.1's root has no locator), plus its containers in the
// same entityJSON shape every other `show --json` child uses.
type rootOutput struct {
	Schema      int64        `json:"schema"`
	ParaVersion string       `json:"para-version"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Created     string       `json:"created,omitempty"`
	ConfigKeys  int          `json:"config-keys"`
	Containers  []entityJSON `json:"containers"`
}

func rootOutputOf(env *view.Env, t truth.Tree, containers []view.Entity, configKeys int) rootOutput {
	out := rootOutput{
		Schema:      t.Schema,
		ParaVersion: t.ParaVersion,
		Name:        t.Name,
		Description: t.Description,
		ConfigKeys:  configKeys,
		Containers:  make([]entityJSON, 0, len(containers)),
	}
	if created, ok := ptime.StoredAt(t.Created); ok {
		out.Created = created.UTC().Format(time.RFC3339)
	}
	for _, c := range containers {
		out.Containers = append(out.Containers, newEntityJSON(env, c))
	}
	return out
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
	attentionComputed := ago(env.DaysSinceIn(ent.Attention, zone))
	if src := attentionSource(ent); src != "" {
		attentionComputed += "   " + src
	}
	add("attention", day(ent.Attention, zone), attentionComputed)
	if untilDate, note, end, ok := env.ActiveSuppression(ent); ok {
		suppressionComputed := until(env.DaysUntilIn(end, zone))
		if note != "" {
			suppressionComputed += "   " + note
		}
		add("suppression", untilDate, suppressionComputed)
	}
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
	return fmt.Sprintf("stale (%s %s, %s)", th.Key, staleValue(th), from)
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
