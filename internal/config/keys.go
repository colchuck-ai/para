// Package config implements design §7: a config.toml in any .para/, and
// resolution that walks the full ancestor chain with nearest winning.
//
// The chain is a first-class value here rather than a side effect of a
// lookup, because §7 makes printability the condition of chained resolution
// being defensible at all: "the ability to print the chain and the winning
// value is not optional". So Resolve returns every level it consulted, in
// order, with the winner marked — which is what `config show` prints (§22)
// and what lets `show` name where a threshold came from (§16.1).
//
// The set of keys is closed. A key is a knob some code reads, so a key
// nothing reads is a key that will be wrong (§20's argument for --skills),
// and the inverse holds too: a `config set projet.stale-after 30` that
// silently writes a typo is a knob the user believes in and para never
// consults. Unknown keys already in a file are a different case — they are
// preserved on rewrite (see File) and are doctor's `invalid` finding to
// report (§10), not this package's to delete.
package config

import (
	"math"
	"strconv"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
)

// The keys code looks up by name. A key read from a literal in two packages is
// a key one of them will eventually misspell, and §7's whole argument for a
// closed key set is that a knob nothing consults is a knob that will be wrong.
// The `<kind>.stale-after` family has no constant because its name is built
// from the kind — StaleKey is where that lives.
const (
	KeyAtRiskPace    = "key-result.at-risk-pace"
	KeyReviewCadence = "review.cadence"
	// The two keys that decide the Claude Code surface's shape, and turn on
	// together because it is one concern (§6.1). Named because a mutation has
	// to recognise a write to either of them and refresh the surface.
	KeyEmitClaude       = "emit.claude"
	KeyEmitClaudeSkills = "emit.claude-skills"
	// KeyEmitGitattributes decides whether §9's block sits in .gitattributes.
	// Named for the same reason: a `config set` of it has to put the block in
	// or take it out in the same command, or the command leaves a tree doctor
	// calls stale.
	KeyEmitGitattributes = "emit.gitattributes"
)

// AffectsClaudeSurface reports whether writing key changes what the Claude Code
// surface should contain — which is the question `config set` asks before
// deciding whether one file changed or eight did (§6.1).
func AffectsClaudeSurface(key string) bool {
	return key == KeyEmitClaude || key == KeyEmitClaudeSkills
}

// RootOnly reports whether key is read at the tree root whatever locator it is
// asked about — the three keys in §7's table whose "where it usefully lives"
// column says `root` and means it.
//
// It is a rule rather than advice for all three, and for one reason: each
// decides whether a file at a *fixed* location exists or what it holds. The two
// `emit.claude` keys govern eight CLAUDE.md files and one `.claude/skills/`,
// which sit at different depths, so a chained answer cannot be one answer (§6.1).
// `emit.gitattributes` governs a single file at the root (§9), so a value set
// anywhere else is read by nothing at all — a knob the user believes in and para
// never consults, which is precisely what this package's closed key set exists
// to prevent.
//
// Two things follow from it and neither may drift from the other: Resolver
// resolves these keys at the root whatever it is asked, and `config set --at`
// refuses them, naming the root.
func RootOnly(key string) bool {
	return AffectsClaudeSurface(key) || key == KeyEmitGitattributes
}

// Type is a config key's declared value type. It decides how a command-line
// string parses (Parse), what a stored value must be (Check), and how a
// resolved value prints (Format).
type Type int

const (
	// TypeInt is a non-negative whole number: a day count, a byte count.
	TypeInt Type = iota
	// TypeFloat is a ratio, such as a pace threshold.
	TypeFloat
	// TypeBool is a TOML boolean — true or false, and no other spelling.
	TypeBool
	// TypeEnum is a string drawn from a fixed set.
	TypeEnum
)

// Spec is one row of §7's config table: a key, the type of value it takes,
// and the default para uses when no level in the chain supplies one.
type Spec struct {
	// Key is the dotted key as it is written in a config.toml and typed on
	// the command line.
	Key string
	// Type is the value type.
	Type Type
	// Enum is the legal set for a TypeEnum key, in the order `config list`
	// and the error message list them.
	Enum []string
	// Doc is the one-line description `config list --json` and the help
	// text carry.
	Doc string

	// def is the built-in default, or the zero Value when the key has none.
	def ptoml.Value
	// hasDef distinguishes "defaults to false" from "has no default", which
	// a zero Value cannot.
	hasDef bool
}

