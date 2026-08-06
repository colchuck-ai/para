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
// It exists for the relocating verbs, which are the operations that touch more
// than one entity's worth of bytes (§19). A move or an archive changes the
// locator of every descendant, and a locator is a path — so `move` has to know
// what it is about to carry with it, both to report the count §26 prints and to
// re-render the one projection that names a locator.
//
// It is a walk of a subtree, which §2.3 forbids of a field mutation and which
// these four verbs are the whole exception to. Nothing else may call it.
func Subtree(root string, loc locator.Locator) ([]Node, error) {
	if len(loc) == 0 {
		return nil, paraerr.New(paraerr.KindValidation, "the tree root is not a subtree")
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
		if err := collect(self); err != nil {
			return nil, err
		}
	}
	if err := walkChildren(path, loc, collect); err != nil {
		return nil, err
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
