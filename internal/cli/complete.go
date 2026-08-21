package cli

import (
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/view"
)

// Shell completion. The scripts are cobra's; what is para's is the candidate
// sets, and they are worth having for one reason: the address (R1). A noun at
// position 0, an id-chain at position 1 — every segment of which para already
// knows, and a mistyped one is an error rather than a near miss.
//
// Every candidate set here is derived from the same table the verb validates
// its argument against — R12 for what each verb accepts, §15's matrix for
// fields and their vocabularies, §7's spec list for config keys. A completion
// that offered something the verb refuses would be para disagreeing with
// itself, and the way to keep that from happening is to not write the answer
// down twice.
//
// Two rules hold throughout:
//
//   - **Silence on error.** A tree that cannot be found, a walk that fails, a
//     noun or chain that does not parse: all of them return no candidates and
//     no message. A completion function's output lands in the middle of
//     somebody's command line, which is no place for a diagnostic.
//   - **No file completion anywhere**, except `init`'s path. Para takes
//     addresses, not paths, so the shell's default of offering filenames is
//     always wrong — see denyFileCompletion, which applies it to every flag
//     nothing else claims.

// completer is cobra's completion signature, for both positional arguments and
// flag values.
type completer func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

// noFiles is the directive every para completion carries: para has no path
// arguments outside `init`, so falling back to filenames is never right.
const noFiles = cobra.ShellCompDirectiveNoFileComp

// nodeAddress converts n to the external address it names, or false when it
// has none: a stub (R11 — no noun, no address, because a stub has no kind and
// the grammar's first token is a kind) or anything address.FromLocator itself
// refuses.
func nodeAddress(n tree.Node) (address.Address, bool) {
	if n.Stub {
		return address.Address{}, false
	}
	a, err := address.FromLocator(n.Locator)
	if err != nil {
		return address.Address{}, false
	}
	return a, true
}

// chainCandidates returns the dotted chains (R1's second token) of every
// existing, non-bucket address whose noun is noun and whose archive side
// matches archived, sorted. This is the one walk every chain completion in
// this file funnels through — the counterpart of add's own completeAddChain,
// which walks for a *parent* rather than an existing entity.
func chainCandidates(noun kindmeta.Kind, archived bool) []string {
	root, ok := completionRoot()
	if !ok {
		return nil
	}
	var out []string
	err := tree.Walk(root, func(n tree.Node) error {
		a, ok := nodeAddress(n)
		if !ok || a.Noun != noun || a.Archived != archived || len(a.Chain) == 0 {
			return nil
		}
		out = append(out, strings.Join(a.Chain, "."))
		return nil
	})
	if err != nil {
		return nil
	}
	slices.Sort(out)
	return out
}

// dottedAddresses returns every existing, non-bucket address in R24's
// one-token form, of any noun and either archive side, sorted — for the two
// places an address must be a single token: config's `--at` and `show`'s own
// second argument, and `--scope`, which is a comma-separated list of them.
func dottedAddresses() []string {
	root, ok := completionRoot()
	if !ok {
		return nil
	}
	var out []string
	err := tree.Walk(root, func(n tree.Node) error {
		a, ok := nodeAddress(n)
		if !ok || len(a.Chain) == 0 {
			return nil
		}
		out = append(out, a.String())
		return nil
	})
	if err != nil {
		return nil
	}
	slices.Sort(out)
	return out
}

// archivedFlagSet reads --archived (R7) off cmd, wherever it is registered.
// cobra parses every flag typed so far before calling a completion function,
// so this sees exactly what the command line already carries — which is how
// --archived flips the candidate set (task P21.6) without this file needing
// to reparse anything itself. A command with no such flag (measure, R13's own
// irregularity) reads false, its zero value.
func archivedFlagSet(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("archived")
	return v
}

// completeNoun offers words — R2's noun vocabulary, already narrowed to
// whichever subset the calling verb accepts — for the positional argument at
// index 0 and no other.
func completeNoun(words []string) completer {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, noFiles
		}
		return withPrefix(words, toComplete), noFiles
	}
}

// sevenNounWords is R2's seven words in address.AllNouns' order, for the
// commands where a container is a legal address (R17): show, log, path,
// activity, review, rebuild, doctor. addableNounWords (write.go) is the other
// six — reused rather than re-listed, since add, set, and unset already
// define that vocabulary for their own noun-dispatching subcommands.
func sevenNounWords() []string {
	nouns := address.AllNouns()
	words := make([]string, len(nouns))
	for i, n := range nouns {
		words[i] = n.String()
	}
	return words
}

