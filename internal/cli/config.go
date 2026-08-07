package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/tree"
)

// unset is how an absent value prints in every config output shape: the
// em dash §22's chain uses for a level that supplies nothing.
const unset = "—"

// defaultLabel names the built-in default as a level of the chain, so that
// the arrow marking the winner is never missing.
const defaultLabel = "(default)"

// newConfigCmd implements `para config` (§22): the command that makes §7's
// chain printable, which §7 calls the condition of chained resolution being
// defensible at all.
func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "read and write the tree's configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newConfigSetCmd(), newConfigUnsetCmd(), newConfigListCmd(), newConfigShowCmd())
	return cmd
}

// scope is the tree and the level a config command operates on.
type scope struct {
	root     string
	locator  locator.Locator
	resolver *config.Resolver
	env      *mutate.Env
}

// openScope discovers the tree root and resolves the locator naming a level
// — "" for the root, which is where `set` and `unset` write unless --at says
// otherwise (§22).
//
// A named level must exist. A `config set --at projects.acme` against a
// project that is not there would otherwise write a config.toml into a
// directory nothing reads, which is exactly the silent misconfiguration the
// closed key set exists to prevent.
func openScope(cmd *cobra.Command, at string) (scope, error) {
	env, cwd, err := openEnv(cmd)
	if err != nil {
		return scope{}, err
	}
	root := env.Root
	s := scope{root: root, resolver: env.Resolver, env: env}
	if at == "" {
		return s, nil
	}

	loc, err := resolveLocatorArg(root, cwd, at)
	if err != nil {
		return scope{}, err
	}
	exists, err := tree.Exists(root, loc)
	if err != nil {
		return scope{}, err
	}
	if !exists {
		return scope{}, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", loc.String())
	}
	s.locator = loc
	return s, nil
}

// file is the scope's own config.toml: the root-relative path for output,
// and the OS path to write.
func (s scope) file() (rel, abs string, err error) {
	levels, err := config.Chain(s.locator)
	if err != nil {
		return "", "", err
	}
	rel = levels[0].File
	return rel, filepath.Join(s.root, filepath.FromSlash(rel)), nil
}

func newConfigSetCmd() *cobra.Command {
	var at string
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "set a config key, at the root by default",
		Long: "Set a config key.\n\n" +
			"Writes the root's config.toml unless --at names a level, because that is\n" +
			"where a knob usually belongs, and --at is how the resolution chain gets\n" +
			"built deliberately rather than by accident.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, raw := args[0], args[1]
			spec, ok := config.Lookup(key)
			if !ok {
				return unknownKey(key)
			}
			value, err := spec.Parse(raw)
			if err != nil {
				return err
			}
			s, err := openScope(cmd, at)
			if err != nil {
				return err
			}
			return writeLevel(cmd.OutOrStdout(), s, key, func(f *config.File) (bool, error) {
				return f.Set(key, value)
			})
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "the locator whose config.toml to write (default: the tree root)")
	return cmd
}

func newConfigUnsetCmd() *cobra.Command {
	var at string
	cmd := &cobra.Command{
		Use:   "unset <key>",
		Short: "remove a config key, at the root by default",
		Long: "Remove a config key at one level.\n\n" +
			"Resolution then continues up the chain, so unsetting a value uncovers\n" +
			"whatever an ancestor sets. Unset everywhere means the check never fires.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			if _, ok := config.Lookup(key); !ok {
				return unknownKey(key)
			}
			s, err := openScope(cmd, at)
			if err != nil {
				return err
			}
			return writeLevel(cmd.OutOrStdout(), s, key, func(f *config.File) (bool, error) {
				return f.Unset(key), nil
			})
		},
	}
	cmd.Flags().StringVar(&at, "at", "", "the locator whose config.toml to write (default: the tree root)")
	return cmd
}

// writeLevel applies edit to the scope's own config.toml and reports what it
// wrote.
//
// A change that changes nothing writes nothing and says so (§23): a config file
// rewritten with identical bytes would still churn its mtime, and every re-run
// of a provisioning script would look like a change.
//
// A change that does land is recorded in the level's own journal (§8.1), which
// is why this goes through mutate rather than writing the file directly: the
// config.toml, the event, and that level's ACTIVITY.md are one mutation, and a
// crash between them would leave the file changed with nothing to say so.
func writeLevel(out io.Writer, s scope, key string, edit func(*config.File) (bool, error)) error {
	_, abs, err := s.file()
	if err != nil {
		return err
	}
	f, err := config.Read(abs)
	if err != nil {
		return err
	}
	before, _ := f.Get(key)
	changed, err := edit(&f)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(out, "no change")
		return nil
	}
	data, err := f.Encode()
	if err != nil {
		return err
	}
	after, _ := f.Get(key)

	res, err := s.env.ConfigChange(s.locator, key, config.Format(before), config.Format(after), data)
	if err != nil {
		return err
	}
	// Not just the file list: setting either `emit.claude` key writes or sweeps
	// the whole Claude surface in the same command (§6.1, §26).
	printEffects(out, res)
	return nil
}

func newConfigShowCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <key> [<locator>]",
		Short: "print a resolved config value and the chain that produced it",
		Long: "Print a resolved config value and the chain that produced it.\n\n" +
			"Every level consulted is listed, nearest first, with an arrow on the one\n" +
			"that won — so a value that surprises you can be traced to the file that\n" +
			"set it.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			at := ""
			if len(args) == 2 {
				at = args[1]
			}
			// The key is checked before the tree is opened, so a typo
			// reports the typo rather than whatever the tree says.
			if _, ok := config.Lookup(key); !ok {
				return unknownKey(key)
			}
			s, err := openScope(cmd, at)
			if err != nil {
				return err
			}
			res, err := s.resolver.Resolve(s.locator, key)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), showJSON(s, res))
			}
			fmt.Fprint(cmd.OutOrStdout(), formatChain(res))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the value and its chain as JSON")
	return cmd
}

func newConfigListCmd() *cobra.Command {
	var (
		asJSON bool
		prefix string
	)
	cmd := &cobra.Command{
		Use:   "list [<locator>]",
		Short: "list every config key as resolved at a locator",
		Long: "List every config key para recognises, resolved at a locator (the tree\n" +
			"root by default), with the level each value came from.\n\n" +
			"Keys nothing sets are listed too: a knob nobody can find is a knob that\n" +
			"will be wrong.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			at := ""
			if len(args) == 1 {
				at = args[0]
			}
			s, err := openScope(cmd, at)
			if err != nil {
				return err
			}

			var rows []config.Resolution
			for _, spec := range config.Specs() {
				if !strings.HasPrefix(spec.Key, prefix) {
					continue
				}
				res, err := s.resolver.Resolve(s.locator, spec.Key)
				if err != nil {
					return err
				}
				rows = append(rows, res)
			}

			if asJSON {
				return writeJSON(cmd.OutOrStdout(), listJSON(s, rows))
			}
			fmt.Fprint(cmd.OutOrStdout(), formatList(rows))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the resolved keys as JSON")
	cmd.Flags().StringVar(&prefix, "prefix", "", "list only keys starting with this text")
	return cmd
}

func unknownKey(key string) error {
	return paraerr.Newf(paraerr.KindValidation, "unknown config key %q (run `para config list` for the keys para reads)", key)
}

// formatChain renders §22's output: the resolved value, a blank line, then
// every level consulted, nearest first, with the winner marked.
//
//	30
//
//	  projects.acme-migration      —
//	→ projects                     30
//	  <root>                       14
func formatChain(res config.Resolution) string {
	type row struct {
		label  string
		value  string
		winner bool
	}
	rows := make([]row, 0, len(res.Levels)+1)
	for i, l := range res.Levels {
		r := row{label: l.Label(), value: unset, winner: i == res.Winner}
		if l.Set {
			r.value = config.Format(l.Value)
		}
		rows = append(rows, r)
	}
	if res.FromDefault {
		rows = append(rows, row{label: defaultLabel, value: config.Format(res.Value), winner: true})
	}

	// §22's example puts the value column six spaces past the longest
	// label, itself indented by the two-column winner marker.
	const marker, gap = 2, 6
	width := 0
	for _, r := range rows {
		width = max(width, len([]rune(r.label)))
	}

	var b strings.Builder
	value := unset
	if res.Found {
		value = config.Format(res.Value)
	}
	b.WriteString(value)
	b.WriteString("\n\n")
	for _, r := range rows {
		if r.winner {
			b.WriteString("→ ")
		} else {
			b.WriteString(strings.Repeat(" ", marker))
		}
		b.WriteString(r.label)
		b.WriteString(strings.Repeat(" ", width-len([]rune(r.label))+gap))
		b.WriteString(r.value)
		b.WriteString("\n")
	}
	return b.String()
}

// formatList renders one line per key: the key, its resolved value, and the
// level that supplied it.
func formatList(rows []config.Resolution) string {
	const gap = 3
	keyWidth, valueWidth := 0, 0
	for _, res := range rows {
		keyWidth = max(keyWidth, len(res.Key))
		valueWidth = max(valueWidth, len([]rune(resolvedValue(res))))
	}

	var b strings.Builder
	for _, res := range rows {
		value := resolvedValue(res)
		b.WriteString(res.Key)
		b.WriteString(strings.Repeat(" ", keyWidth-len(res.Key)+gap))
		b.WriteString(value)
		b.WriteString(strings.Repeat(" ", valueWidth-len([]rune(value))+gap))
		b.WriteString(resolvedSource(res))
		b.WriteString("\n")
	}
	return b.String()
}

func resolvedValue(res config.Resolution) string {
	if !res.Found {
		return unset
	}
	return config.Format(res.Value)
}

// resolvedSource names where a value came from: the level that supplied it,
// the built-in default, or nothing at all.
func resolvedSource(res config.Resolution) string {
	if src, ok := res.Source(); ok {
		return src.Label()
	}
	if res.FromDefault {
		return defaultLabel
	}
	return unset
}

// levelJSON is one link of the chain in --json form. Level is the locator
// itself — empty for the root — rather than the <root> label, because JSON
// is read by programs and a program wants the locator.
type levelJSON struct {
	Level  string `json:"level"`
	File   string `json:"file"`
	Set    bool   `json:"set"`
	Value  any    `json:"value"`
	Winner bool   `json:"winner"`
}

// showPayload is `config show --json`. Set and Default answer different
// questions: Set is whether a level in the chain supplied the value, Default
// whether para's built-in one did. Both false with a non-null Value is
// impossible; both false with a null Value is §7's "unset everywhere".
type showPayload struct {
	Key     string      `json:"key"`
	Locator string      `json:"locator"`
	Value   any         `json:"value"`
	Set     bool        `json:"set"`
	Default bool        `json:"default"`
	Source  *string     `json:"source"`
	Chain   []levelJSON `json:"chain"`
}

func showJSON(s scope, res config.Resolution) showPayload {
	out := showPayload{
		Key:     res.Key,
		Locator: s.locator.String(),
		Set:     res.Found && !res.FromDefault,
		Default: res.FromDefault,
		Source:  sourceJSON(res),
		Chain:   make([]levelJSON, 0, len(res.Levels)),
	}
	if res.Found {
		out.Value = jsonValue(res.Value)
	}
	for i, l := range res.Levels {
		link := levelJSON{Level: l.Locator.String(), File: l.File, Set: l.Set, Winner: i == res.Winner}
		if l.Set {
			link.Value = jsonValue(l.Value)
		}
		out.Chain = append(out.Chain, link)
	}
	return out
}

type keyPayload struct {
	Key     string  `json:"key"`
	Value   any     `json:"value"`
	Set     bool    `json:"set"`
	Default bool    `json:"default"`
	Source  *string `json:"source"`
	Doc     string  `json:"doc"`
}

type listPayload struct {
	Locator string       `json:"locator"`
	Keys    []keyPayload `json:"keys"`
}

func listJSON(s scope, rows []config.Resolution) listPayload {
	out := listPayload{Locator: s.locator.String(), Keys: make([]keyPayload, 0, len(rows))}
	for _, res := range rows {
		k := keyPayload{
			Key:     res.Key,
			Set:     res.Found && !res.FromDefault,
			Default: res.FromDefault,
			Source:  sourceJSON(res),
			Doc:     res.Spec.Doc,
		}
		if res.Found {
			k.Value = jsonValue(res.Value)
		}
		out.Keys = append(out.Keys, k)
	}
	return out
}

// sourceJSON names the level that supplied the value, or null when the
// built-in default did or nothing did. The root is the empty string, the
// same spelling the chain uses.
func sourceJSON(res config.Resolution) *string {
	src, ok := res.Source()
	if !ok {
		return nil
	}
	s := src.Locator.String()
	return &s
}

// jsonValue converts a stored value to its natural JSON type, so a number
// stays a number rather than becoming the string a terminal would print.
func jsonValue(v ptoml.Value) any {
	switch v.Kind {
	case ptoml.KindString:
		return v.Str
	case ptoml.KindInt64:
		return v.Int
	case ptoml.KindFloat64:
		return v.Float
	case ptoml.KindBool:
		return v.Bool
	case ptoml.KindStringArray:
		return v.StrArray
	default:
		return nil
	}
}

func writeJSON(out io.Writer, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return paraerr.Wrap(paraerr.KindInternal, err, "encoding JSON output")
	}
	_, err = fmt.Fprintf(out, "%s\n", data)
	return err
}
