package cli

import (
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/tree"
)

// Shell completion. The scripts are cobra's; what is para's is the candidate
// sets, and they are worth having for one reason: the locator (§1.4). Six
// segments, every one of which para already knows, and a mistyped one is an
// error rather than a near miss.
//
// Every candidate set here is derived from the same table the verb validates
// its argument against — §14 for what each verb accepts, §15's matrix for
// fields and their vocabularies, §7's spec list for config keys. A completion
// that offered something the verb refuses would be para disagreeing with
// itself, and the way to keep that from happening is to not write the answer
// down twice.
//
// Two rules hold throughout:
//
//   - **Silence on error.** A tree that cannot be found, a walk that fails, a
//     locator that does not parse: all of them return no candidates and no
//     message. A completion function's output lands in the middle of somebody's
//     command line, which is no place for a diagnostic.
//   - **No file completion anywhere**, except `init`'s path. Para takes
//     locators, not paths, so the shell's default of offering filenames is
//     always wrong — see denyFileCompletion, which applies it to every flag
//     nothing else claims.

// completer is cobra's completion signature, for both positional arguments and
// flag values.
type completer func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// noFiles is the directive every para completion carries: para has no path
// arguments outside `init`, so falling back to filenames is never right.
const noFiles = cobra.ShellCompDirectiveNoFileComp

// nodeFilter selects the walked nodes a verb accepts, which is §14's table read
// as a predicate.
type nodeFilter func(tree.Node) bool

// addressable is `show`, `log`, `activity`, and `path`: an entity or a
// container. Containers are offered even though `list` never prints one as a
// row, because §16.2's transparency is about rows and §14 is explicit that
// these take "entity or container".
func addressable(n tree.Node) bool { return true }

// entityOnly is `add`'s siblings in §14's first row — `set`, `unset`, `move`,
// `remove`, `archive`, `unarchive`, `note` — for which naming a container is
// the error §14 spells out.
func entityOnly(n tree.Node) bool { return !n.IsContainer }

// keyResultOnly is `measure`, §14's last row.
func keyResultOnly(n tree.Node) bool { return n.Kind == kindmeta.KindKeyResult }

// completeLocator offers the locators matching want, for the positional
// argument at index pos and no other.
func completeLocator(pos int, want nodeFilter) completer {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != pos {
			return nil, noFiles
		}
		return withPrefix(locators(want), toComplete), noFiles
	}
}

// locators walks the tree and returns the locators of the nodes want accepts,
// sorted. Archived things are included: they are addressable exactly as they
// were, and §16.2's rule about not traversing archive/ governs listing rather
// than typing.
func locators(want nodeFilter) []string {
	root, ok := completionRoot()
	if !ok {
		return nil
	}
	var out []string
	err := tree.Walk(root, func(n tree.Node) error {
		// A stub is a locator segment with no entity behind it (§1.6). Nothing
		// takes one as an argument, so nothing offers one.
		if !n.Stub && want(n) {
			out = append(out, n.Locator.String())
		}
		return nil
	})
	if err != nil {
		return nil
	}
	slices.Sort(out)
	return out
}

// completeScope is `list`, `review`, `rebuild`, and `doctor`: a place to look
// rather than a thing to print. Everything addressable is a place you can look
// inside of, and so is one thing that is not addressable — the `skills` bucket,
// which has no truth of its own for the walk to find (§1.4).
//
// **This follows the code rather than §14's table**, and the two differ. §14
// puts `review`, `rebuild`, and `doctor` in the entity-or-container row beside
// `show`/`log`/`activity`/`path`, and gives "container, bucket, or root" to
// `list` alone — but all four of these verbs accept `skills` today and the
// other four refuse it. The completion must match what the verb does, since a
// candidate the verb rejects is worse than one it never offered; the
// divergence itself is recorded in the plan for whoever reconciles it.
func completeScope(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, noFiles
	}
	places := locators(addressable)
	if len(places) > 0 || treeFound() {
		places = append(places, skillsBucket)
		slices.Sort(places)
	}
	return withPrefix(places, toComplete), noFiles
}

// skillsBucket is the one locator segment that names a place without naming a
// directory the walk visits (§1.4).
const skillsBucket = "skills"

func treeFound() bool {
	_, ok := completionRoot()
	return ok
}

// completeAddParent is `add`'s, and it is the one locator completion that
// cannot offer what exists: `add` names a thing that does not (§18.1). So it
// offers the places a new id may go — each with its trailing dot and no space
// after it, because the id is still to be typed.
func completeAddParent(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, noFiles
	}
	// Nothing may be created under archive/: things arrive there by `archive`
	// (§1.6), never by being created there.
	return prefixes(nestingPlaces(live, anyKind), toComplete)
}