// completeChain offers the chains an already-typed noun (args[0]) accepts —
// the R17 "entity or container" and entity-only shapes both, since chain
// candidates come from the noun alone and neither shape narrows it further.
// It is silent if args[0] does not parse as a noun at all, which cannot
// happen through cobra's own dispatch but can through a hand-typed one.
func completeChain(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	noun, err := address.ParseNoun(first(args))
	if err != nil {
		return nil, noFiles
	}
	return withPrefix(chainCandidates(noun, archivedFlagSet(cmd)), toComplete), noFiles
}

// completeChainFixed is completeChain's counterpart for add's, set's, and
// unset's noun-dispatching subcommands (R3): the noun is already fixed by
// which subcommand cobra dispatched to, so the chain is the *first* argument
// rather than the second.
func completeChainFixed(kind kindmeta.Kind) completer {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, noFiles
		}
		return withPrefix(chainCandidates(kind, archivedFlagSet(cmd)), toComplete), noFiles
	}
}

// completeUnsetField is unset's dispatched subcommand shape: the chain
// (completeChainFixed's own question), then the kind's own unsettable
// fields, repeated for as many as are typed (§13's `unset <noun> <chain>
// <field>…`).
func completeUnsetField(kind kindmeta.Kind) completer {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return withPrefix(chainCandidates(kind, archivedFlagSet(cmd)), toComplete), noFiles
		}
		var out []string
		for _, f := range kindmeta.UnsettableFields(kind) {
			// A field already named is not offered again: `unset x tags tags`
			// is the same field twice, which is at best noise.
			if !slices.Contains(args[1:], string(f)) {
				out = append(out, string(f))
			}
		}
		return withPrefix(out, toComplete), noFiles
	}
}

// addParent maps a noun to the noun whose own existing addresses are legal
// parents for a new id of that noun (R3's nesting structure): area and
// resource nest under themselves to any depth; objective nests under
// project; key-result nests under objective. Project and skill have no
// entry — neither nests, so an id is typed with nothing to complete, and
// container is add's own refusal (R15) rather than a nesting question.
var addParent = map[kindmeta.Kind]kindmeta.Kind{
	kindmeta.KindArea:      kindmeta.KindArea,
	kindmeta.KindResource:  kindmeta.KindResource,
	kindmeta.KindObjective: kindmeta.KindProject,
	kindmeta.KindKeyResult: kindmeta.KindObjective,
}

// completeAddChain is `add`'s dispatched-subcommand chain completion (task
// P21.3), and it is the one chain completion that cannot offer what exists:
// `add` names a thing that does not (§18.1). So it offers the places a new id
// may go, narrowed by the noun already fixed — `para add key-result <TAB>`
// offers objective chains, never every nesting place in the tree the way
// today's completeAddParent does regardless of what is being created.
//
// Nothing is created under archive/ (R8), so this never reads --archived.
func completeAddChain(kind kindmeta.Kind) completer {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, noFiles
		}
		parent, ok := addParent[kind]
		if !ok {
			return nil, noFiles
		}
		return prefixDots(chainCandidates(parent, false), toComplete)
	}
}

// prefixDots renders chains as the not-yet-typed-id prefixes add's and
// move's own chain arguments hang a new segment off: "chain." with no space,
// so the id can be typed right after.
func prefixDots(chains []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	out := make([]string, len(chains))
	for i, c := range chains {
		out[i] = c + "."
	}
	return withPrefix(out, toComplete), cobra.ShellCompDirectiveNoSpace | noFiles
}

// completeMoveTarget is `move`'s third argument (task P21.4): a destination
// rather than a thing, narrowed by the noun already given once (R14) rather
// than derived from the address the way today's completeMoveTarget derives it
// from the first argument's own locator. §18.3's target must not already
// exist and must give the entity the same kind it has now, so this is
// completeAddChain's own question — the places a new id of this kind may go
// — with one more restriction add never needs: an entity cannot move inside
// itself, so the source's own chain and everything beneath it is excluded.
func completeMoveTarget(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 2 {
		return nil, noFiles
	}
	kind, err := address.ParseNoun(args[0])
	if err != nil {
		return nil, noFiles
	}
	parent, ok := addParent[kind]
	if !ok {
		return nil, noFiles
	}
	from := args[1]
	archived := archivedFlagSet(cmd)
	var out []string
	for _, c := range chainCandidates(parent, archived) {
		if c == from || strings.HasPrefix(c, from+".") {
			continue
		}
		out = append(out, c)
	}
	return prefixDots(out, toComplete)
}

