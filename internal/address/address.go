package address

import (
	"fmt"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Address is the external address form (R1): the noun and id-chain a user
// types, and everything para prints or serializes. Archived is R7's locator
// qualifier — a place, not a kind — carried alongside rather than folded
// into Chain, so a bucket ("project", no chain) and its archived mirror
// ("archive.project") differ by exactly this one field.
//
// It is a pure value with no filesystem and no clock (plan §0). Converting
// it to and from locator.Locator, the internal representation, is
// ToLocator/FromLocator's job, not this type's.
type Address struct {
	Noun     Noun
	Chain    []string
	Archived bool
}

// Parse builds an Address from the CLI's two-token form: a bare noun word
// and a dot-joined id-chain. An empty chain is R17's bucket — the noun
// alone, naming everything of that kind.
func Parse(noun, chain string) (Address, error) {
	n, err := ParseNoun(noun)
	if err != nil {
		return Address{}, err
	}
	var segs []string
	if chain != "" {
		segs = strings.Split(chain, ".")
	}
	a := Address{Noun: n, Chain: segs}
	if err := a.Validate(); err != nil {
		return Address{}, err
	}
	return a, nil
}

// ParseDotted builds an Address from the one-token serialized form (R1):
// the CLI form with its first space turned into a dot, optionally prefixed
// with "archive." (R10) — the form used in TOML, frontmatter, journal
// events, and flag values.
func ParseDotted(s string) (Address, error) {
	if s == "" {
		return Address{}, paraerr.New(paraerr.KindValidation, "empty address")
	}
	segs := strings.Split(s, ".")

	archived := false
	if segs[0] == "archive" {
		archived = true
		segs = segs[1:]
		if len(segs) == 0 {
			return Address{}, paraerr.Newf(paraerr.KindValidation, "%q: archive alone is not an address", s)
		}
	}

	n, err := ParseNoun(segs[0])
	if err != nil {
		return Address{}, err
	}
	var chain []string
	if len(segs) > 1 {
		chain = segs[1:]
	}

	a := Address{Noun: n, Chain: chain, Archived: archived}
	if err := a.Validate(); err != nil {
		return Address{}, err
	}
	return a, nil
}

// String renders a in the one-token dotted form ParseDotted parses back:
// an optional "archive" segment, the noun, then the chain, joined with ".".
// It is exactly the CLI form (Parse's two tokens) with the first space
// turned into a dot (R1's decision log) — String does not validate a; it
// formats whatever fields it is given, valid or not, the way
// locator.Locator.String() does.
func (a Address) String() string {
	parts := make([]string, 0, len(a.Chain)+2)
	if a.Archived {
		parts = append(parts, "archive")
	}
	parts = append(parts, a.Noun.String())
	parts = append(parts, a.Chain...)
	return strings.Join(parts, ".")
}

// Validate reports whether a's chain has the arity R4 fixes for a's noun and
// every id segment in it is legal (§1.4): not one of locator.ReservedWords,
// and drawn from locator's segment charset. Parse and ParseDotted call it so
// neither entry point can produce an Address the other would refuse, and
// ToLocator calls it too, so an Address built by hand (e.g. a struct
// literal in a test) is held to the same rule a parsed one is.
//
// It produces the same two error kinds §10's findings use: KindConflict for
// a reserved word in an id position, KindValidation for a chain the noun
// cannot take. Every message names the noun and the arity it wants, per the
// task's acceptance criterion.
func (a Address) Validate() error {
	if a.Archived && a.Noun == Skill {
		return paraerr.New(paraerr.KindValidation, "skill cannot be archived — skills are not archivable (§1.6)")
	}

	switch a.Noun {
	case Project, Skill:
		return validateOneOrBucket(a.Noun, a.Chain)
	case Area, Resource:
		return validateOneOrMoreOrBucket(a.Chain)
	case Objective:
		return validateFixed(a.Noun, a.Chain, 2)
	case KeyResult:
		return validateFixed(a.Noun, a.Chain, 3)
	case Link:
		return validateLink(a.Chain)
	case Container:
		return validateContainer(a.Chain)
	default:
		return paraerr.Newf(paraerr.KindValidation, "%q is not a noun", a.Noun.String())
	}
}

// validateOneOrBucket is project and skill's arity (R4): no chain (the
// bucket, R17) or exactly one id.
func validateOneOrBucket(n Noun, chain []string) error {
	if len(chain) == 0 {
		return nil
	}
	if len(chain) != 1 {
		return arityErr(n, len(chain), "no chain (the bucket) or exactly one segment")
	}
	return checkIDs(chain)
}

// validateOneOrMoreOrBucket is area and resource's arity (R4): no chain
// (the bucket) or one or more ids, nesting to any depth. There is no upper
// bound to violate, so — unlike validateOneOrBucket and validateFixed —
// this can never itself produce an arity error; only checkIDs can refuse.
func validateOneOrMoreOrBucket(chain []string) error {
	if len(chain) == 0 {
		return nil
	}
	return checkIDs(chain)
}

// validateFixed is objective's and key-result's arity (R4): exactly want
// ids, no bucket form — an objective or key-result is never a bucket (R3
// lists no bucket path for either).
func validateFixed(n Noun, chain []string, want int) error {
	if len(chain) != want {
		return arityErr(n, len(chain), exactSegmentsPhrase(want))
	}
	return checkIDs(chain)
}

// validateContainer is container's arity (R4): two segments ending in
// "objectives" (under a project id), three ending in "key-results" (under
// an objective id), or — para-6g7 — a parent selector word (project/area/
// resource) followed by that noun's own id-chain and a trailing "links". The
// last two forms' final segment is the reserved structural word itself, not
// an id, so it is checked for equality rather than passed to checkIDs.
func validateContainer(chain []string) error {
	const want = `two segments ending in "objectives", three ending in "key-results", or a parent kind (project/area/resource) followed by its id chain and "links"`
	if len(chain) >= 1 && chain[len(chain)-1] == "links" {
		return validateLinkParent(Container, chain[:len(chain)-1])
	}
	switch len(chain) {
	case 2:
		if err := checkIDs(chain[:1]); err != nil {
			return err
		}
		if chain[1] != "objectives" {
			return arityErr(Container, len(chain), want)
		}
		return nil
	case 3:
		if err := checkIDs(chain[:2]); err != nil {
			return err
		}
		if chain[2] != "key-results" {
			return arityErr(Container, len(chain), want)
		}
		return nil
	default:
		return arityErr(Container, len(chain), want)
	}
}

// linkParentWords are the three parent nouns a link, or a links container,
// may name as the disambiguating first chain segment (para-6g7) — the same
// "spell the reserved word literally when a chain's shape alone cannot say
// which noun it belongs to" device validateContainer already uses at the
// *other* end of its own chain, for "objectives" vs "key-results".
var linkParentWords = []string{"project", "area", "resource"}

// validateLinkParent checks a link's (or a links container's) parent
// portion: chain[0] must be one of linkParentWords, and chain[1:] must be a
// legal id-chain for that noun's own arity — project takes exactly one id,
// area and resource take one or more, and neither takes the bucket (empty)
// form here, because a link always names a specific parent entity, never an
// entire bucket. It is shared between validateLink (which strips the link's
// own trailing id first) and validateContainer's "links" branch (which
// strips the trailing "links" word instead), so the parent-chain rule is
// asked once rather than twice. blame is the noun an arity error should
// name — the caller's own noun, not always Link, so a bad `container
// project.links` refusal reads as container's arity rather than link's.
func validateLinkParent(blame Noun, chain []string) error {
	if len(chain) == 0 || !slices.Contains(linkParentWords, chain[0]) {
		return paraerr.Newf(paraerr.KindValidation,
			"%s's chain must begin with its parent kind (%s)", blame, strings.Join(linkParentWords, ", "))
	}
	ids := chain[1:]
	if chain[0] == Project.String() {
		if len(ids) != 1 {
			return arityErr(blame, len(chain), `"project" followed by exactly one id`)
		}
	} else if len(ids) < 1 {
		return arityErr(blame, len(chain), fmt.Sprintf("%q followed by one or more ids", chain[0]))
	}
	return checkIDs(ids)
}

// validateLink is link's arity (R4): validateLinkParent's parent-chain rule,
// plus the link's own trailing id.
func validateLink(chain []string) error {
	if len(chain) < 2 {
		return arityErr(Link, len(chain), "a parent kind, its id chain, and the link's own id")
	}
	if err := validateLinkParent(Link, chain[:len(chain)-1]); err != nil {
		return err
	}
	return checkID(chain[len(chain)-1])
}

// checkIDs is the two rules an id position must satisfy, over every segment
// in chain: the word is not one locator.ReservedWords reserves, and it is a
// legal locator segment (§1.4). It mirrors kindmeta's unexported checkID,
// which this package cannot use directly — the CLI path this package serves
// must not call kindmeta.KindOf or anything built only for it (plan Phase
// 17 task 6) — so the two rules are asked here against the same
// locator.IsReserved / locator.ValidSegment kindmeta itself asks them
// against, rather than re-declared.
func checkIDs(ids []string) error {
	for _, id := range ids {
		if err := checkID(id); err != nil {
			return err
		}
	}
	return nil
}

func checkID(id string) error {
	if locator.IsReserved(id) {
		return paraerr.Newf(paraerr.KindConflict, "%q is a reserved word and cannot be used as an id", id)
	}
	if !locator.ValidSegment(id) {
		return paraerr.Newf(paraerr.KindValidation, "%q is not a legal id (must match [a-z0-9-]+)", id)
	}
	return nil
}

func arityErr(n Noun, got int, want string) error {
	return paraerr.Newf(paraerr.KindValidation, "%s takes %s, not a chain of %d", n.String(), want, got)
}

func exactSegmentsPhrase(n int) string {
	switch n {
	case 2:
		return "exactly two segments"
	case 3:
		return "exactly three segments"
	default:
		return "a different number of segments"
	}
}
