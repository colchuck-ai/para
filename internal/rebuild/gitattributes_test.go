package rebuild_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/rebuild"
)

// gitattributesOff writes the config that turns §9's block off.
func gitattributesOff(t *testing.T, root string) {
	t.Helper()
	write(t, root, ".para/config.toml", "emit.gitattributes = false\n")
}

// TestRebuildRemovesTheBlockWhenTheKeyGoesOff is Phase 13's carried-forward
// debt: `emit.gitattributes = false` used to leave para's block in a file
// nothing removed and nothing reported.
//
// The repair is a shorter file rather than a deleted one, because para owns the
// block and the repository owns every other line (§2.2, §9).
func TestRebuildRemovesTheBlockWhenTheKeyGoesOff(t *testing.T) {
	root := plantTree(t)
	run(t, root, rebuild.Options{})
	if !strings.Contains(read(t, root, ".gitattributes"), mdfile.HashMarkers.Begin) {
		t.Fatal("the planted tree has no .gitattributes block to remove")
	}
	write(t, root, ".gitattributes", "*.png binary\n"+read(t, root, ".gitattributes")+"*.md text\n")

	gitattributesOff(t, root)
	res := run(t, root, rebuild.Options{})

	if !slices.Contains(res.Changed, ".gitattributes") {
		t.Errorf("rebuild did not rewrite .gitattributes; wrote %v", res.Changed)
	}
	if slices.Contains(res.Removed, ".gitattributes") {
		t.Error("rebuild removed .gitattributes; it owns the block, not the file")
	}
	if got, want := read(t, root, ".gitattributes"), "*.png binary\n*.md text\n"; got != want {
		t.Errorf(".gitattributes = %q, want %q", got, want)
	}
}

// TestRebuildWithTheKeyOffLeavesAnEmptyFile is the case where para wrote the
// whole file: what is left is what was there before para, which is nothing.
// Deleting it would be para taking a file it cannot prove it created.
func TestRebuildWithTheKeyOffLeavesAnEmptyFile(t *testing.T) {
	root := plantTree(t)
	run(t, root, rebuild.Options{})

	gitattributesOff(t, root)
	run(t, root, rebuild.Options{})

	if got := read(t, root, ".gitattributes"); got != "" {
		t.Errorf(".gitattributes = %q, want it emptied", got)
	}
}

// TestRebuildWithTheKeyOffIsIdempotent: once the block is gone there is nothing
// left to remove, so the second pass writes nothing. Without this, `doctor`
// would report drift on a tree `rebuild` had just finished with.
func TestRebuildWithTheKeyOffIsIdempotent(t *testing.T) {
	root := plantTree(t)
	run(t, root, rebuild.Options{})
	gitattributesOff(t, root)
	run(t, root, rebuild.Options{})

	res := run(t, root, rebuild.Options{})

	if slices.Contains(res.Changed, ".gitattributes") || slices.Contains(res.Removed, ".gitattributes") {
		t.Errorf("a second rebuild touched .gitattributes: changed %v, removed %v", res.Changed, res.Removed)
	}
}

// TestRebuildWithTheKeyOffCreatesNoFile: a tree that never had a .gitattributes
// does not get an empty one just because the key is off.
func TestRebuildWithTheKeyOffCreatesNoFile(t *testing.T) {
	root := plantTree(t)
	gitattributesOff(t, root)

	res := run(t, root, rebuild.Options{})

	if slices.Contains(res.Changed, ".gitattributes") {
		t.Errorf("rebuild wrote .gitattributes with emit.gitattributes off; wrote %v", res.Changed)
	}
	if exists(t, root, ".gitattributes") {
		t.Error(".gitattributes exists after a rebuild that should not have created one")
	}
}

// TestRebuildLeavesAForeignGitAttributesAlone: a repository's own file, with no
// para block in it, is not para's to shorten.
func TestRebuildLeavesAForeignGitAttributesAlone(t *testing.T) {
	root := plantTree(t)
	gitattributesOff(t, root)
	write(t, root, ".gitattributes", "*.png binary\n")

	res := run(t, root, rebuild.Options{})

	if slices.Contains(res.Changed, ".gitattributes") {
		t.Errorf("rebuild rewrote a .gitattributes it did not write into; wrote %v", res.Changed)
	}
	if got, want := read(t, root, ".gitattributes"), "*.png binary\n"; got != want {
		t.Errorf(".gitattributes = %q, want %q", got, want)
	}
}

// TestRebuildDryRunReportsTheShortening: §21.1's dry run lists what a run would
// do, and this is a rewrite rather than a removal in both.
func TestRebuildDryRunReportsTheShortening(t *testing.T) {
	root := plantTree(t)
	run(t, root, rebuild.Options{})
	gitattributesOff(t, root)

	res := run(t, root, rebuild.Options{DryRun: true})

	if !slices.Contains(res.Changed, ".gitattributes") {
		t.Errorf("dry run did not report .gitattributes; would write %v", res.Changed)
	}
	if !strings.Contains(read(t, root, ".gitattributes"), mdfile.HashMarkers.Begin) {
		t.Error("a dry run removed the block")
	}
}
