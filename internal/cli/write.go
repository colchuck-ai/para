package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

// openEnv discovers the tree and builds the environment one mutating command
// runs in, plus the working directory "." resolves against (§14).
func openEnv(cmd *cobra.Command) (*mutate.Env, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", paraerr.Wrap(paraerr.KindInternal, err, "determining working directory")
	}
	root, err := tree.Find(cwd)
	if err != nil {
		return nil, "", err
	}
	return mutate.NewEnv(root, clock.FromContext(cmd.Context())), cwd, nil
}

// fieldFlags is the flag set §15's field matrix produces, built from the matrix
// rather than beside it: §0.3 says outright that "add's legal flags, set/unset
// validation, --priority applicability, and --sort key validation all fall out
// as one lookup", so a new row in §15 must not need a second edit here.
//
// Every flag is a string, including the two list-valued fields, because pflag's
// StringSlice would accept `--tags a --tags b` as accumulation while §15 says
// both list fields replace wholly.
type fieldFlags struct {
	values map[kindmeta.Field]*string
}

// help is the one-line description each field's flag carries. A field with no
// entry would be a field nobody could be told about, so the map is keyed by the
// same §15 row order register walks.
var fieldHelp = map[kindmeta.Field]string{
	kindmeta.FieldName:        "the thing's name",
	kindmeta.FieldDescription: "one line: what it is, or when to use it",
	// The vocabulary is a kind's, not the tree's (§1.7): a key-result's status
	// is derived, with exactly one value carved out as settable. Naming that
	// exception here is cheaper than a reader discovering it as a refusal.
	// This entry is only reached by the unconditional `register`, used by
	// `set`/`unset` (P19.5 has not run yet) — `registerForKind` builds its
	// own status text instead, via statusHelpForKind, since it already knows
	// which kind it is and does not need the footnote at all (task P19.4).
	kindmeta.FieldStatus: "one of " + strings.Join(kindmeta.AllStatuses(), ", ") +
		"; a key-result takes only " + strings.Join(kindmeta.SettableStatuses(kindmeta.KindKeyResult), ", "),
	kindmeta.FieldPriority: "one of " + strings.Join(kindmeta.Priorities(), ", "),
	kindmeta.FieldDue:      "a deadline, in progressive precision",
	kindmeta.FieldTags:     "a comma-separated list, replacing whatever is there",
	kindmeta.FieldCreated:  "the creation time; defaults to now, never in the future",
	// type, start, target, and scope each belong to exactly one kind, so
	// naming it here was never disambiguating anything — it only read as an
	// apology for the flag being offered everywhere. The one subcommand that
	// registers each of these already says which kind it is.
	kindmeta.FieldType:   "the measurement grammar: " + strings.Join(krvalue.TypeNames(), ", "),
	kindmeta.FieldStart:  "the baseline; defaults to the first measurement",
	kindmeta.FieldTarget: "the target",
	kindmeta.FieldScope:  "a comma-separated locator list, or omit for the whole tree",
}

// statusHelpForKind is registerForKind's status text: the settable
// vocabulary kind actually has, with no footnote about any other kind's
// restriction, because the subcommand that shows this text simply does not
// register the values it cannot take.
func statusHelpForKind(kind kindmeta.Kind) string {
	return "one of " + strings.Join(kindmeta.SettableStatuses(kind), ", ")
}

func (f *fieldFlags) register(cmd *cobra.Command) {
	f.values = map[kindmeta.Field]*string{}
	// AllFields, not the map: §15's row order is declared once and the map is
	// only ever asked for one key's help text (§0.2 — no map iteration reaches
	// output, and flag registration order reaches --help).
	for _, field := range kindmeta.AllFields() {
		var v string
		f.values[field] = &v
		cmd.Flags().StringVar(&v, string(field), "", fieldHelp[field])
		// Registered here, where the role is known: these are values being
		// stored, so the kind the locator names decides the vocabulary and
		// whether the flag applies at all. `add` is the creation, so it alone
		// may set a field that is fixed at creation. The error is the "already
		// registered" one, which cannot happen on a freshly built command.
		_ = cmd.RegisterFlagCompletionFunc(string(field),
			completeFieldValue(field, cmd.Name() == "add"))
	}
}

// registerForKind is add's own registration (R3, task P19.3): only the
// fields kindmeta.Has(kind, field) gives kind, in AllFields order — so a
// project's --help shows seven flags and a key-result's nine, instead of
// every command offering all eleven and refusing the wrong ones at
// mutate-time. atCreation is always true here: add is the one command that
// may set a RequiredFixed field at all.
func (f *fieldFlags) registerForKind(cmd *cobra.Command, kind kindmeta.Kind) {
	f.values = map[kindmeta.Field]*string{}
	for _, field := range kindmeta.AllFields() {
		if !kindmeta.Has(kind, field) {
			continue
		}
		help := fieldHelp[field]
		if field == kindmeta.FieldStatus {
			help = statusHelpForKind(kind)
		}
		var v string
		f.values[field] = &v
		cmd.Flags().StringVar(&v, string(field), "", help)
		_ = cmd.RegisterFlagCompletionFunc(string(field), completeFieldValue(field, true))
	}
}