// DefaultValue returns the key's built-in default and whether it has one.
//
// Only §7's four emit/log knobs do. The three thresholds deliberately do not:
// §7 says "unset everywhere means the check never fires", so a default would
// make review fire on a tree that never asked it to.
func (s Spec) DefaultValue() (ptoml.Value, bool) {
	return s.def, s.hasDef
}

// staleAfterKinds are the kinds whose staleness is measured by
// <kind>.stale-after (§7's `<kind>.stale-after` row). A skill is measured by
// review.cadence instead and a container by nothing at all — see StaleKey.
var staleAfterKinds = []kindmeta.Kind{
	kindmeta.KindArea,
	kindmeta.KindKeyResult,
	kindmeta.KindObjective,
	kindmeta.KindProject,
	kindmeta.KindResource,
}

// specs is every key para recognises, in the lexical order `config list`
// prints and File writes. One order, declared once: §0.2 forbids map
// iteration in output, and a per-family order would be a second thing to
// keep in step with the table.
var specs = buildSpecs()

func buildSpecs() []Spec {
	out := []Spec{
		{
			Key: KeyEmitClaude, Type: TypeBool,
			def: ptoml.Bool(false), hasDef: true,
			Doc: "emit the Claude Code compatibility surface: a block in CLAUDE.md and the .claude/skills mirror",
		},
		{
			Key: KeyEmitClaudeSkills, Type: TypeEnum, Enum: []string{"symlink", "copy"},
			def: ptoml.String("symlink"), hasDef: true,
			Doc: "how skills are mirrored into .claude/skills when emit.claude is on",
		},
		{
			Key: KeyEmitGitattributes, Type: TypeBool,
			def: ptoml.Bool(true), hasDef: true,
			Doc: "write the merge-attribute block in .gitattributes",
		},
		{
			Key: KeyAtRiskPace, Type: TypeFloat,
			Doc: "the pace below which a key-result reads at-risk",
		},
		{
			Key: "log.rotate-bytes", Type: TypeInt,
			def: ptoml.Int64(4 << 20), hasDef: true,
			Doc: "the size a journal file may exceed before the next event opens a new one",
		},
		{
			Key: KeyReviewCadence, Type: TypeInt,
			Doc: "days a skill may go untouched before review --skills lists it",
		},
	}
	for _, k := range staleAfterKinds {
		out = append(out, Spec{
			Key: k.String() + ".stale-after", Type: TypeInt,
			Doc: "days since attention before a " + k.String() + " reads stale",
		})
	}
	sortSpecs(out)
	return out
}

func sortSpecs(s []Spec) {
	// A hand-rolled insertion sort rather than slices.SortFunc so the
	// declared order is fixed at init and never depends on a comparator
	// somebody changes later. The list is eleven long.
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Key < s[j-1].Key; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// Specs returns every key para recognises, in lexical order.
func Specs() []Spec {
	out := make([]Spec, len(specs))
	copy(out, specs)
	return out
}

// Lookup returns the spec for key, or false if para does not recognise it.
func Lookup(key string) (Spec, bool) {
	for _, s := range specs {
		if s.Key == key {
			return s, true
		}
	}
	return Spec{}, false
}

// StaleKey is the config key that decides when a thing of this kind reads
// stale, and whether it has one at all.
//
// §20 is explicit that these are two different knobs: a skill has no status
// and no due date, so it can only ever be stale, and its threshold is
// review.cadence — "putting it in --stale would mean one group reading two
// different knobs". A container has neither, since it never appears in any
// review group.
func StaleKey(k kindmeta.Kind) (string, bool) {
	if k == kindmeta.KindSkill {
		return KeyReviewCadence, true
	}
	for _, kind := range staleAfterKinds {
		if kind == k {
			return k.String() + ".stale-after", true
		}
	}
	return "", false
}

// Parse converts a command-line string into a value of the key's declared
// type, rejecting anything the type does not admit.
func (s Spec) Parse(raw string) (ptoml.Value, error) {
	switch s.Type {
	case TypeInt:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes a whole number, not %q", s.Key, raw)
		}
		if n < 0 {
			return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes a non-negative number, not %q", s.Key, raw)
		}
		return ptoml.Int64(n), nil
	case TypeFloat:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes a number, not %q", s.Key, raw)
		}
		// ParseFloat admits nan, inf and -inf, and TOML can spell all
		// three — but a threshold that is not a number is a comparison
		// that can never be true, which is a check silently switched off
		// rather than a value.
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes a finite number, not %q", s.Key, raw)
		}
		return ptoml.Float64(f), nil
	case TypeBool:
		// TOML's two spellings and no others. Accepting "yes" or "1" would
		// mean the file and the command line disagreed about what a boolean
		// looks like, and the file is the one a human edits by hand.
		switch raw {
		case "true":
			return ptoml.Bool(true), nil
		case "false":
			return ptoml.Bool(false), nil
		}
		return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes true or false, not %q", s.Key, raw)
	case TypeEnum:
		for _, want := range s.Enum {
			if raw == want {
				return ptoml.String(raw), nil
			}
		}
		return ptoml.Value{}, paraerr.Newf(paraerr.KindValidation, "%s takes one of %s, not %q", s.Key, strings.Join(s.Enum, ", "), raw)
	default:
		return ptoml.Value{}, paraerr.Newf(paraerr.KindInternal, "config: key %q has an unknown type", s.Key)
	}
}

