// Package tree implements discovery, the walk, locator resolution, and
// placement legality against a real filesystem tree (design §1, §8).
package tree

import (
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Find discovers the tree root by walking up from startDir looking for
// .para/tree.toml, the way git finds .git (§1.1, §8.1). Looking specifically
// for tree.toml — rather than the outermost .para/ — is what lets discovery
// succeed from inside a skill: the .para/ chain is deliberately broken under
// .agents/, so a skill's own .para/state.toml must never be mistaken for the
// root marker.
//
// $PARA_HOME overrides the walk entirely: when set, it must itself hold
// .para/tree.toml or Find reports an error naming it.
func Find(startDir string) (string, error) {
	if home := os.Getenv("PARA_HOME"); home != "" {
		home, err := filepath.Abs(home)
		if err != nil {
			return "", paraerr.Wrap(paraerr.KindInternal, err, "resolving PARA_HOME")
		}
		if !hasTreeMarker(home) {
			return "", paraerr.Newf(paraerr.KindNotFound, "PARA_HOME=%q has no .para/tree.toml", home)
		}
		return home, nil
	}

	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", paraerr.Wrap(paraerr.KindInternal, err, "resolving start directory")
	}
	for {
		if hasTreeMarker(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", paraerr.Newf(paraerr.KindNotFound, "no para tree found above %q (looked for .para/tree.toml)", startDir)
		}
		dir = parent
	}
}

func hasTreeMarker(dir string) bool {
	return fileExists(filepath.Join(dir, ".para", "tree.toml"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
