package address

import (
	"slices"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// ToLocator converts a to locator.Locator, the internal representation
// (plan §0.1). a is validated first, so a hand-built Address (e.g. a struct
// literal in a test) is held to the same rule a parsed one is.
//
// The conversion applies R5's two absorbed exceptions: the skill noun's
// singular word becomes the "skills" bucket segment — the "para-" path
// prefix is locator.Locator.Path's own concern, unchanged by this package
// (plan §0.1) — and, for objective and key-result, the structural words
// "objectives" and "key-results" that a.Chain does not carry (§11's
// decision log: the chain is short, the structural segments are
// recoverable from the noun) are inserted between the ids that surround
// them.
func (a Address) ToLocator() (locator.Locator, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}

	var segs []string
	if a.Archived {
		segs = append(segs, "archive")
	}

	switch a.Noun {
	case Project:
		segs = append(segs, "projects")
		segs = append(segs, a.Chain...)
	case Area:
		segs = append(segs, "areas")
		segs = append(segs, a.Chain...)
	case Resource:
		segs = append(segs, "resources")
		segs = append(segs, a.Chain...)
	case Skill:
		segs = append(segs, "skills")
		segs = append(segs, a.Chain...)
	case Objective:
		// a.Chain is [projectID, objectiveID] (R3): "objectives" is
		// inserted between them, never typed or stored.
		segs = append(segs, "projects", a.Chain[0], "objectives", a.Chain[1])
	case KeyResult:
		// a.Chain is [projectID, objectiveID, keyResultID] (R3): both
		// structural words are inserted around the middle id.
		segs = append(segs, "projects", a.Chain[0], "objectives", a.Chain[1], "key-results", a.Chain[2])
	case Container:
		// a.Chain's last segment already is the structural word (R4); only
		// the three-segment shape needs "objectives" inserted before the
		// objective id that precedes "key-results".
		segs = append(segs, "projects", a.Chain[0])
		if len(a.Chain) == 2 {
			segs = append(segs, a.Chain[1])
		} else {
			segs = append(segs, "objectives", a.Chain[1], a.Chain[2])
		}
	default:
		return nil, paraerr.Newf(paraerr.KindValidation, "%q is not a noun", a.Noun.String())
	}

	return locator.Locator(segs), nil
}

// FromLocator converts loc, the internal representation, to the external
// address it names — the inverse of ToLocator, and total over every
// locator ToLocator can produce (plan §0.2), including the skill exception
// and the archive qualifier.
//
// It does not call kindmeta.KindOf or kindmeta.IsContainer: nothing in the
// CLI path may (plan Phase 17 task 6) — deriving a kind from a locator by
// inference is exactly the thing speaking the noun makes unnecessary for
// this caller. kindmeta.KindOf remains the walk's own way of classifying a
// directory found on disk, a different question for a different caller
// (plan §0.1); FromLocator asks the structural question a locator built
// from typed or stored input needs answered — was this a legal address to
// begin with — over the same locator.IsReserved / locator.ValidSegment
// rules kindmeta itself asks it against.
func FromLocator(loc locator.Locator) (Address, error) {
	if len(loc) == 0 {
		return Address{}, paraerr.New(paraerr.KindValidation, "empty locator has no address")
	}

	segs := []string(loc)
	archived := false
	if loc.IsArchived() {
		archived = true
		segs = segs[1:]
		if len(segs) == 0 {
			return Address{}, paraerr.Newf(paraerr.KindValidation, "%q: archive alone is not an address", loc.String())
		}
	}

	switch segs[0] {
	case "projects":
		return projectAddress(loc, segs[1:], archived)
	case "areas":
		return nestingAddress(segs[1:], Area, archived)
	case "resources":
		return nestingAddress(segs[1:], Resource, archived)
	case "skills":
		if archived {
			return Address{}, paraerr.Newf(paraerr.KindValidation, "%q: skills cannot be archived", loc.String())
		}
		return skillAddress(loc, segs[1:])
	default:
		return Address{}, paraerr.Newf(paraerr.KindValidation, "%q: %q is not a bucket", loc.String(), segs[0])
	}
}

// projectAddress walks the one bucket that nests in a fixed, named shape
// (R3): a project id, optionally "objectives" and an objective id,
// optionally "key-results" and a key-result id — mirroring
// kindmeta.projectChain's traversal, independently, for the reason
// FromLocator's own doc comment gives.
func projectAddress(loc locator.Locator, rest []string, archived bool) (Address, error) {
	if len(rest) == 0 {
		return Address{Noun: Project, Archived: archived}, nil
	}
	if err := checkID(rest[0]); err != nil {
		return Address{}, err
	}
	if len(rest) == 1 {
		return Address{Noun: Project, Chain: []string{rest[0]}, Archived: archived}, nil
	}
	if rest[1] != "objectives" {
		return Address{}, misplaced(loc, "a project cannot nest — only objectives/ may follow a project id")
	}
	if len(rest) == 2 {
		return Address{Noun: Container, Chain: []string{rest[0], "objectives"}, Archived: archived}, nil
	}
	if err := checkID(rest[2]); err != nil {
		return Address{}, err
	}
	if len(rest) == 3 {
		return Address{Noun: Objective, Chain: []string{rest[0], rest[2]}, Archived: archived}, nil
	}
	if rest[3] != "key-results" {
		return Address{}, misplaced(loc, "an objective cannot nest — only key-results/ may follow an objective id")
	}
	if len(rest) == 4 {
		return Address{Noun: Container, Chain: []string{rest[0], rest[2], "key-results"}, Archived: archived}, nil
	}
	if err := checkID(rest[4]); err != nil {
		return Address{}, err
	}
	if len(rest) > 5 {
		return Address{}, misplaced(loc, "a key-result is a leaf — nothing may follow its id")
	}
	return Address{Noun: KeyResult, Chain: []string{rest[0], rest[2], rest[4]}, Archived: archived}, nil
}

// nestingAddress handles areas/ and resources/, which nest to arbitrary
// depth (R3): every remaining segment is an id.
func nestingAddress(rest []string, noun Noun, archived bool) (Address, error) {
	if len(rest) == 0 {
		return Address{Noun: noun, Archived: archived}, nil
	}
	if err := checkIDs(rest); err != nil {
		return Address{}, err
	}
	return Address{Noun: noun, Chain: slices.Clone(rest), Archived: archived}, nil
}

func skillAddress(loc locator.Locator, rest []string) (Address, error) {
	if len(rest) == 0 {
		return Address{Noun: Skill}, nil
	}
	if len(rest) != 1 {
		return Address{}, misplaced(loc, "skills is one level only")
	}
	if err := checkID(rest[0]); err != nil {
		return Address{}, err
	}
	return Address{Noun: Skill, Chain: []string{rest[0]}}, nil
}

func misplaced(loc locator.Locator, why string) error {
	return paraerr.Newf(paraerr.KindValidation, "%q derives no address: %s", loc.String(), why)
}
