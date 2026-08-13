package mutate

import (
	"path/filepath"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/version"
)

// Identity is what a tree is called and what it holds — tree.toml's two prose
// fields (§8.1).
//
// It is a struct rather than two arguments because both are optional and the
// pair is what `init` is being told. Name defaults to the directory's own name,
// which is the only thing `para init brain` says about the tree; description
// has no default, because inventing one would be para putting words in a file
// whose whole job is to be the user's sentence about their own tree.
//
// It is also the only chance to set either. The root has no locator (§14), so
// no `para set` can ever reach it — a tree renamed later is renamed by editing
// tree.toml, which is truth and is meant to be readable.
type Identity struct {
	Name        string
	Description string
}

// buckets is every container `init` creates, in the order they are written:
// the four §1.2 names, then the archive's mirror of the three live ones (§1.6).
//
// The locators come from render.AgentsLocations, which is the same eight places
// AGENTS.md and CLAUDE.md are emitted (§6) — the root plus exactly these seven.
// Deriving the list from that one rather than writing it out again is what stops
// a tree from being created with a bucket no AGENTS.md describes, or an
// AGENTS.md location no bucket exists at.
//
// The prose is per bucket and lives here because it is the container's own
// identity (§8.2), not the orientation block AGENTS.md carries. §27 defers this
// wording out of the spec; Phase 15 revises it against a tree in use.
var bucketIdentity = map[string]Identity{
	"projects":          {Name: "Projects", Description: "Work with a finish line."},
	"areas":             {Name: "Areas", Description: "Responsibilities with no finish line."},
	"resources":         {Name: "Resources", Description: "Reference material kept because it is useful later."},
	"archive":           {Name: "Archive", Description: "Everything that has left the live tree."},
	"archive.projects":  {Name: "Archived projects", Description: "Projects that are finished or dropped."},
	"archive.areas":     {Name: "Archived areas", Description: "Areas you no longer hold."},
	"archive.resources": {Name: "Archived resources", Description: "Resources you no longer consult."},
}

// Init creates a tree at e.Root: the root marker and its two siblings, the
// root's own generated files, the seven bucket containers, and .agents/ (§8.1,
// §26).
//
// Two things about it are unlike every other verb, and both come from what it
// is making rather than from a special case:
//
//   - **The root is the last subject.** writeset writes every subject's truth
//     before any subject's projections (§0.2), so making the root last makes
//     .para/tree.toml the last truth byte to land. Until it is there the
//     directory is not a tree — `para` will not find a root above it, and `init`
//     can simply be run again. Once it is there, every bucket's truth is already
//     written and only projections can be missing, which is the one degraded
//     state the design defines a repair for (§2.4).
//
//   - **The root logs four child events and the archive three.** A `child`
//     event is the parent's own record that its set of children changed (§3.3),
//     and `init` creates seven containers under two parents. §8.1's "the root's
//     journal is thin but real — init, config changes, and child events for the
//     four buckets" is those four; the archive's three are the archive's, for
//     the same reason `projects/` rather than the root logs a project.
//
// There is no `init` event, and there is no kind of event that could be one:
// §3.1 fixes four kinds and says outright that "created is a field in the truth
// file, not an event". What §8.1 calls the journal recording `init` is
// tree.toml's `created`, which is what the root's ACTIVITY.md renders its
// "Created." line from — the same rule every entity already follows.
func (e *Env) Init(id Identity) (Result, error) {
	if root, ok, err := tree.Enclosing(e.Root); err != nil {
		return Result{}, err
	} else if ok && root == e.Root {
		return Result{}, paraerr.Newf(paraerr.KindConflict, "%s is already a para tree", e.Root)
	} else if ok {
		return Result{}, paraerr.Newf(paraerr.KindConflict,
			"%s is inside the para tree at %s — a tree cannot contain another", e.Root, root)
	}

	if id.Name == "" {
		id.Name = filepath.Base(e.Root)
	}

	plans, err := e.bucketPlans()
	if err != nil {
		return Result{}, err
	}
	rootPlan, err := e.rootPlan(id)
	if err != nil {
		return Result{}, err
	}

	wrote, err := apply(e, append(plans, rootPlan), nil)
	return Result{Wrote: wrote}, err
}

// rootPlan is the tree itself: tree.toml, an empty config.toml, the four
// child events for the buckets, and the two .agents/ directories.
//
// .agents/rules/ and .agents/skills/ are created empty and nothing may depend
// on their being there — git does not carry an empty directory, so a fresh
// clone will not have them. They exist so that a tree a human is looking at has
// the shape §5 describes rather than two directories that appear the first time
// a skill is added.
func (e *Env) rootPlan(id Identity) (*plan, error) {
	// KindUnknown, which is what tree.KindAt answers for the empty locator and
	// therefore what `open` gives every other verb that touches the root: the
	// root is not an entity, it is the tree (§8.1), so it has no kind. Nothing
	// downstream reads it — render decides the root by the locator being empty —
	// and giving it a different answer here would be a second opinion about a
	// question that already has one.
	subj, err := e.subjectAt(nil, kindmeta.KindUnknown)
	if err != nil {
		return nil, err
	}
	subj.tree = truth.Tree{
		Schema:      truth.Schema,
		ParaVersion: version.String(),
		Name:        id.Name,
		Description: id.Description,
		Created:     stamp(e.Now),
	}

	events := make([]journal.Event, 0, 4)
	for _, bucket := range []string{"projects", "areas", "resources", "archive"} {
		events = append(events, journal.NewChild(e.Now.UTC(), journal.ChildOpAdded, bucket, "", "", ""))
	}

	return &plan{
		subj:      subj,
		events:    events,
		writeTree: true,
		config:    []byte{},
		creating:  true,
		dirs: []string{
			filepath.Join(e.Root, filepath.FromSlash(tree.RulesDir())),
			filepath.Join(e.Root, filepath.FromSlash(tree.SkillsDir())),
		},
	}, nil
}

// bucketPlans is the seven containers, in write order, with the archive's own
// three child events on the archive.
func (e *Env) bucketPlans() ([]*plan, error) {
	var out []*plan
	for _, loc := range render.AgentsLocations() {
		if len(loc) == 0 {
			continue // the root, which rootPlan owns
		}
		id, ok := bucketIdentity[loc.String()]
		if !ok {
			return nil, paraerr.Newf(paraerr.KindInternal, "init: no identity for the bucket %s", relocateAddr(loc))
		}
		subj, err := e.subjectAt(loc, kindmeta.KindContainer)
		if err != nil {
			return nil, err
		}
		subj.state = truth.State{Name: id.Name, Description: id.Description, Created: stamp(e.Now)}
		out = append(out, &plan{
			subj:       subj,
			events:     archiveChildEvents(e, loc),
			writeState: true,
			config:     []byte{},
			creating:   true,
		})
	}
	return out, nil
}

// archiveChildEvents is the archive's record that it gained three containers.
// Every other bucket gains nothing at init, and the root's four are rootPlan's.
func archiveChildEvents(e *Env, loc locator.Locator) []journal.Event {
	if loc.String() != "archive" {
		return nil
	}
	out := make([]journal.Event, 0, 3)
	for _, bucket := range []string{"projects", "areas", "resources"} {
		out = append(out, journal.NewChild(e.Now.UTC(), journal.ChildOpAdded, bucket, "", "", ""))
	}
	return out
}
