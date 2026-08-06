// Package kindmeta derives an entity's kind from its locator (design §1.3)
// and holds the §15 field matrix, the one source of truth from which a
// kind's legal fields, required fields, and sortable fields all fall out.
package kindmeta

import (
	"fmt"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Kind is one of the kinds of directory the tree recognises (§1.2, §1.3).
// KindContainer is never returned by KindOf — containers have no id and so
// no locator addresses them — but it is a row in the field matrix because
// containers have their own state.toml (§8.2).
type Kind int

const (
	KindUnknown Kind = iota
	KindProject
	KindArea
	KindResource
	KindObjective
	KindKeyResult
	KindSkill
	KindContainer
)

func (k Kind) String() string {
	switch k {
	case KindProject:
		return "project"
	case KindArea:
		return "area"
	case KindResource:
		return "resource"
	case KindObjective:
		return "objective"
	case KindKeyResult:
		return "key-result"
	case KindSkill:
		return "skill"
	case KindContainer:
		return "container"
	default:
		return "unknown"
	}
}

// Info is what KindOf derives from a locator: the kind, and whether the
// locator names something under archive/ (§1.6). Archival never changes the
// kind — a project under archive/ is still a project, just dormant.
type Info struct {
	Kind     Kind
	Archived bool
}

// KindOf derives an entity's kind from its locator alone (§1.3) — no
// filesystem read is involved or needed. It rejects two distinct kinds of
// illegal locator, matching doctor's finding names (§10):
//
//   - a reserved word used as an id, tagged KindConflict ("collision");
//   - a locator shaped so that no position in §1.3's table derives a kind
//     (e.g. a project nested two deep, or a key-result outside a
//     key-results/ container), tagged KindValidation ("misplaced").
func KindOf(loc locator.Locator) (Info, error) {
	if len(loc) == 0 {
		return Info{}, paraerr.New(paraerr.KindValidation, "empty locator has no kind")
	}

	segs := []string(loc)
	archived := false
	if loc.IsArchived() {
		archived = true
		segs = segs[1:]
		if len(segs) == 0 {
			return Info{}, misplaced(loc, "archive alone names no entity")
		}
	}

	switch segs[0] {
	case "projects":
		return projectChain(loc, segs[1:], archived)
	case "areas":
		return nestingChain(loc, segs[1:], KindArea, archived)
	case "resources":
		return nestingChain(loc, segs[1:], KindResource, archived)
	case "skills":
		if archived {
			return Info{}, misplaced(loc, "skills cannot be archived")
		}
		return skillChain(loc, segs[1:])
	default:
		return Info{}, misplaced(loc, fmt.Sprintf("%q is not a bucket", segs[0]))
	}
}

func projectChain(loc locator.Locator, rest []string, archived bool) (Info, error) {
	if len(rest) == 0 {
		return Info{}, misplaced(loc, "projects with no id names no entity")
	}
	if err := checkID(loc, rest[0]); err != nil {
		return Info{}, err
	}
	if len(rest) == 1 {
		return Info{Kind: KindProject, Archived: archived}, nil
	}
	if rest[1] != "objectives" {
		return Info{}, misplaced(loc, "a project cannot nest — only objectives/ may follow a project id")
	}
	rest = rest[2:]
	if len(rest) == 0 {
		return Info{}, misplaced(loc, "objectives with no id names no entity")
	}
	if err := checkID(loc, rest[0]); err != nil {
		return Info{}, err
	}
	if len(rest) == 1 {
		return Info{Kind: KindObjective, Archived: archived}, nil
	}
	if rest[1] != "key-results" {
		return Info{}, misplaced(loc, "an objective cannot nest — only key-results/ may follow an objective id")
	}
	rest = rest[2:]
	if len(rest) == 0 {
		return Info{}, misplaced(loc, "key-results with no id names no entity")
	}
	if err := checkID(loc, rest[0]); err != nil {
		return Info{}, err
	}
	if len(rest) > 1 {
		return Info{}, misplaced(loc, "a key-result is a leaf — nothing may follow its id")
	}
	return Info{Kind: KindKeyResult, Archived: archived}, nil
}

// nestingChain handles areas/ and resources/, which nest to arbitrary depth
// (§1.3): every remaining segment is an id.
func nestingChain(loc locator.Locator, rest []string, kind Kind, archived bool) (Info, error) {
	if len(rest) == 0 {
		return Info{}, misplaced(loc, "names no entity — at least one id is required")
	}
	for _, id := range rest {
		if err := checkID(loc, id); err != nil {
			return Info{}, err
		}
	}
	return Info{Kind: kind, Archived: archived}, nil
}

func skillChain(loc locator.Locator, rest []string) (Info, error) {
	if len(rest) != 1 {
		return Info{}, misplaced(loc, "skills is one level only")
	}
	if err := checkID(loc, rest[0]); err != nil {
		return Info{}, err
	}
	return Info{Kind: KindSkill}, nil
}

// checkID is the two rules an id position must satisfy: the word is not one
// §1.4 reserves, and it is a legal locator segment.
//
// The second is not redundant with locator.Parse, and the gap it closes is a
// real one. KindOf's argument does not always come from something a human
// typed: the walk builds a locator out of *directory names* (§1.4 — "locator =
// path, always"), and a directory called `UPPER` or `Big Thing` would otherwise
// derive a perfectly good project kind, be walked as an entity, and be given
// generated files by rebuild — at a locator `show` and `list` then refuse to
// parse. Refusing here makes it §10's `misplaced`, which is what it is.
func checkID(loc locator.Locator, id string) error {
	if locator.IsReserved(id) {
		return paraerr.Newf(paraerr.KindConflict, "%q: %q is a reserved word and cannot be used as an id", loc.String(), id)
	}
	if !locator.ValidSegment(id) {
		return misplaced(loc, fmt.Sprintf("%q is not a legal id (must match [a-z0-9-]+)", id))
	}
	return nil
}

func misplaced(loc locator.Locator, why string) error {
	return paraerr.Newf(paraerr.KindValidation, "%q derives no kind: %s", loc.String(), why)
}