// completeMoveTarget is `move`'s second argument, and it is a destination
// rather than a thing: §18.3's target must not already exist, must give the
// entity the same kind it has now, and must be on the same side of the archive
// boundary — which `move` refuses to cross in either direction.
//
// So it offers prefixes, exactly as `add` does, narrowed to the parents that
// would preserve the kind. Offering the tree's existing entities here would be
// offering every locator `move` is about to refuse.
func completeMoveTarget(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	from, ok := kindOfArg(first(args))
	if !ok {
		return nil, noFiles
	}
	loc, err := locator.Parse(first(args))
	if err != nil {
		return nil, noFiles
	}
	side := live
	if loc.IsArchived() {
		side = archived
	}

	// An entity cannot move inside itself — "areas.health.inner is inside
	// areas.health" is the refusal — so its own subtree is not a destination
	// even though every position in it derives the right kind.
	var places []locator.Locator
	for _, place := range nestingPlaces(side, from) {
		if !within(place, loc) {
			places = append(places, place)
		}
	}
	return prefixes(places, toComplete)
}

// within reports whether loc is at or beneath ancestor.
func within(loc, ancestor locator.Locator) bool {
	if len(loc) < len(ancestor) {
		return false
	}
	return slices.Equal(loc[:len(ancestor)], ancestor)
}

// prefixes renders places as the `parent.` prefixes a not-yet-typed id hangs
// off, with no space after the dot.
func prefixes(places []locator.Locator, toComplete string) ([]string, cobra.ShellCompDirective) {
	out := make([]string, 0, len(places))
	for _, loc := range places {
		out = append(out, loc.String()+".")
	}
	slices.Sort(out)
	return withPrefix(out, toComplete), cobra.ShellCompDirectiveNoSpace | noFiles
}

// side selects which half of the tree a destination may be in. `move` refuses
// to cross the archive boundary (§18.3) and `add` refuses to reach across it
// at all (§1.6), so neither ever wants both halves at once.
type side bool

const (
	live     side = false
	archived side = true
)

// anyKind is nestingPlaces' "no narrowing", for `add`, which may create any
// kind the position allows.
const anyKind = kindmeta.KindUnknown

// nestingPlaces returns the locators a new id may be created directly beneath,
// on the given side of the archive boundary, and — when want is not anyKind —
// only those where the new id would take that kind.
//
// The kinds come from §1.3's derivation rather than from a table here: probe
// the position with an id that is legal everywhere and ask what kind comes
// back. That is the same question `add` answers when it refuses.
func nestingPlaces(where side, want kindmeta.Kind) []locator.Locator {
	root, ok := completionRoot()
	if !ok {
		return nil
	}

	// The buckets are seeded rather than walked. On the live side, three of
	// them the walk does find, but `skills` it cannot — skills live under
	// .agents/skills/ and the bucket itself is not a directory with truth in it
	// (§1.4). Skills cannot be archived at all (§1.4), so the archived side has
	// three.
	places := []locator.Locator{{"projects"}, {"areas"}, {"resources"}, {skillsBucket}}
	if where == archived {
		places = []locator.Locator{{"archive", "projects"}, {"archive", "areas"}, {"archive", "resources"}}
	}
	err := tree.Walk(root, func(n tree.Node) error {
		if !n.Stub && n.Archived == bool(where) && len(n.Locator) > 1 {
			places = append(places, n.Locator)
		}
		return nil
	})
	if err != nil {
		return nil
	}

	out := make([]locator.Locator, 0, len(places))
	for _, loc := range places {
		kind, ok := nestedKind(loc)
		if !ok || (want != anyKind && kind != want) {
			continue
		}
		out = append(out, loc)
	}
	return out
}

// nestedKind is the kind a new id directly beneath loc would take, or false
// where no id may go there at all.
func nestedKind(loc locator.Locator) (kindmeta.Kind, bool) {
	info, err := kindmeta.KindOf(append(slices.Clone(loc), "x"))
	if err != nil {
		return kindmeta.KindUnknown, false
	}
	return info.Kind, true
}

// canNest reports whether a new id may be created directly beneath loc.
func canNest(loc locator.Locator) bool {
	_, ok := nestedKind(loc)
	return ok
}