// completeList is `list`'s own grammar (task P21.5), the completion mirror of
// R19's lookahead rule: a kind filter at position 0 (container excluded,
// R20), then — because the second argument is genuinely ambiguous until it
// is typed in full — the union of every noun word (for "first is a filter,
// second is a bucket scope") and the first noun's own chains (for "the pair
// is a scope") at position 1, then the second noun's own chains at position
// 2 when the second argument parsed as a noun at all.
//
// `container` at position 0 breaks that ambiguity rather than sharing it: it
// can never play the kind-filter role (R20), so a second *noun* can never
// follow it — parseListArgs (list_args.go) refuses `list container <noun>
// …` outright, for any noun and anything after it. The only reading left is
// "the pair is a scope", so position 1 offers only container's own chains,
// and position 2 offers nothing at all. Without this, position 1 would offer
// six noun words that parseListArgs is guaranteed to refuse the moment one
// is chosen — para disagreeing with itself, which is exactly the failure
// this file's own doc comment says a completion must never cause.
func completeList(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	archived := archivedFlagSet(cmd)
	switch len(args) {
	case 0:
		return withPrefix(addableNounWords(), toComplete), noFiles
	case 1:
		noun, err := address.ParseNoun(args[0])
		if err != nil {
			return nil, noFiles
		}
		if noun == address.Container {
			return withPrefix(chainCandidates(noun, archived), toComplete), noFiles
		}
		out := append([]string{}, sevenNounWords()...)
		out = append(out, chainCandidates(noun, archived)...)
		slices.Sort(out)
		return withPrefix(out, toComplete), noFiles
	case 2:
		if args[0] == address.Container.String() {
			return nil, noFiles
		}
		noun, err := address.ParseNoun(args[1])
		if err != nil {
			return nil, noFiles
		}
		return withPrefix(chainCandidates(noun, archived), toComplete), noFiles
	default:
		return nil, noFiles
	}
}

// completeDottedAddress offers R24's one-token form of every existing
// address, for the two places that read a whole address as one string rather
// than as noun-then-chain: `config`'s `--at` flag and `config show`'s own
// second argument.
func completeDottedAddress(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return withPrefix(dottedAddresses(), toComplete), noFiles
}

// completeScopeValue is `--scope`'s completion (R24, task P21.7): a
// comma-separated list of dotted addresses (`--scope project.acme,area.growth`),
// one of which is being typed at any moment. Only the text after the last
// comma is completed; whatever precedes it is kept verbatim in front of every
// candidate, so completion narrows the entry being typed rather than the
// whole flag value.
func completeScopeValue(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	head, last := splitLastComma(toComplete)
	matches := withPrefix(dottedAddresses(), last)
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = head + m
	}
	return out, cobra.ShellCompDirectiveNoSpace | noFiles
}

