package mutate

import (
	"path/filepath"
	"slices"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/writeset"
)

// archiveBucket is the one segment archival prepends. Archiving moves bytes and
// nothing else: no field changes, no status, because location *is* archival state
// (§1.6, §1.7).
const archiveBucket = "archive"

// archived is loc's counterpart under archive/.
func archived(loc locator.Locator) locator.Locator {
	return append(locator.Locator{archiveBucket}, loc...)
}

// PlanArchive decides an `archive` in full without touching a byte (§18.5).
//
// It takes the whole subtree in one move and creates stubs for any live ancestors
// that stay behind, which is the only bookkeeping archival needs: everything else
// about being archived — dormancy, terminal cascade, which rules apply — is
// derived from the path (§1.6).
func (e *Env) PlanArchive(loc locator.Locator) (*Relocation, error) {
	if loc.IsArchived() {
		return nil, paraerr.Newf(paraerr.KindValidation, "%s is already archived", loc)
	}
	kind, err := e.relocatable(loc, "archive", "archived")
	if err != nil {
		return nil, err
	}
	if kind == kindmeta.KindSkill {
		// §1.7: skills have no status and no off switch, and no archiving verb
		// either. A skill that exists applies; one you want gone is one you
		// remove, and removing it takes its derived rule with it (§5.3).
		return nil, paraerr.Newf(paraerr.KindValidation,
			"a skill cannot be archived — `para remove %s` is how a skill stops applying", loc)
	}

	dst := archived(loc)
	exists, err := tree.Exists(e.Root, dst)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, paraerr.Newf(paraerr.KindConflict,
			"%s already exists — rename one of them first", dst)
	}

	srcPath, err := tree.ResolvePath(e.Root, loc)
	if err != nil {
		return nil, err
	}
	dstPath, err := tree.ResolvePath(e.Root, dst)
	if err != nil {
		return nil, err
	}

	r := &Relocation{
		Verb:  VerbArchived,
		From:  loc,
		To:    dst,
		Kind:  kind,
		env:   e,
		moves: []entityMove{{From: loc, To: dst, Subtree: true}},
	}

	// The ancestry chain on the archive side. A segment that is not there is
	// created as a bare directory — no README.md, no .para/ — purely to record
	// where the thing came from (§1.6). Only id positions are reported as stubs:
	// archive/ and archive/projects/ are containers `init` owns, and creating one
	// that a hand-planted tree is missing is repair, not ancestry.
	for i := 2; i < len(dst); i++ {
		ancestor := dst[:i]
		present, err := tree.DirExists(e.Root, ancestor)
		if err != nil {
			return nil, err
		}
		path, err := tree.ResolvePath(e.Root, ancestor)
		if err != nil {
			return nil, err
		}
		r.reloc.Dirs = append(r.reloc.Dirs, path)
		if !present && i >= 3 {
			r.StubsCreated = append(r.StubsCreated, ancestor)
		}
	}

	adopting, err := tree.DirExists(e.Root, dst)
	if err != nil {
		return nil, err
	}
	if adopting {
		// The destination is already there without an entity behind it: a stub
		// an earlier archive left when this very directory's descendant went
		// first. The stub becomes the entity (§26), which cannot be one rename —
		// rename(2) will not merge — so each of the source's entries moves in and
		// the emptied source is pruned.
		if err := r.planAdoption(srcPath, dstPath, loc, dst); err != nil {
			return nil, err
		}
		r.StubAdopted = dst
	} else {
		r.reloc.Moves = append(r.reloc.Moves, writeset.Move{From: srcPath, To: dstPath})
	}

	if err := r.carried(); err != nil {
		return nil, err
	}
	if err := r.planScope(); err != nil {
		return nil, err
	}
	return r, nil
}

// planAdoption moves every entry of the source into an existing destination
// directory, refusing a name the destination already holds.
//
// That collision is the mirror of §1.6's id collision on unarchive: a live child
// and an already-archived child with the same id cannot both become the archived
// entity's child, so para refuses and names them rather than picking one.
func (r *Relocation) planAdoption(srcPath, dstPath string, src, dst locator.Locator) error {
	taken, err := tree.Entries(r.env.Root, dst)
	if err != nil {
		return err
	}
	entries, err := tree.Entries(r.env.Root, src)
	if err != nil {
		return err
	}
	for _, name := range entries {
		if slices.Contains(taken, name) {
			return paraerr.Newf(paraerr.KindConflict,
				"%s already exists — rename or archive %s separately",
				r.env.rel(filepath.Join(dstPath, name)), r.env.rel(filepath.Join(srcPath, name)))
		}
		r.reloc.Moves = append(r.reloc.Moves, writeset.Move{
			From: filepath.Join(srcPath, name),
			To:   filepath.Join(dstPath, name),
		})
	}
	r.reloc.Prunes = append(r.reloc.Prunes, srcPath)
	return nil
}