// completeUnset is the locator, then the kind's own unsettable fields, repeated
// for as many as are typed (§13's `unset <locator> <field>…`).
func completeUnset(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return completeLocator(0, entityOnly)(cmd, args, toComplete)
	}
	kind, ok := kindOfArg(args[0])
	if !ok {
		return nil, noFiles
	}
	var out []string
	for _, f := range kindmeta.UnsettableFields(kind) {
		// A field already named is not offered again: `unset x tags tags` is
		// the same field twice, which is at best noise.
		if !slices.Contains(args[1:], string(f)) {
			out = append(out, string(f))
		}
	}
	return withPrefix(out, toComplete), noFiles
}

// completeStatus is §15's status vocabulary for the kind being addressed, which
// the locator already typed says — a key-result's settable statuses are not a
// project's (§4.3). With no locator to read, every kind's are offered.
func completeStatus(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out := kindmeta.AllStatuses()
	if kind, ok := kindOfArg(first(args)); ok {
		out = kindmeta.SettableStatuses(kind)
	}
	return withPrefix(out, toComplete), noFiles
}

// completeConfigKey offers §7's keys, which are a fixed vocabulary and long
// enough to be worth not memorising.
func completeConfigKey(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, noFiles
	}
	var out []string
	for _, spec := range config.Specs() {
		out = append(out, spec.Key)
	}
	return withPrefix(out, toComplete), noFiles
}

// completeConfigValue offers the values the key already typed accepts, which
// §7's spec table says for two of its four types: an enum's members, and a
// boolean's two spellings. An int or a float is a number nobody can enumerate,
// and offers nothing.
func completeConfigValue(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	spec, ok := config.Lookup(first(args))
	if !ok {
		return nil, noFiles
	}
	switch spec.Type {
	case config.TypeEnum:
		return withPrefix(spec.Enum, toComplete), noFiles
	case config.TypeBool:
		return withPrefix([]string{"false", "true"}, toComplete), noFiles
	default:
		return nil, noFiles
	}
}

// fixed is a completion over a vocabulary that does not depend on the tree.
func fixed(values []string) completer {
	return func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return withPrefix(values, toComplete), noFiles
	}
}

// nothing completes an argument that has no candidates — the prose `note` takes,
// the value `measure` takes. It still refuses filenames.
func nothing(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return nil, noFiles
}

// kindOfArg derives the kind of an already-typed locator argument, or false if
// it names nothing derivable. `.` is not resolved: it means the working
// directory (§14), and a completion that read the filesystem to answer what a
// flag accepts would be doing more work than the flag is worth.
func kindOfArg(arg string) (kindmeta.Kind, bool) {
	if arg == "" || arg == "." {
		return kindmeta.KindUnknown, false
	}
	loc, err := locator.Parse(arg)
	if err != nil {
		return kindmeta.KindUnknown, false
	}
	info, err := kindmeta.KindOf(loc)
	if err != nil {
		return kindmeta.KindUnknown, false
	}
	return info.Kind, true
}

func first(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// withPrefix keeps the candidates that extend what has been typed. Cobra hands
// the whole list to the shell and most shells filter it themselves; filtering
// here as well is what makes the candidate set the same under every shell, and
// what makes it assertable in a script test.
func withPrefix(candidates []string, toComplete string) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		if strings.HasPrefix(c, toComplete) {
			out = append(out, c)
		}
	}
	return out
}

// completionRoot finds the tree the completion is being typed in, or false.
func completionRoot() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root, err := tree.Find(cwd)
	if err != nil {
		return "", false
	}
	return root, true
}