// splitLastComma splits s at its last comma, head holding everything up to
// and including it (empty when there is none) and last holding the rest —
// the entry currently being typed.
func splitLastComma(s string) (head, last string) {
	if i := strings.LastIndex(s, ","); i >= 0 {
		return s[:i+1], s[i+1:]
	}
	return "", s
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
// §7's spec table says for two of its five types: an enum's members, and a
// boolean's two spellings. An int, a float, or a day count is a number nobody
// can enumerate, and offers nothing.
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

// nothing completes an argument that has no candidates — the prose `note`
// takes, the value `measure` takes. It still refuses filenames.
func nothing(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return nil, noFiles
}

// completeFilterStatus is `--status` in its *filter* role: §17's filter,
// which compares against effective status and so reaches the four a
// key-result derives (§4.3) as well as the five any kind can be set to.
//
// It deliberately does not read the first argument. On a filtering verb that
// argument is the *scope* — a place to look — and the rows under it are
// generally not its kind: narrowing to a project's vocabulary inside
// `list project acme` would hide `on-track`, which is exactly what the
// key-results under it report.
func completeFilterStatus(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return withPrefix(view.EffectiveStatuses(), toComplete), noFiles
}

// fieldApplies reports whether the verb would accept this flag for this kind:
// the kind must have the field (§15), and `set` additionally refuses one fixed
// at creation, which is `mutate.Set`'s own first check.
func fieldApplies(kind kindmeta.Kind, field kindmeta.Field, atCreation bool) bool {
	if !kindmeta.Has(kind, field) {
		return false
	}
	return atCreation || kindmeta.Requirement(kind, field) != kindmeta.RequiredFixed
}

// fieldVocabulary is the set of values kind accepts for field, for the three
// fields that have a closed one. Everything else in §15 is free text.
func fieldVocabulary(kind kindmeta.Kind, field kindmeta.Field) []string {
	switch field {
	case kindmeta.FieldStatus:
		return kindmeta.SettableStatuses(kind)
	case kindmeta.FieldPriority:
		return kindmeta.Priorities()
	case kindmeta.FieldType:
		return krvalue.TypeNames()
	default:
		return nil
	}
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
// R12's table can be read off in one place — the table is the thing being
// implemented, and a table implemented in nineteen places is a table nobody
// can check.
func registerCompletions(root *cobra.Command) {
	byName := map[string]*cobra.Command{}
	for _, cmd := range root.Commands() {
		byName[cmd.Name()] = cmd
	}

	// R17: entity or container — a noun alone, followed by a chain narrowed
	// by that noun. `path` sits here too, unlike the old §14 table's split:
	// R11 gives every one of these commands the same rule now that stubs are
	// addressless everywhere rather than special-cased for `path` alone.
	for _, name := range []string{"show", "log", "path", "activity", "review", "rebuild", "doctor"} {
		setArgCompletion(byName[name], positional(completeNoun(sevenNounWords()), completeChain))
	}

	// R19's own lookahead, on its own (task P21.5).
	setArgCompletion(byName["list"], completeList)

	// R17: entity only — container is refused, so it is never offered.
	for _, name := range []string{"archive", "unarchive", "remove"} {
		setArgCompletion(byName[name], positional(completeNoun(addableNounWords()), completeChain))
	}
	setArgCompletion(byName["note"], positional(completeNoun(addableNounWords()), completeChain, nothing))
	setArgCompletion(byName["move"], positional(completeNoun(addableNounWords()), completeChain, completeMoveTarget))
	setArgCompletion(byName["measure"], positional(completeChainFixed(address.KeyResult), nothing))

	// `add`, `set`, and `unset` dispatch to one subcommand per noun (R3), and
	// cobra completes a parent's own subcommand names natively — container
	// excluded because newNounDispatchCmd never registers one for it (R15) —
	// so only each subcommand's own chain position (and unset's field names)
	// needs a completer here.
	for _, kind := range kindmeta.AllKinds() {
		setArgCompletion(nounSubcommand(byName["add"], kind), completeAddChain(kind))
		setArgCompletion(nounSubcommand(byName["set"], kind), completeChainFixed(kind))
		setArgCompletion(nounSubcommand(byName["unset"], kind), completeUnsetField(kind))
	}

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
	// <value>`, `unset <key>`, `show <key> [<noun.chain>]`, `list
	// [--prefix …]` — the one shape with no address argument at all (R12).
	for _, sub := range byName["config"].Commands() {
		switch sub.Name() {
		case "set":
			setArgCompletion(sub, positional(completeConfigKey, completeConfigValue))
		case "unset":
			setArgCompletion(sub, positional(completeConfigKey))
		case "show":
			setArgCompletion(sub, positional(completeConfigKey, completeDottedAddress))
		case "list":
			setArgCompletion(sub, nothing)
		}
		// --at names the level to write at, a dotted address wherever it
		// appears under config (R24, task P21.7). It shares its name with
		// `note`/`measure`'s instant, which is why it is registered here, per
		// command, rather than by name across the tree.
		if sub.Flags().Lookup("at") != nil {
			_ = sub.RegisterFlagCompletionFunc("at", completeDottedAddress)
		}
	}

	registerFlagCompletions(root)
	denyFileCompletion(root)
}

// nounSubcommand finds kind's own subcommand of a noun-dispatching parent
// (add, set, unset — see newNounDispatchCmd), or nil.
func nounSubcommand(parent *cobra.Command, kind kindmeta.Kind) *cobra.Command {
	for _, sub := range parent.Commands() {
		if sub.Name() == kind.String() {
			return sub
		}
	}
	return nil
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

// registerFlagCompletions attaches the vocabularies for the flags that mean
// the same thing wherever they appear, keyed by name across the whole tree.
//
// The §15 field flags are *not* here, and that is the point of the split. A
// flag name does not fix a meaning: `--status` and `--priority` appear both as
// a value to store (`add`/`set`, §15) and as a filter (`list`/`review`, §17),
// and the two want different sets. Those are registered by the two functions
// that already know which role they are declaring — fieldFlags.registerForKind
// and filterFlags.register — so the question "which meaning is this?" is
// answered where the flag is defined rather than guessed from its name here.
func registerFlagCompletions(root *cobra.Command) {
	byFlag := map[string]completer{
		"sort": fixed(sortKeyNames()),
		"kind": fixed(eventKindNames()),
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
// Para takes addresses, dates, numbers, and tags — not paths (§13's only path
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
