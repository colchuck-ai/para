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
// parallel root (§1.4's skills.<id> exception).
//
// visit is called once per container, entity, or archive stub, in a
// stable, lexical, depth-first order. Containers are visited too — the
// walk's job is classification, not presentation — so a caller that wants
// only entities (e.g. list, §16.2) filters IsContainer out itself.
func Walk(root string, visit func(Node) error) error {
	if err := walkChildren(root, nil, visit); err != nil {
		return err
	}
	return walkAgentsSkills(filepath.Join(root, ".agents", "skills"), visit)
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
		case inArchive && !locator.IsReserved(name):
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
	if _, err := kindmeta.KindOf(loc); err != nil {
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

// classify derives a found directory's Node from its locator alone, per
// §1.2: a container's name is always a reserved word, and a reserved word
// can never be an id, so the test is exact. ok is false for a locator
// kindmeta cannot derive a kind for — malformed placements are doctor's
// job (a later phase), not the walk's, so Walk simply does not visit them.
func classify(loc locator.Locator, path string) (Node, bool, error) {
	name := loc[len(loc)-1]
	if locator.IsReserved(name) {
		return Node{
			Locator:     loc,
			Path:        path,
			Kind:        kindmeta.KindContainer,
			IsContainer: true,
			Archived:    loc.IsArchived(),
		}, true, nil
	}

	info, err := kindmeta.KindOf(loc)
	if err != nil {
		return Node{}, false, nil
	}
	return Node{Locator: loc, Path: path, Kind: info.Kind, Archived: info.Archived}, true, nil
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
		node := Node{Locator: locator.Locator{"skills", id}, Path: childPath, Kind: kindmeta.KindSkill}
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
