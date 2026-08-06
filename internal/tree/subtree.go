package tree

import (
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Subtree returns the container or entity at loc followed by everything beneath
// it, in the same stable, lexical, depth-first order Walk visits (§8.5).
//
// It exists for the operations that are about a *region* of the tree rather
// than one entity: the relocating verbs, which §19 names as "the operations
// that touch more than one entity's worth of bytes" and which have to know what
// they are about to carry with them; a scoped `list` or `review`; and Phase
// 12's `rebuild <locator>` and `doctor <locator>`. It is the subtree walk §2.3
// forbids a field mutation, and those are the whole exception to it.
//
// `skills` is answered here rather than by each caller, because the skills are a
// second root rather than a subtree of the first (§1.4) and the segments after
// `skills` still have to mean something: a scope naming the container is every
// skill, and one naming a skill is that skill alone — §5.1 gives a skill no
// children, so descending into one would answer a question about a single thing
// with an answer about the files inside it.
func Subtree(root string, loc locator.Locator) ([]Node, error) {
	if len(loc) == 0 {
		return nil, paraerr.New(paraerr.KindValidation, "the tree root is not a subtree")
	}
	if loc.Bucket() == "skills" {
		return skillSubtree(root, loc)
	}
	path, err := ResolvePath(root, loc)
	if err != nil {
		return nil, err
	}

	var out []Node
	collect := func(n Node) error {
		out = append(out, n)
		return nil
	}

	self, ok, err := classify(loc, path)
	if err != nil {
		return nil, err
	}
	if ok {
		// classify answers from the locator alone, which is right for every
		// node the walk reaches — it only ever classifies a directory it has
		// already found a state.toml in. The scope root is the one node nothing
		// has checked, and it may legitimately have no truth of its own: an
		// archive stub is a bare ancestry placeholder (§1.6), and calling one an
		// entity hands the caller a subject whose state.toml is not there.
		if !fileExists(filepath.Join(path, ".para", "state.toml")) {
			self = Node{Locator: loc, Path: path, Archived: loc.IsArchived(), Stub: true}
		}
		if err := collect(self); err != nil {
			return nil, err
		}
	}
	if err := walkChildren(path, loc, collect); err != nil {
		return nil, err
	}
	return out, nil
}

// skillSubtree is Subtree over the second root: every skill for a bare
// `skills`, and the one named skill for `skills.<id>`, with no descent in
// either case.
func skillSubtree(root string, loc locator.Locator) ([]Node, error) {
	all, err := Skills(root)
	if err != nil {
		return nil, err
	}
	if len(loc) == 1 {
		return all, nil
	}
	var out []Node
	for _, n := range all {
		if n.Locator.String() == loc.String() {
			out = append(out, n)
		}
	}
	return out, nil
}

// Children returns loc's immediate containers, entities, and archive stubs —
// one level of the §8.5 walk, without descending.
//
// It is what the relocating verbs ask when deciding what stays put: unarchiving
// one thing out of an archived subtree moves the ancestor chain it needs and
// leaves every archived sibling where it is (§1.6), so the sibling list is the
// question, and a full Subtree walk would be the wrong shape and the wrong cost.
func Children(root string, loc locator.Locator) ([]Node, error) {
	path, err := ResolvePath(root, loc)
	if err != nil {
		return nil, err
	}

	var out []Node
	depth := len(loc) + 1
	collect := func(n Node) error {
		// walkChildren descends, and a stub is only ever reported together with
		// what it leads to, so the deeper nodes arrive too. Only the immediate
		// generation is being asked about.
		if len(n.Locator) == depth {
			out = append(out, n)
		}
		return nil
	}
	if err := walkChildren(path, loc, collect); err != nil {
		return nil, err
	}
	return out, nil
}

// Skills lists the skills under .agents/skills/, the second root the walk scans
// (§1.4).
//
// It is a directory listing rather than a walk of the tree because §5.4 makes
// para responsible for rewriting every `scope` entry a relocation invalidates,
// and scope lives in exactly one place: a skill's state.toml (§5.2). That
// listing is bounded by the number of skills and reaches no entity, so it is
// not the subtree walk §2.3 forbids — it is the price §5.4 names for scope being
// an enumerated locator list.
func Skills(root string) ([]Node, error) {
	var out []Node
	err := walkAgentsSkills(filepath.Join(root, ".agents", "skills"), func(n Node) error {
		out = append(out, n)
		return nil
	})
	return out, err
}

// SkillIDs lists the ids of the skills that exist, in the same lexical order
// Skills walks them in.
//
// It is what CLAUDE.md's import list is built from (§6.1): one derived rule per
// skill, named by render.RuleFilenames. Asking the skills rather than listing
// .agents/rules/ is what keeps a projection from being derived from a
// projection (§21.1) — see Rules, which is the listing and has exactly one
// legitimate caller.
func SkillIDs(root string) ([]string, error) {
	nodes, err := Skills(root)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if len(n.Locator) == 2 {
			out = append(out, n.Locator[1])
		}
	}
	return out, nil
}