// PlanUnarchive decides an `unarchive` in full without touching a byte (§18.5).
//
// It is archive's mirror. Archive drags the named entity's subtree down and leaves
// a stub for a live ancestor that stayed put; unarchive drags the named entity's
// subtree *and its archived ancestor chain* up, and leaves a stub for an archived
// sibling that stays put (§1.6). An ancestor that is already live is adopted
// rather than duplicated — `areas/health` existing is exactly the condition under
// which nothing needs reinstating.
func (e *Env) PlanUnarchive(loc locator.Locator) (*Relocation, error) {
	if !loc.IsArchived() {
		return nil, paraerr.Newf(paraerr.KindValidation, "%s is not archived", loc)
	}
	if len(loc) < 3 {
		// archive/ and archive/projects/ are the archive's own containers, not
		// things that came from anywhere (§6, §8.2).
		return nil, paraerr.Newf(paraerr.KindValidation, "%s is part of the archive itself", loc)
	}
	kind, err := e.relocatable(loc, "unarchive", "unarchived")
	if err != nil {
		return nil, err
	}

	dst := loc[1:]
	exists, err := tree.Exists(e.Root, dst)
	if err != nil {
		return nil, err
	}
	if exists {
		// §1.6, and §26's spelling: the id is taken by something live, so there
		// is no place to put this. Only the entity named gets this answer; an
		// ancestor whose id is taken is adopted below.
		return nil, paraerr.Newf(paraerr.KindConflict, "%s exists; rename it or leave this archived", dst)
	}

	r := &Relocation{
		Verb: VerbUnarchived,
		From: loc,
		To:   dst,
		Kind: kind,
		env:  e,
	}
	if err := r.planCascade(dst); err != nil {
		return nil, err
	}

	srcPath, err := tree.ResolvePath(e.Root, loc)
	if err != nil {
		return nil, err
	}
	dstPath, err := tree.ResolvePath(e.Root, dst)
	if err != nil {
		return nil, err
	}
	r.reloc.Moves = append(r.reloc.Moves, writeset.Move{From: srcPath, To: dstPath})

	// The named entity is listed first among the moves, so scope rewriting tries
	// the most specific match before an ancestor's exact one (see rewriteScope).
	r.moves = append([]entityMove{{From: loc, To: dst, Subtree: true}}, r.moves...)

	if err := r.carried(); err != nil {
		return nil, err
	}
	if err := r.planScope(); err != nil {
		return nil, err
	}
	return r, nil
}

// level is one ancestor of the thing being unarchived, seen from both sides.
type level struct {
	live      locator.Locator
	arch      locator.Locator
	reinstate bool
	// stub reports that the archive side is a bare directory with no truth of
	// its own, which is what makes it safe to remove once it records nothing.
	stub bool
	// next is the entry name leading down the path — the child that is leaving,
	// one way or another.
	next string
}

