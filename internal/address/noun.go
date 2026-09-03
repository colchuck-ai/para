// Package address implements the external address form (requirements.md
// R1-R29): the (noun, id-chain) pair a user types and the tool prints and
// serializes, and its conversion to and from locator.Locator, the internal
// representation the walk, tree, mutate, query, review, and doctor's subject
// discovery reason about instead (plan §0.1).
package address

import (
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Noun is the word a user types for an address's kind (R2). It is
// kindmeta.Kind itself — not a second enum declared beside it — so a kind
// kindmeta grows can never leave a corresponding noun out of AllNouns.
type Noun = kindmeta.Kind

// The nouns (R2), aliasing kindmeta's constants under the singular names R2
// gives them.
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

// AllNouns returns R2's words in their fixed order, which is
// kindmeta.AllStorableKinds() — the addressable kinds, then Container.
//
// It is that list rather than a list built here, because the words a user may
// type and the words a state.toml's `kind` may hold are the same vocabulary
// (§8.3, §30), and building it twice would make it two facts. Compare
// kindmeta.AllKinds(), which excludes Container because no locator ever names
// one: a caller enumerating what can be addressed wants that one.
func AllNouns() []Noun {
	return kindmeta.AllStorableKinds()
}

// ParseNoun parses s as one of R2's words. It is distinct from the
// plural bucket words locator.ReservedWords carries — "projects" is not a
// noun, "project" is — because the noun is singular by design (R2's decision
// log: the stored form is "the CLI form with the first space as a dot", and
// the CLI form's first token is always singular.
// It delegates the match to kindmeta.ParseKind, the one place a kind word
// becomes a Kind (§30), and replaces only the wording: the caller typed a noun
// and a message about kinds would name a concept the command line does not
// have.
func ParseNoun(s string) (Noun, error) {
	n, err := kindmeta.ParseKind(s)
	if err != nil {
		return kindmeta.KindUnknown, paraerr.Newf(paraerr.KindValidation, "%q is not a noun (want one of: %s)", s, kindmeta.KindWordList())
	}
	return n, nil
}
