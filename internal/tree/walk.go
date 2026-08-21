package tree

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
)

// Node is one container, entity, or archive stub the walk finds.
type Node struct {
	Locator     locator.Locator
	Path        string // absolute filesystem path
	Kind        kindmeta.Kind
	IsContainer bool
	Archived    bool
	// Stub marks an archive.* directory with no .para/ of its own (§1.6):
	// a bare placeholder recording ancestry for a descendant that was
	// archived while this segment's live counterpart stayed behind.
	Stub bool
}

// Walk implements the §8.5 walk from root: read a directory's immediate
// children, descend into those holding .para/state.toml, never into
// content. It never follows a symlink (§6.1, §21.2), so a mirrored skill
// can never manufacture a phantom entity. Skills live under .agents/skills/
// rather than under a bucket, so Walk scans that directory as a second,
// parallel root (§1.4's skills.<id> exception) — and, since para-a3p, visits
// the bucket directory itself first, the same way walkChildren visits every
// other container before its own children.
//
// visit is called once per container, entity, or archive stub, in a
// stable, lexical, depth-first order. Containers are visited too — the
// walk's job is classification, not presentation — so a caller that wants
// only entities (e.g. list, §16.2) filters IsContainer out itself.
func Walk(root string, visit func(Node) error) error {
	if err := walkChildren(root, nil, visit); err != nil {
		return err
	}
	skillsPath := filepath.Join(root, filepath.FromSlash(skillsDir))
	if err := visitSkillsBucket(skillsPath, visit); err != nil {
		return err
	}
	return walkAgentsSkills(skillsPath, visit)
}

func walkChildren(dir string, loc locator.Locator, visit func(Node) error) error {
	entries, err := readDirs(dir)
	if err != nil {
		return err
	}

	inArchive := loc.IsArchived()

	for _, e := range entries {
		name := e.Name()
		childPath := filepath.Join(dir, name)
		childLoc := append(slices.Clone(loc), name)

		switch {
		case fileExists(filepath.Join(childPath, ".para", "state.toml")):
			node, ok, err := classify(childLoc, childPath)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := visit(node); err != nil {
				return err
			}
			if err := walkChildren(childPath, childLoc, visit); err != nil {
				return err
			}
		case inArchive && (!locator.IsReserved(name) || isContainerPosition(childLoc)):
			// A stub chain runs through containers as well as entities:
			// archiving one objective out of a live project leaves
			// archive/projects/acme/objectives/ as a bare directory with the
			// archived objective inside it (§1.6). Refusing every reserved name
			// here would stop the walk at that `objectives/` and lose the
			// entity beneath it — which `doctor` then reports as an orphan, for
			// a tree para itself produced.
			if err := walkArchiveStub(childPath, childLoc, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// walkArchiveStub handles a bare, non-reserved-named directory under
// archive/ that has no .para/ of its own. It may be a genuine stub — the
// ancestry placeholder §1.6 describes, left behind when a descendant was
// archived but this segment's live counterpart stayed put — or it may be
// ordinary untracked content that merely happens to sit at a position
// kindmeta considers legal (areas and resources nest to unlimited depth, so
// shape alone can't tell the two apart there).
//
// The discriminator is downstream reality, not shape: descend and buffer
// what's found; only report this directory as a stub, and flush the buffer,
// if something real — an entity or a further stub chain leading to one —
// actually turns up beneath it. An empty result means it was content all
// along, and per §8.5 the walk never descends into content, so nothing here
// is reported. A position kindmeta can't derive any kind for at all (e.g.
// content nested inside a project, which cannot nest) is content beyond
// doubt, and is skipped without even trying.
func walkArchiveStub(path string, loc locator.Locator, visit func(Node) error) error {
	// KindAt rather than kindmeta.KindOf, because a stub chain passes through
	// container positions too and KindOf is defined to refuse every one of
	// them: a container's name is always a reserved word (§1.2).
	if _, err := KindAt(loc); err != nil {
		return nil
	}

	var found []Node
	buffer := func(n Node) error {
		found = append(found, n)
		return nil
	}
	if err := walkChildren(path, loc, buffer); err != nil {
		return err
	}
	if len(found) == 0 {
		return nil
	}

	if err := visit(Node{Locator: loc, Path: path, Archived: true, Stub: true}); err != nil {
		return err
	}
	for _, n := range found {
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}

// classify derives a found directory's Node from its locator alone, via the
// same KindAt every other caller uses. ok is false for a locator kindmeta
// cannot derive a kind for — malformed placements are doctor's job (a later
// phase), not the walk's, so Walk simply does not visit them.
func classify(loc locator.Locator, path string) (Node, bool, error) {
	kind, err := KindAt(loc)
	if err != nil {
		return Node{}, false, nil
	}
	return Node{
		Locator:     loc,
		Path:        path,
		Kind:        kind,
		IsContainer: kind == kindmeta.KindContainer,
		Archived:    loc.IsArchived(),
	}, true, nil
}

// visitSkillsBucket visits the skill bucket itself (para-a3p), if it has a
// state.toml — a tree `init` made after the skill bucket became a container
// does; an older tree, hand-planted, or built before this task, does not,
// and simply has one fewer node to visit, the same as any other absent
// container.
//
// It is deliberately not folded into walkAgentsSkills: that function's other
// two callers, Skills and skillSubtree, both promise exactly the skills and
// nothing else — Skills feeds §5.4's scope-rewrite, which only ever touches
// a skill's own state.toml, never the bucket's — so the bucket node is
// visited here, in Walk alone, instead.
func visitSkillsBucket(skillsPath string, visit func(Node) error) error {
	if !fileExists(filepath.Join(skillsPath, ".para", "state.toml")) {
		return nil
	}
	node, ok, err := classify(locator.Locator{"skills"}, skillsPath)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return visit(node)
}

// walkAgentsSkills scans .agents/skills/para-* the way walkChildren scans a
// bucket: each para-prefixed directory holding .para/state.toml is a skill
// entity (§1.4). Skills are one level only (§1.3), so there is no further
// recursion.
func walkAgentsSkills(skillsDir string, visit func(Node) error) error {
	entries, err := readDirs(skillsDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		name := e.Name()
		id, ok := strings.CutPrefix(name, "para-")
		if !ok {
			continue
		}
		childPath := filepath.Join(skillsDir, name)
		if !fileExists(filepath.Join(childPath, ".para", "state.toml")) {
			continue
		}
		// The id is checked here for the same reason classify checks a bucket's
		// children: the second root is a root, not an exemption. A directory
		// called `para-projects` or `para-Foo` names no skill — the first
		// because §1.4 reserves the word, the second because it is not a legal
		// segment — and admitting it would have `add` refuse an id that `list`
		// then prints. Skipping it leaves the state.toml for doctor to classify
		// as `collision` or `misplaced`, which is what it is (§10).
		skillLoc := locator.Locator{"skills", id}
		if _, err := KindAt(skillLoc); err != nil {
			continue
		}
		node := Node{Locator: skillLoc, Path: childPath, Kind: kindmeta.KindSkill}
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}

// readDirs lists dir's walkable children: real subdirectories, never a
// symlink (§6.1, §21.2) — the rule that keeps a mirrored skill from ever
// manufacturing a phantom entity. A missing directory (e.g. no .agents/
// convention adopted yet) is not an error; it simply has no children.
func readDirs(dir string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	walkable := entries[:0]
	for _, e := range entries {
		if e.Type()&fs.ModeSymlink == 0 && e.IsDir() {
			walkable = append(walkable, e)
		}
	}
	return walkable, nil
}