// planCascade decides what happens to each archived ancestor of dst: reinstated
// as a live entity, or adopted because a live one is already there.
//
// It also decides the fate of each archive-side directory the cascade empties or
// hollows out. A directory left holding an archived sibling stays as a stub — that
// sibling's ancestry is exactly what a stub is for. One left holding nothing is
// removed, because a stub that records nothing records nothing.
func (r *Relocation) planCascade(dst locator.Locator) error {
	e := r.env

	levels := make([]level, 0, len(dst)-1)
	for i := 1; i < len(dst); i++ {
		l := level{live: dst[:i], arch: archived(dst[:i]), next: dst[i]}

		liveExists, err := tree.Exists(e.Root, l.live)
		if err != nil {
			return err
		}
		if i == 1 {
			// The bucket. It is the tree's own furniture (§6), never archived and
			// never reinstated: archive/areas/ is the archive's container, not an
			// archived copy of areas/.
			if !liveExists {
				return paraerr.Newf(paraerr.KindNotFound, "%s does not exist", l.live)
			}
			levels = append(levels, l)
			continue
		}
		if !liveExists {
			archExists, err := tree.Exists(e.Root, l.arch)
			if err != nil {
				return err
			}
			if !archExists {
				// Neither side has an entity here: the live parent was removed
				// after its descendant was archived, leaving the stub pointing at
				// nothing. There is nothing to reinstate and nowhere to land.
				return paraerr.Newf(paraerr.KindNotFound, "%s does not exist — create it first", l.live)
			}
			l.reinstate = true
		}
		if l.stub, err = tree.IsStub(e.Root, l.arch); err != nil {
			return err
		}
		levels = append(levels, l)
	}

	// Deepest first, because whether a level is left empty depends on whether the
	// level below it was removed. Index 0 is the bucket, which is skipped: it is
	// not a stub and never becomes one.
	pruned := map[string]bool{}
	for i := len(levels) - 1; i >= 1; i-- {
		l := levels[i]
		// The child leaving is gone from this directory when it is the entity
		// being unarchived, or when it is a deeper archive directory this pass
		// already decided to remove.
		gone := i == len(levels)-1 || pruned[archived(dst[:i+2]).String()]

		moving, leftover, err := r.split(l, gone)
		if err != nil {
			return err
		}
		if l.reinstate {
			r.Reinstated = append(r.Reinstated, l.live)
			for _, name := range moving {
				r.reloc.Moves = append(r.reloc.Moves, writeset.Move{
					From: filepath.Join(r.abs(l.arch), name),
					To:   filepath.Join(r.abs(l.live), name),
				})
			}
			r.moves = append(r.moves, entityMove{From: l.arch, To: l.live, Subtree: false})
		}
		switch {
		// Prunable only once there is provably no truth left in it: a directory
		// this pass reinstated has had its .para/ moved up, and a stub never had
		// one. An archived entity that stayed put keeps its journal and its
		// fields, so an empty-looking one is still an entity and deleting it
		// would destroy truth (§2.1).
		case leftover == 0 && (l.reinstate || l.stub):
			r.reloc.Prunes = append(r.reloc.Prunes, r.abs(l.arch))
			r.StubsRemoved = append(r.StubsRemoved, l.arch)
			pruned[l.arch.String()] = true
		case l.reinstate:
			r.StubsDemoted = append(r.StubsDemoted, l.arch)
		}
	}

	// Report shallowest first, which is the order the reinstatement reads in and
	// the order the renames happen in.
	slices.Reverse(r.Reinstated)
	slices.Reverse(r.StubsRemoved)
	slices.Reverse(r.StubsDemoted)
	slices.Reverse(r.moves)
	slices.Reverse(r.reloc.Moves)
	return nil
}

// split decides, for one archive-side directory, which of its entries move up
// with the ancestor being reinstated and how many stay behind.
//
// The discriminator is §8.5's own: a directory the walk reports as a container,
// an entity, or a stub is a thing in its own right and stays archived; everything
// else — the ancestor's .para/, its README.md and ACTIVITY.md, and any content
// beside them — belongs to the ancestor and travels with it. An adopted level
// moves nothing, because its live counterpart never left.
func (r *Relocation) split(l level, gone bool) (moving []string, leftover int, err error) {
	entries, err := tree.Entries(r.env.Root, l.arch)
	if err != nil {
		return nil, 0, err
	}
	children, err := tree.Children(r.env.Root, l.arch)
	if err != nil {
		return nil, 0, err
	}
	isNode := make(map[string]bool, len(children))
	for _, c := range children {
		isNode[c.Locator[len(c.Locator)-1]] = true
	}

	for _, name := range entries {
		switch {
		case name == l.next:
			if !gone {
				leftover++
			}
		case l.reinstate && !isNode[name]:
			moving = append(moving, name)
		default:
			leftover++
		}
	}
	return moving, leftover, nil
}

// abs is loc's absolute path. The error ResolvePath can return is impossible
// here — every locator these callers hold has already been resolved once — and
// swallowing it keeps the cascade readable.
func (r *Relocation) abs(loc locator.Locator) string {
	path, err := tree.ResolvePath(r.env.Root, loc)
	if err != nil {
		return ""
	}
	return path
}
