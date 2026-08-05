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
	root = filepath.Clean(root)
	dir := filepath.Clean(startDir)

	for {
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, paraerr.Newf(paraerr.KindNotFound, "%q is not inside tree root %q", startDir, root)
		}
		if rel != "." && fileExists(filepath.Join(dir, ".para", "state.toml")) {
			return locator.FromPath(filepath.ToSlash(rel))
		}
		if dir == root {
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil, paraerr.Newf(paraerr.KindNotFound, "no entity or container contains %q", startDir)
}
