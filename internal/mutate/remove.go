package mutate

import (
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/writeset"
)

// Removal is a planned `remove`: the blast radius, decided before anything is
// deleted, so the interactive confirmation and `--dry-run` are naming the same
// set of bytes the apply would take (§18.4, §19).
type Removal struct {
	Locator locator.Locator
	Kind    kindmeta.Kind

	// Descendants are the containers and entities beneath it, which go too.
	Descendants []locator.Locator

	// KeepFiles is §18.4's flag: leave your content and delete para's footprint
	// throughout the subtree.
	KeepFiles bool

	// Deletes are the paths that will be removed, root-relative and
	// slash-separated. With KeepFiles this is every .para/, ACTIVITY.md, and
	// MEASUREMENTS.csv beneath the subject; without it, the subject's directory.
	Deletes []string

	// Strips are the README.md and SKILL.md files whose frontmatter block will be
	// removed while their body survives — v4's cost for generating into files you
	// also write in (§18.4).
	Strips []string

	env   *Env
	reloc writeset.Relocation
	files []writeset.File
}

// PlanRemove decides a `remove` in full without touching a byte (§18.4).
func (e *Env) PlanRemove(loc locator.Locator, keepFiles bool) (*Removal, error) {
	kind, err := e.relocatable(loc, "remove", "removed")
	if err != nil {
		return nil, err
	}

	nodes, err := tree.Subtree(e.Root, loc)
	if err != nil {
		return nil, err
	}

	r := &Removal{Locator: loc, Kind: kind, KeepFiles: keepFiles, env: e}
	for _, n := range nodes {
		if n.Locator.String() != loc.String() {
			r.Descendants = append(r.Descendants, n.Locator)
		}
	}

	// A skill's derived rule goes with it. §18.2 is explicit — "`remove skills.x`
	// is how a skill stops applying, and it takes its derived rule with it" — and
	// §5.3 makes the ownership checkable: the rule carries `generated_from`, so
	// para knows the file is its own. A rule left behind is `doctor`'s orphan-rule.
	if kind == kindmeta.KindSkill {
		r.prune(filepath.Join(e.Root, filepath.FromSlash(render.RulesDir), render.RuleFilename(loc[1])))
	}

	if !keepFiles {
		r.prune(nodes[0].Path)
	} else if err := r.planKeepFiles(nodes); err != nil {
		return nil, err
	}

	// A `scope` entry naming the removed locator is deliberately left alone. §5.4
	// makes para responsible for rewriting scope on move, archive, unarchive, and
	// id changes — every case where the thing still exists somewhere. There is no
	// locator to rewrite to here, and an entry naming nothing is precisely what
	// `doctor`'s scope-unresolved finding is for (§5.4, §10): silently narrowing a
	// skill's scope would be a decision para has no standing to make.

	// The stub above a removed entity may now record nothing at all, and a stub
	// that records nothing records nothing (§1.6).
	if err := r.planEmptyStub(loc); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Removal) prune(path string) {
	r.reloc.Prunes = append(r.reloc.Prunes, path)
	r.Deletes = append(r.Deletes, r.env.rel(path))
}

// planKeepFiles is §18.4's second half: delete every .para/, every ACTIVITY.md,
// and every MEASUREMENTS.csv throughout the subtree, and strip the frontmatter
// block from every identity file while keeping the body.
//
// The body is kept byte for byte, including a leading blank line the frontmatter
// used to sit above. §2.1 makes bodies human-owned everywhere else, and a flag
// whose whole purpose is "keep my content" is the worst place to start editing it.
func (r *Removal) planKeepFiles(nodes []tree.Node) error {
	for _, n := range nodes {
		if n.Stub {
			continue
		}
		r.prune(filepath.Join(n.Path, ".para"))
		r.prune(filepath.Join(n.Path, "ACTIVITY.md"))
		r.prune(filepath.Join(n.Path, "MEASUREMENTS.csv"))

		// The identity file, which is SKILL.md for a skill and README.md for
		// everything else (§2.2). Both are frontmatter over a human-owned body, so
		// both get the same treatment; §18.4 names only README.md because a skill
		// is the one kind whose identity file is not one.
		identity := "README.md"
		if n.Kind == kindmeta.KindSkill {
			identity = "SKILL.md"
		}
		path := filepath.Join(n.Path, identity)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return paraerr.Wrap(paraerr.KindInternal, err, "reading "+path)
		}
		_, body, err := mdfile.Split(data)
		if err != nil {
			// No frontmatter to strip: a file that is already all body. Nothing
			// to write, and nothing lost.
			continue
		}
		r.files = append(r.files, writeset.File{Path: path, Bytes: body})
		r.Strips = append(r.Strips, r.env.rel(path))
	}
	return nil
}

// planEmptyStub drops the archive stub above loc when loc was the last thing
// beneath it (§1.6).
func (r *Removal) planEmptyStub(loc locator.Locator) error {
	if !loc.IsArchived() || len(loc) < 3 {
		return nil
	}
	parent := loc[:len(loc)-1]
	isStub, err := tree.IsStub(r.env.Root, parent)
	if err != nil || !isStub {
		return err
	}
	children, err := tree.Children(r.env.Root, parent)
	if err != nil {
		return err
	}
	for _, c := range children {
		if c.Locator.String() != loc.String() {
			return nil
		}
	}
	path, err := tree.ResolvePath(r.env.Root, parent)
	if err != nil {
		return err
	}
	r.prune(path)
	return nil
}

// Apply deletes what the plan named and records the removal at the parent.
//
// Deleting comes first, for the reason writeset.Relocation gives: the bytes are
// the truth, and the parent's `child removed` line is the record of what happened
// to them. A crash between the two loses the record of a deletion that happened,
// which is a stale projection; the other order would leave a record of a deletion
// that did not.
func (r *Removal) Apply() (Result, error) {
	e := r.env
	res := Result{Locator: r.Locator, Kind: r.Kind}

	ops, err := writeset.Relocate(r.reloc)
	res.Wrote = e.wrote(ops)
	if err != nil {
		return res, err
	}

	if len(r.files) > 0 {
		// Not a plan, because there is no subject left to have one: the entity is
		// gone and these are the remains of its README. They ride in as a
		// Subject's projections because that is what they are — the generated half
		// of a partly-generated file, being rewritten to its human-owned half.
		dir, err := tree.ResolvePath(e.Root, r.Locator)
		if err != nil {
			return res, err
		}
		wrote, err := writeset.Apply(writeset.Mutation{
			Subjects: []writeset.Subject{{Dir: dir, Projections: r.files}},
		})
		res.Wrote = append(res.Wrote, e.wrote(wrote)...)
		if err != nil {
			return res, err
		}
	}

	parent, err := e.parentPlan(r.Locator, []journal.Event{
		journal.NewChild(e.Now.UTC(), journal.ChildOpRemoved, r.Locator[len(r.Locator)-1], "", "", ""),
	})
	if err != nil {
		return res, err
	}
	if parent != nil {
		wrote, err := apply(e, nil, []*plan{parent}, false)
		res.Wrote = append(res.Wrote, wrote...)
		if err != nil {
			return res, err
		}
	}
	// A skill has no parent to log at (§1.4), so this is the only place its
	// removal reaches the surface: the mirror goes with the rule (§6.1, §18.2).
	return e.syncSurface(res, nil, false, nil)
}
