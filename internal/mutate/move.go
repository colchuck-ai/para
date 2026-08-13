package mutate

import (
	"path/filepath"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/writeset"
)

// PlanMove decides a `move` in full without touching a byte (§18.3).
//
// One rename, plus the projection work a hand-`mv` cannot do: the moved entity's
// README frontmatter and every descendant's, both parents' journals and
// ACTIVITY.md, and every `scope` entry naming the moved locator or anything
// beneath it (§5.4).
func (e *Env) PlanMove(src, dst locator.Locator) (*Relocation, error) {
	srcKind, err := e.relocatable(src, "move", "moved")
	if err != nil {
		return nil, err
	}

	dstKind, err := tree.KindAt(dst)
	if err != nil {
		return nil, err
	}
	if dstKind == kindmeta.KindContainer {
		return nil, paraerr.Newf(paraerr.KindValidation,
			"%s names a container — a container is part of its parent's shape, not a destination", relocateAddr(dst))
	}

	// Same-kind only, and it is checked before anything about the destination's
	// surroundings, because §26's answer to `move projects.acme areas.acme` is
	// about the kinds and not about whether areas/ happens to exist. A kind
	// change would change which fields are legal (§15), so there is no honest
	// way to carry the state across.
	if srcKind != dstKind {
		return nil, paraerr.Newf(paraerr.KindValidation,
			"kind would change (%s → %s); create the target and move your content", srcKind, dstKind)
	}

	// The archive is a place, and getting to it or out of it is a verb of its own
	// (§1.6, §18.5). Letting `move` do it would mean `move` owning stubs,
	// cascades, and id collisions — all of which archive/unarchive already own.
	if src.IsArchived() != dst.IsArchived() {
		if dst.IsArchived() {
			return nil, paraerr.Newf(paraerr.KindValidation,
				"%s is in the archive — that is `para archive`, not `para move`", relocateAddr(dst))
		}
		return nil, paraerr.Newf(paraerr.KindValidation,
			"%s is in the archive — that is `para unarchive`, not `para move`", relocateAddr(src))
	}

	if strings.HasPrefix(dst.String(), src.String()+".") {
		return nil, paraerr.Newf(paraerr.KindValidation, "%s is inside %s", relocateAddr(dst), relocateAddr(src))
	}

	exists, err := tree.Exists(e.Root, dst)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, paraerr.Newf(paraerr.KindConflict, "%s already exists", relocateAddr(dst))
	}
	parentExists, err := tree.ParentExists(e.Root, dst)
	if err != nil {
		return nil, err
	}
	if !parentExists {
		return nil, paraerr.Newf(paraerr.KindNotFound, "%s does not exist — create it first", relocateAddr(dst[:len(dst)-1]))
	}

	srcPath, err := tree.ResolvePath(e.Root, src)
	if err != nil {
		return nil, err
	}
	dstPath, err := tree.ResolvePath(e.Root, dst)
	if err != nil {
		return nil, err
	}

	r := &Relocation{
		Verb:  VerbMoved,
		From:  src,
		To:    dst,
		Kind:  srcKind,
		env:   e,
		moves: []entityMove{{From: src, To: dst, Subtree: true}},
		reloc: writeset.Relocation{Moves: []writeset.Move{{From: srcPath, To: dstPath}}},
	}

	// Renaming a skill renames its derived rule, and the rule the old id owned
	// has to go: a rule is a projection of a skill and nothing else (§5.3), so one
	// left behind would be `doctor`'s orphan-rule — para's own droppings reported
	// as a fault. The new one is written by the skill's own renderer set.
	if srcKind == kindmeta.KindSkill {
		r.reloc.Prunes = append(r.reloc.Prunes,
			filepath.Join(e.Root, filepath.FromSlash(render.RulesDir), render.RuleFilename(src[1])))
	}

	if err := r.carried(); err != nil {
		return nil, err
	}
	if err := r.planScope(); err != nil {
		return nil, err
	}
	return r, nil
}