// IsStub reports whether loc names an archive stub: a directory under archive/
// that is there but holds no .para/state.toml of its own (§1.6).
//
// Unlike Walk's Stub flag this asks nothing about what lies beneath. The walk
// has to tell a genuine ancestry placeholder from ordinary untracked content
// that happens to sit at a legal position, and only downstream reality can
// settle that. A relocating verb is asking a narrower question — is this segment
// a directory with no entity behind it — because that is what decides whether
// there is a journal to log a child event in.
func IsStub(root string, loc locator.Locator) (bool, error) {
	if !loc.IsArchived() {
		return false, nil
	}
	path, err := ResolvePath(root, loc)
	if err != nil {
		return false, err
	}
	return dirExists(path) && !fileExists(filepath.Join(path, ".para", "state.toml")), nil
}

// Resolves reports whether loc addresses something that is actually there: a
// live container or entity, an archive stub, or the second root.
//
// It is the question two commands ask in different words and must not answer
// differently. A scoped read refuses a locator that resolves to nothing,
// because an empty result is a wrong answer to an explicit question (Phase 10);
// doctor asks the same of every `scope` entry a skill carries, because an entry
// naming nothing is §10's `scope-unresolved`. A skill whose scope doctor calls
// broken and `list` happily lists would be para contradicting itself about one
// locator.
//
// The three ways to resolve are three different kinds of existing, and all
// three are real:
//
//   - a directory holding .para/state.toml — an entity or a container;
//   - an archive stub, which is a position in the tree with real things
//     beneath it and no entity of its own (§1.6);
//   - a bare `skills`, which addresses the second root rather than a directory
//     of its own (§1.4) and exists whenever the tree does.
func Resolves(root string, loc locator.Locator) (bool, error) {
	if len(loc) == 0 {
		return true, nil
	}
	if len(loc) == 1 && loc[0] == "skills" {
		return true, nil
	}
	exists, err := Exists(root, loc)
	if err != nil || exists {
		return exists, err
	}
	return IsStub(root, loc)
}

// DirExists reports whether loc's directory is present at all, entity or not.
// A stub answers yes here and no to Exists, which is the difference §1.6 rests
// on: a locator segment with no entity behind it.
func DirExists(root string, loc locator.Locator) (bool, error) {
	path, err := ResolvePath(root, loc)
	if err != nil {
		return false, err
	}
	return dirExists(path), nil
}

// Entries lists every name directly inside loc's directory, including files and
// the .para/ directory — everything a relocation has to decide the fate of.
//
// The §8.5 walk deliberately sees only containers and entities; reinstating an
// ancestor has to move its truth, its projections, and any content beside them,
// so it needs the unfiltered listing that walk is defined to skip.
func Entries(root string, loc locator.Locator) ([]string, error) {
	path, err := ResolvePath(root, loc)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, paraerr.Wrap(paraerr.KindInternal, err, "reading "+path)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
