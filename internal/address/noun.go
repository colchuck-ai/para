// Package address implements the external address form (requirements.md
// R1-R29): the (noun, id-chain) pair a user types and the tool prints and
// serializes, and its conversion to and from locator.Locator, the internal
// representation the walk, tree, mutate, query, review, and doctor's subject
// discovery reason about instead (plan §0.1).
package address

import (
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Noun is the word a user types for an address's kind (R2). It is
// kindmeta.Kind itself — not a second enum declared beside it — so a kind
// kindmeta grows can never leave a corresponding noun out of AllNouns.
type Noun = kindmeta.Kind

// The seven nouns (R2), aliasing kindmeta's constants under the singular
// names R2 gives them.
const (
	Project   = kindmeta.KindProject
	Area      = kindmeta.KindArea
	Resource  = kindmeta.KindResource
	Objective = kindmeta.KindObjective
	KeyResult = kindmeta.KindKeyResult
	Link      = kindmeta.KindLink
	Skill     = kindmeta.KindSkill
	Container = kindmeta.KindContainer
)

// allNouns is R2's seven words, built from kindmeta.AllKinds() rather than
// listed again by hand, so a kind kindmeta's own list grows to include is
// never silently missing from the noun vocabulary. Container is the one
// noun kindmeta.AllKinds() does not carry — it is deliberately excluded
// there because no locator ever names one (kindmeta.go's own comment) — so
// it is appended once, explicitly, rather than folded into the derivation.
var allNouns = append(kindmeta.AllKinds(), Container)

// AllNouns returns the seven nouns in R2's fixed order: kindmeta.AllKinds()'s
// six addressable kinds, then Container. Compare kindmeta.AllKinds(), which
// excludes Container because no locator ever names one on its own — a
// caller building "every kind" for a walk-adjacent purpose wants that list,
// not this one.
func AllNouns() []Noun {
	return slices.Clone(allNouns)
}

// ParseNoun parses s as one of R2's seven words. It is distinct from the
// plural bucket words locator.ReservedWords carries — "projects" is not a
// noun, "project" is — because the noun is singular by design (R2's decision
// log: the stored form is "the CLI form with the first space as a dot", and
// the CLI form's first token is always singular.
func ParseNoun(s string) (Noun, error) {
	for _, n := range allNouns {
		if n.String() == s {
			return n, nil
		}
	}
	return kindmeta.KindUnknown, paraerr.Newf(paraerr.KindValidation, "%q is not a noun (want one of: %s)", s, nounList())
}

func nounList() string {
	words := make([]string, len(allNouns))
	for i, n := range allNouns {
		words[i] = n.String()
	}
	return strings.Join(words, ", ")
}