// registerCompletions attaches every candidate set to the command tree.
//
// It is one function rather than a line in each command's constructor so that
// §14's table can be read off in one place — the table is the thing being
// implemented, and a table implemented in nineteen places is a table nobody can
// check.
func registerCompletions(root *cobra.Command) {
	byName := map[string]*cobra.Command{}
	for _, cmd := range root.Commands() {
		byName[cmd.Name()] = cmd
	}

	// §14: entity or container.
	for _, name := range []string{"show", "log", "activity", "path"} {
		setArgCompletion(byName[name], completeLocator(0, addressable))
	}
	// A place to look inside of, the `skills` bucket included — see
	// completeScope for why that set is the code's rather than §14's.
	for _, name := range []string{"list", "review", "rebuild", "doctor"} {
		setArgCompletion(byName[name], completeScope)
	}
	// §14: entity only.
	for _, name := range []string{"set", "remove", "archive", "unarchive"} {
		setArgCompletion(byName[name], completeLocator(0, entityOnly))
	}
	// The three that take an entity and then something else.
	setArgCompletion(byName["note"], positional(completeLocator(0, entityOnly), nothing))
	setArgCompletion(byName["measure"], positional(completeLocator(0, keyResultOnly), nothing))
	setArgCompletion(byName["move"], positional(completeLocator(0, entityOnly), completeMoveTarget))
	setArgCompletion(byName["unset"], completeUnset)
	setArgCompletion(byName["add"], completeAddParent)

	// `init` is the one command whose argument is a filesystem path, so it is
	// the one place the shell's own file completion is right — narrowed to
	// directories, since a tree is one.
	setArgCompletion(byName["init"], func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, noFiles
		}
		return nil, cobra.ShellCompDirectiveFilterDirs
	})

	// §22's four shapes, each with its own second argument: `set <key>
	// <value>`, `unset <key>`, `show <key> [<locator>]`, `list [<locator>]`.
	// One loop, four cases, because the shapes genuinely differ — offering a
	// locator where a value belongs is the mistake this spells out to avoid.
	for _, sub := range byName["config"].Commands() {
		switch sub.Name() {
		case "set":
			setArgCompletion(sub, positional(completeConfigKey, completeConfigValue))
		case "unset":
			setArgCompletion(sub, positional(completeConfigKey))
		case "show":
			setArgCompletion(sub, positional(completeConfigKey, completeLocator(1, addressable)))
		case "list":
			setArgCompletion(sub, completeLocator(0, addressable))
		}
		// --at names the level to write at, which is a locator wherever it
		// appears under config (§22). It is the only flag in para that takes
		// one, and it shares its name with `note`/`measure`'s instant — which
		// is why it is registered here, per command, rather than by name across
		// the tree.
		if sub.Flags().Lookup("at") != nil {
			_ = sub.RegisterFlagCompletionFunc("at", func(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
				return withPrefix(locators(addressable), toComplete), noFiles
			})
		}
	}

	registerFlagCompletions(root)
	denyFileCompletion(root)
}

// positional dispatches on which argument is being typed. Past the last
// completer there is nothing to offer, which is how a fixed-arity command stops
// suggesting.
func positional(byIndex ...completer) completer {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= len(byIndex) {
			return nil, noFiles
		}
		return byIndex[len(args)](cmd, args, toComplete)
	}
}

func setArgCompletion(cmd *cobra.Command, fn completer) {
	if cmd != nil {
		cmd.ValidArgsFunction = fn
	}
}

// registerFlagCompletions attaches the §15 and §17 vocabularies to the flags
// that take them, wherever those flags appear. The flag name is the key
// because §15's "one spelling per field, shared by add and set" makes it one:
// `--status` means the same thing on every command that has it.
func registerFlagCompletions(root *cobra.Command) {
	byFlag := map[string]completer{
		string(kindmeta.FieldStatus):   completeStatus,
		string(kindmeta.FieldPriority): fixed(kindmeta.Priorities()),
		string(kindmeta.FieldType):     fixed(krvalue.TypeNames()),
		"sort":                         fixed(sortKeyNames()),
		"kind":                         fixed(eventKindNames()),
	}
	walkCommands(root, func(cmd *cobra.Command) {
		for name, fn := range byFlag {
			if cmd.Flags().Lookup(name) == nil {
				continue
			}
			// The error is the "already registered" one, which cannot happen
			// here: this runs once, before denyFileCompletion, on a freshly
			// built tree.
			_ = cmd.RegisterFlagCompletionFunc(name, fn)
		}
	})
}

// denyFileCompletion gives every flag nothing else claimed a completion that
// offers nothing and refuses filenames.
//
// Para takes locators, dates, numbers, and tags — not paths (§13's only path
// argument is `init`'s). So the shell's default of completing filenames after
// `--due ` is always wrong, and turning it off flag by flag as they are added
// is a rule that would be forgotten. This applies it to the flag set as it
// actually is.
func denyFileCompletion(root *cobra.Command) {
	walkCommands(root, func(cmd *cobra.Command) {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			if _, ok := cmd.GetFlagCompletionFunc(f.Name); ok {
				return
			}
			_ = cmd.RegisterFlagCompletionFunc(f.Name, nothing)
		})
	})
}

// walkCommands visits root and every command beneath it.
func walkCommands(root *cobra.Command, visit func(*cobra.Command)) {
	visit(root)
	for _, cmd := range root.Commands() {
		walkCommands(cmd, visit)
	}
}

func sortKeyNames() []string {
	keys := query.SortKeys()
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = string(k)
	}
	return out
}

func eventKindNames() []string {
	out := make([]string, len(eventKinds))
	for i, k := range eventKinds {
		out[i] = string(k)
	}
	return out
}
