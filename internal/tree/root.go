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

	root, ok, err := Enclosing(startDir)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", paraerr.Newf(paraerr.KindNotFound, "no para tree found above %q (looked for .para/tree.toml)", startDir)
	}
	return root, nil
}

// Enclosing is Find's walk without $PARA_HOME and without the error: the tree
// root at or above dir, and whether there is one.
//
// It exists because `init` asks a different question from every other command.
// Find answers "where do I operate", for which $PARA_HOME is an override and
// finding nothing is a failure; Enclosing answers "is this path already inside
// a tree", for which the override is beside the point — a `para init foo` run
// with $PARA_HOME pointing somewhere else is not creating a tree inside that
// one — and finding nothing is the answer that lets the command proceed (§1.1).
//
// dir need not exist. The walk is over path components, so `init` can ask about
// the directory it is about to create.
func Enclosing(dir string) (string, bool, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false, paraerr.Wrap(paraerr.KindInternal, err, "resolving start directory")
	}
	for {
		if hasTreeMarker(abs) {
			return abs, true, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false, nil
		}
		abs = parent
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
