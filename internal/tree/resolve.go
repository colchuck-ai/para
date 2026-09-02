package tree

import (
	"path/filepath"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// ResolvePath joins loc onto root, returning the absolute filesystem path
// it addresses. It does not check existence.
func ResolvePath(root string, loc locator.Locator) (string, error) {
	rel, err := loc.Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, filepath.FromSlash(rel)), nil
}

// KindAt classifies what a locator names, which is more than kindmeta.KindOf
// answers on its own: a container's name is always a reserved word and a
// reserved word can never be an id (§1.2), so KindOf — which only ever sees id
// positions — never returns KindContainer.
//
// A reserved last segment is *not* on its own enough to make something a
// container, and the difference is one of §10's findings. `projects/skills/` is
// a project someone named with a reserved word (`collision`);
// `projects/acme/key-results/` is a container in a position that has none
// (`misplaced`). Both look like containers to a rule that tests only the name,
// which is why the position is asked instead — and why the error KindOf returns
// is passed through rather than replaced: it already distinguishes the two,
// tagged KindConflict and KindValidation respectively.
//
// The empty locator is the tree root, which is not an entity but the tree
// (§8.1), and so has no kind: it reports KindUnknown and no error. Every caller
// that can be handed a root already has to branch on it, because the root's
// identity lives in tree.toml rather than state.toml.
func KindAt(loc locator.Locator) (kindmeta.Kind, error) {
	if len(loc) == 0 {
		return kindmeta.KindUnknown, nil
	}
	if isContainerPosition(loc) {
		return kindmeta.KindContainer, nil
	}
	info, err := kindmeta.KindOf(loc)
	if err != nil {
		return kindmeta.KindUnknown, err
	}
	return info.Kind, nil
}

// isContainerPosition reports whether loc names a legal container position
// (§1.2): the four buckets, the three archived mirrors, objectives/ under a
// project, and key-results/ under an objective.
//
// This was kindmeta.IsContainer until Phase 17 task P17.6 deleted it: a
// container's name is always a reserved word and a reserved word can never
// be an id, so every container locator is one KindOf is defined to refuse —
// and once an address's noun is spoken (R5), nothing needs to infer a
// container from a locator's shape any more, except the walk. The walk
// still finds containers as bare directories on disk with no noun attached
// (walk.go's own classify), so the position table survives here, its one
// remaining caller — using address.IsBucket for the bucket case, since that
// list moved there with the rest of what IsContainer used to answer, and
// kindmeta.KindOf for the parent-kind case, exactly as IsContainer did.
func isContainerPosition(loc locator.Locator) bool {
	if len(loc) == 0 {
		return false
	}
	last := loc[len(loc)-1]

	if address.IsBucket(loc) {
		return true
	}

	parent, err := kindmeta.KindOf(loc[:len(loc)-1])
	if err != nil {
		return false
	}
	switch last {
	case "objectives":
		return parent.Kind == kindmeta.KindProject
	case "key-results":
		return parent.Kind == kindmeta.KindObjective
	case "links":
		// para-6g7: unlike objectives/key-results, a links container can sit
		// under any of three parent kinds — a project, or an area/resource at
		// any nesting depth.
		return parent.Kind == kindmeta.KindProject || parent.Kind == kindmeta.KindArea || parent.Kind == kindmeta.KindResource
	default:
		return false
	}
}

// Exists reports whether loc currently addresses a live container or
// entity in root — a directory holding .para/state.toml. An archive stub
// (§1.6) has no state.toml and so is not "existing" by this test; it has a
// locator segment but no entity behind it.
func Exists(root string, loc locator.Locator) (bool, error) {
	abs, err := ResolvePath(root, loc)
	if err != nil {
		return false, err
	}
	return fileExists(filepath.Join(abs, ".para", "state.toml")), nil
}

// ParentExists reports whether the directory that would contain loc
// already exists as a container or entity — the "parent must exist" half
// of placement legality (§1.5): an entity's parent chain is entities and
// containers all the way up, never content.
//
// skills.<id> is the one exception (§1.4): its "parent" is .agents/skills/,
// which is shared with non-para content, never carries its own
// .para/state.toml — and is not a precondition at all.
//
// It used to be one, and that was a bug with no repair. The directory is created
// empty by `init` and git does not carry an empty directory (§18.1), so every
// fresh clone of a tree with no skills in it arrives without one — a tree
// `doctor` calls clean and `rebuild` finds nothing to do, on which `para add
// skills.x` failed with ".agents/skills/ does not exist — create it first" and
// no command that would create it. The directory is para's own and the write
// path makes it on the way past, so its absence is not a question about
// placement legality (§1.5), which is what this function answers.
func ParentExists(root string, loc locator.Locator) (bool, error) {
	if len(loc) == 0 {
		return false, paraerr.New(paraerr.KindValidation, "empty locator has no parent")
	}
	if loc.Bucket() == "skills" {
		return true, nil
	}
	if len(loc) >= 2 && loc[len(loc)-2] == "links" {
		// para-6g7: links/ is para's own, created lazily by the first `add
		// link` under a parent (mutate.linksContainerPlan) — the same "make
		// it on the way past" reasoning as .agents/skills/ above. What must
		// actually pre-exist is the *parent* two segments up (the project,
		// or the area/resource at whatever depth), not the links/ container
		// itself.
		return Exists(root, loc[:len(loc)-2])
	}

	parentRel, err := loc[:len(loc)-1].Path()
	if err != nil {
		return false, err
	}
	parentAbs := filepath.Join(root, filepath.FromSlash(parentRel))
	return fileExists(filepath.Join(parentAbs, ".para", "state.toml")), nil
}