// collect turns the flags actually given into a Fields. A flag left alone is
// absent rather than empty, which is the distinction §15's no-op rule and
// `unset` both rest on.
func (f *fieldFlags) collect(cmd *cobra.Command) mutate.Fields {
	var out mutate.Fields
	for _, field := range kindmeta.AllFields() {
		if !cmd.Flags().Changed(string(field)) {
			continue
		}
		value := *f.values[field]
		if mutate.IsListField(field) {
			out.SetList(field, strings.Split(value, ","))
			continue
		}
		out.Set(field, value)
	}
	return out
}

// newAddCmd implements `para add <noun> <chain>` (R3, R15): one subcommand
// per addressable noun, dispatching on the noun the way §25 amended R2's own
// argument turns "which fields, which help, which refusals" into a lookup
// instead of a mutate-time surprise. `container` is not among them — R15
// refuses it because a container is created eagerly by its parent (§18.1)
// and never directly.
func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <noun> <chain> --name … [--field …]",
		Short: "create an entity",
		Long: "Create the entity a noun and a chain name — one subcommand per noun, so\n" +
			"`para add <noun> --help` shows exactly that kind's own fields.\n\n" +
			"A project is created with its objectives/ container and an objective\n" +
			"with its key-results/, because a uniform shape is what lets an agent or\n" +
			"a human look somewhere and trust that absence means none.\n\n" +
			"The new entity's journal starts empty — `created` is a field, not an\n" +
			"event — and the parent logs that its set of children changed.\n\n" +
			"Nouns: " + strings.Join(addableNounWords(), ", ") + ".",
		// Without a RunE, an unmatched first argument (a typo, or R15's
		// refused "container") falls through to cobra printing this
		// command's own help and exiting 0 — a silent no-op indistinguishable
		// from success. This RunE only ever runs for that fallthrough case:
		// every recognized noun is its own subcommand and cobra dispatches to
		// it directly, never reaching here.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if args[0] == kindmeta.KindContainer.String() {
				return paraerr.New(paraerr.KindValidation,
					"container is refused: a container is created eagerly by its parent and never directly")
			}
			return paraerr.Newf(paraerr.KindValidation,
				"%q is not a noun add can create (one of: %s)", args[0], strings.Join(addableNounWords(), ", "))
		},
	}
	for _, kind := range kindmeta.AllKinds() {
		cmd.AddCommand(newAddNounCmd(kind))
	}
	return cmd
}

// addableNounWords is R2's seven words minus container, the six add
// actually registers a subcommand for (R15) — used both for the noun list
// in its help and for naming what a bad noun should have been.
func addableNounWords() []string {
	kinds := kindmeta.AllKinds()
	words := make([]string, len(kinds))
	for i, k := range kinds {
		words[i] = k.String()
	}
	return words
}

// newAddNounCmd is one noun's `add` subcommand: its own chain arity (via
// chainToLocator, P19.1), its own field flags (via fieldFlags.registerForKind,
// this task), and its own --archived refusal (R8, carried over from the
// single command P19.2 registered it on).
func newAddNounCmd(kind kindmeta.Kind) *cobra.Command {
	var f fieldFlags
	var archived archivedFlag
	const archivedWhy = "nothing is created under archive/"

	cmd := &cobra.Command{
		Use:   kind.String() + " <chain> --name … [--field …]",
		Short: "create a " + kind.String(),
		Long: "Create the " + kind.String() + " the chain names.\n\n" +
			"\".\" is deliberately not accepted: it resolves to something that\n" +
			"already exists, and add is for something that does not.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := archived.check(archivedWhy); err != nil {
				return err
			}
			env, _, err := openEnv(cmd)
			if err != nil {
				return err
			}
			var chain string
			if len(args) == 1 {
				chain = args[0]
			}
			loc, err := chainToLocator(kind.String(), chain, false, false)
			if err != nil {
				return err
			}
			res, err := env.Add(loc, f.collect(cmd))
			if err != nil {
				return err
			}
			printResult(cmd.OutOrStdout(), []string{fmt.Sprintf("added  %s  %s", res.Locator, res.Kind)}, res)
			return nil
		},
	}
	f.registerForKind(cmd, kind)
	archived.registerRefused(cmd, archivedWhy)
	return cmd
}

