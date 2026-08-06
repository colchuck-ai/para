package tree

import (
	"path/filepath"

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
// reserved word can never be an id (§1.2), so the last segment decides, and
// KindOf — which only ever sees id positions — never returns KindContainer.
//
// The empty locator is the tree root, which is not an entity but the tree
// (§8.1), and so has no kind: it reports KindUnknown and no error. Every caller
// that can be handed a root already has to branch on it, because the root's
// identity lives in tree.toml rather than state.toml.
func KindAt(loc locator.Locator) (kindmeta.Kind, error) {
	if len(loc) == 0 {
		return kindmeta.KindUnknown, nil
	}
	if locator.IsReserved(loc[len(loc)-1]) {
		return kindmeta.KindContainer, nil
	}
	info, err := kindmeta.KindOf(loc)
	if err != nil {
		return kindmeta.KindUnknown, err
	}
	return info.Kind, nil
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
// which is shared with non-para content and so never carries its own
// .para/state.toml. Existence of the directory itself is the whole check.
func ParentExists(root string, loc locator.Locator) (bool, error) {
	if len(loc) == 0 {
		return false, paraerr.New(paraerr.KindValidation, "empty locator has no parent")
	}
	if loc.Bucket() == "skills" {
		return dirExists(filepath.Join(root, ".agents", "skills")), nil
	}

	parentRel, err := loc[:len(loc)-1].Path()
	if err != nil {
		return false, err
	}
	parentAbs := filepath.Join(root, filepath.FromSlash(parentRel))
	return fileExists(filepath.Join(parentAbs, ".para", "state.toml")), nil
}