// Check validates a value read back from a file, which — unlike one Parse
// produced — may have been hand-edited to the wrong TOML type. A mistyped
// knob is refused loudly rather than read as if it were right, because the
// alternative is a threshold that silently never fires.
func (s Spec) Check(v ptoml.Value) error {
	switch s.Type {
	case TypeInt:
		if v.Kind != ptoml.KindInt64 {
			return s.typeErr(v, "a whole number")
		}
		if v.Int < 0 {
			return paraerr.Newf(paraerr.KindValidation, "%s must be non-negative, but is %d", s.Key, v.Int)
		}
	case TypeFloat:
		// TOML reads `at-risk-pace = 1` as an integer, and a file that
		// spells a whole ratio without a decimal point is not wrong enough
		// to refuse — so an integer widens here, and only here.
		if v.Kind != ptoml.KindFloat64 && v.Kind != ptoml.KindInt64 {
			return s.typeErr(v, "a number")
		}
		if f, _ := Float(v); math.IsNaN(f) || math.IsInf(f, 0) {
			return s.typeErr(v, "a finite number")
		}
	case TypeBool:
		if v.Kind != ptoml.KindBool {
			return s.typeErr(v, "true or false")
		}
	case TypeEnum:
		if v.Kind != ptoml.KindString {
			return s.typeErr(v, "one of "+strings.Join(s.Enum, ", "))
		}
		found := false
		for _, want := range s.Enum {
			if v.Str == want {
				found = true
			}
		}
		if !found {
			return paraerr.Newf(paraerr.KindValidation, "%s must be one of %s, but is %q", s.Key, strings.Join(s.Enum, ", "), v.Str)
		}
	}
	return nil
}

func (s Spec) typeErr(v ptoml.Value, want string) error {
	return paraerr.Newf(paraerr.KindValidation, "%s must be %s, but is %s", s.Key, want, Format(v))
}

// Float returns v as a float64, widening an integer, so a caller reading a
// TypeFloat key does not have to know which of the two spellings the file
// used. It reports false for any other kind.
func Float(v ptoml.Value) (float64, bool) {
	switch v.Kind {
	case ptoml.KindFloat64:
		return v.Float, true
	case ptoml.KindInt64:
		return float64(v.Int), true
	default:
		return 0, false
	}
}

// Format renders v in the unquoted display form every output shape uses —
// §22's chain, §16.1's provenance note, and the terminal generally. It is
// deliberately not TOML: a shell reading `$(para config show …)` wants
// symlink, not "symlink".
func Format(v ptoml.Value) string {
	switch v.Kind {
	case ptoml.KindString:
		return v.Str
	case ptoml.KindInt64:
		return strconv.FormatInt(v.Int, 10)
	case ptoml.KindFloat64:
		// The same spelling the file uses, so a value printed here can be
		// typed straight back into `config set`.
		return ptoml.FormatFloat(v.Float)
	case ptoml.KindBool:
		return strconv.FormatBool(v.Bool)
	case ptoml.KindStringArray:
		return "[" + strings.Join(v.StrArray, ", ") + "]"
	default:
		return ""
	}
}