func newSetCmd() *cobra.Command {
	var (
		f        fieldFlags
		archived archivedFlag
		note     string
	)
	cmd := &cobra.Command{
		Use:   "set <locator> [flags]",
		Short: "change stored fields",
		Long: "Change any number of stored fields at once.\n\n" +
			"One journal event per field that actually changed, and one\n" +
			"write-through pass at the end. Setting a field to the value it already\n" +
			"holds writes nothing and exits 0 — without that rule a loop would buy\n" +
			"permanent silence from every check in `para review`.",
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
			res, err := env.Set(loc, f.collect(cmd), note)
			if err != nil {
				return err
			}
			printResult(cmd.OutOrStdout(), summariseSet(res), res)
			return nil
		},
	}
	f.register(cmd)
	archived.register(cmd)
	cmd.Flags().StringVar(&note, "note", "", "the reason, recorded with the change; required when setting status to blocked")
	return cmd
}

func newUnsetCmd() *cobra.Command {
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "unset <locator> <field>...",
		Short: "remove stored fields",
		Long: "Remove stored fields, which is different from setting them to nothing:\n" +
			"`para unset skills.x scope` widens a skill back to the whole tree,\n" +
			"because absence already says everywhere.\n\n" +
			"`created` cannot be unset — everything has a creation time — and\n" +
			"neither can a field the kind requires or one fixed at creation.",
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			fields := make([]kindmeta.Field, 0, len(args)-1)
			for _, name := range args[1:] {
				fields = append(fields, kindmeta.Field(name))
			}
			res, err := env.Unset(loc, fields)
			if err != nil {
				return err
			}
			printResult(cmd.OutOrStdout(), summariseSet(res), res)
			return nil
		},
	}
	archived.register(cmd)
	return cmd
}

// summariseSet is the shape §26 shows for `set`: one line per change, or the
// no-change line when nothing moved.
func summariseSet(res mutate.Result) []string {
	if len(res.Changes) == 0 {
		return []string{noChangeLine(res)}
	}
	return changeLines(res)
}

func newNoteCmd() *cobra.Command {
	var at string
	var archived archivedFlag
	cmd := &cobra.Command{
		Use:   "note <locator> <text>",
		Short: "record a note against an entity",
		Long: "Record a note.\n\n" +
			"One of the two verbs that move the clock, which is why it is a verb of\n" +
			"its own rather than a field: `attention` is the newest note or\n" +
			"measurement, and every staleness check in `para review` reads it.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			res, err := env.Note(loc, args[1], at)
			if err != nil {
				return err
			}
			printResult(cmd.OutOrStdout(), []string{fmt.Sprintf("noted  %s", res.Locator)}, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "when it happened, in progressive precision; defaults to now")
	archived.register(cmd)
	return cmd
}

func newMeasureCmd() *cobra.Command {
	var at, note string
	cmd := &cobra.Command{
		Use:   "measure <kr-locator> <value>",
		Short: "log a reading against a key-result",
		Long: "Log a reading against a key-result.\n\n" +
			"The value follows the key-result's own type, and two readings may not\n" +
			"share an instant. Correcting a past reading is appending a corrected\n" +
			"one, or editing that line of the journal by hand and running\n" +
			"`para rebuild`.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, cwd, err := openEnv(cmd)
			if err != nil {
				return err
			}
			loc, err := resolveLocatorArg(env.Root, cwd, args[0])
			if err != nil {
				return err
			}
			res, err := env.Measure(loc, args[1], at, note)
			if err != nil {
				return err
			}
			printResult(cmd.OutOrStdout(), []string{measuredLine(res)}, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "when the reading was taken, in progressive precision; defaults to now")
	cmd.Flags().StringVar(&note, "note", "", "a reason or a caveat, recorded with the reading")
	return cmd
}

// measuredLine is §26's reply to a measurement:
//
//	measured signups = 880/11000   decimal 0.0800   progress 0.24   at-risk
//
// The id rather than the whole locator, because the command line already
// carries the locator and this line is the answer to it.
func measuredLine(res mutate.Result) string {
	id := res.Locator[len(res.Locator)-1]
	parts := []string{fmt.Sprintf("measured %s = %s", id, res.Measured.Value.Raw)}

	// A ratio's decimal is worth printing because the reading is not one; a
	// number already is its own decimal and a boolean has nothing to say (§4.1).
	if res.Measured.Value.Type == krvalue.TypeRatio {
		parts = append(parts, "decimal "+strconv.FormatFloat(res.Measured.Value.Decimal, 'f', 4, 64))
	}
	if res.Measured.HasProgress {
		parts = append(parts, "progress "+strconv.FormatFloat(res.Measured.Progress, 'f', 2, 64))
	}
	if res.Measured.Status != "" {
		parts = append(parts, string(res.Measured.Status))
	}
	return strings.Join(parts, "   ")
}
