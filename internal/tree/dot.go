package tree

import (
	"path/filepath"
	"strings"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// ResolveDot implements "." (§14): it walks up from startDir to the nearest
// directory holding .para/state.toml — exactly the walk §8.5 defines — and
// converts that directory's path, relative to root, back into a locator.
// The root itself never satisfies the search (it holds tree.toml, not
// state.toml), so "." at the root is an error rather than resolving to
// nothing in particular.
func ResolveDot(root, startDir string) (locator.Locator, error) {
	loc, atRoot, err := resolveDot(root, startDir)
	if err != nil {
		return nil, err
	}
	if atRoot {
		return nil, paraerr.Newf(paraerr.KindNotFound, "no entity or container contains %q", startDir)
	}
	return loc, nil
}

// ResolveDotOrRoot is ResolveDot for a caller that treats the tree root
// itself as a legitimate answer (para-xbb) rather than a failure: "." still
// means "the thing containing the working directory" (root help), and at
// the root — where nothing else does — that thing is the tree. atRoot is
// true exactly when startDir's nearest container is the root; err still
// reports startDir being outside the tree entirely, which is a different
// failure `show .` has no fallback for.
func ResolveDotOrRoot(root, startDir string) (loc locator.Locator, atRoot bool, err error) {
	return resolveDot(root, startDir)
}

// resolveDot is the walk both entry points share: nearest first, root last,
// with the root's own outcome reported as a bool rather than folded into an
// error, so the two callers can disagree about whether it is one.
func resolveDot(root, startDir string) (locator.Locator, bool, error) {
	root = filepath.Clean(root)
	dir := filepath.Clean(startDir)

	for {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, false, paraerr.Newf(paraerr.KindNotFound, "%q is not inside tree root %q", startDir, root)
		}
		if rel != "." && fileExists(filepath.Join(dir, ".para", "state.toml")) {
			loc, err := locator.FromPath(filepath.ToSlash(rel))
			return loc, false, err
		}
		if dir == root {
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil, true, nil
}
