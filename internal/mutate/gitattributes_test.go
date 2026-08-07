package mutate_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/ptoml"
)

// TestConfigSetEmitGitattributesOffRemovesTheBlock: a command that finishes
// successfully must not leave doctor red. Turning the key off shortens the file
// in the same command, exactly as turning `emit.claude` off removes the eight
// CLAUDE.md files in the command that turns it off.
func TestConfigSetEmitGitattributesOffRemovesTheBlock(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)

	res := setClaude(t, e, root, "emit.gitattributes", ptoml.Bool(false))

	if !slices.Contains(res.Wrote, ".gitattributes") {
		t.Errorf("wrote %v, want .gitattributes among them", res.Wrote)
	}
	if slices.Contains(res.Removed, ".gitattributes") {
		t.Errorf("removed %v; para owns the block, not the file", res.Removed)
	}
	if got := read(t, root, ".gitattributes"); got != "" {
		t.Errorf(".gitattributes = %q, want it emptied", got)
	}
	assertClean(t, root)
}

// TestConfigSetEmitGitattributesBackOnRestoresIt is the other direction, and it
// is the one that would break if the write path only knew how to remove: the
// file is empty when the key comes back on, and §9's block has to go back into
// it without a rebuild.
func TestConfigSetEmitGitattributesBackOnRestoresIt(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	setClaude(t, e, root, "emit.gitattributes", ptoml.Bool(false))

	res := setClaude(t, e, root, "emit.gitattributes", ptoml.Bool(true))

	if !slices.Contains(res.Wrote, ".gitattributes") {
		t.Errorf("wrote %v, want .gitattributes among them", res.Wrote)
	}
	if got := read(t, root, ".gitattributes"); !strings.Contains(got, mdfile.HashMarkers.Begin) {
		t.Errorf(".gitattributes = %q, want para's block back", got)
	}
	assertClean(t, root)
}

// TestConfigSetEmitGitattributesKeepsTheRepositorysLines: whatever else is in
// the file is not para's to take, in either direction (§2.2, §9).
func TestConfigSetEmitGitattributesKeepsTheRepositorysLines(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	writeFiles(t, root, map[string]string{
		".gitattributes": "*.png binary\n" + read(t, root, ".gitattributes"),
	})

	setClaude(t, e, root, "emit.gitattributes", ptoml.Bool(false))

	if got, want := read(t, root, ".gitattributes"), "*.png binary\n"; got != want {
		t.Errorf(".gitattributes = %q, want %q", got, want)
	}
	assertClean(t, root)
}

// TestConfigSetOfAnotherKeyLeavesGitAttributesAlone: the refresh is scoped to
// the key that decides the file. A `config set` of anything else writes the
// config and the level's ACTIVITY.md, and nothing at the root.
func TestConfigSetOfAnotherKeyLeavesGitAttributesAlone(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)

	res := setClaude(t, e, root, "project.stale-after", ptoml.Int64(30))

	if slices.Contains(res.Wrote, ".gitattributes") {
		t.Errorf("wrote %v, want .gitattributes untouched", res.Wrote)
	}
	assertClean(t, root)
}

// TestConfigSetEmitGitattributesOnASkillStillSyncsTheMirror is the regression
// test for the defect Phase 13's review found once and Phase 14 reintroduced for
// one more key: `ConfigChange`'s per-key branch returned instead of falling
// through, so the "any mutation whose subject is a skill" gate never ran and a
// copy-mode mirror was left holding the skill's old ACTIVITY.md.
func TestConfigSetEmitGitattributesOnASkillStillSyncsTheMirror(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	setClaude(t, e, root, "emit.claude", ptoml.Bool(true))
	setClaude(t, e, root, "emit.claude-skills", ptoml.String("copy"))
	assertClean(t, root)

	// The key is read at the root, so setting it at a skill changes nothing
	// about .gitattributes — but it does write the skill's ACTIVITY.md, which is
	// what the mirror holds a copy of.
	skill := loc(t, "skills.report")
	path := filepath.Join(root, ".para", "config.toml")
	f, err := config.Read(path)
	if err != nil {
		t.Fatalf("config.Read: %v", err)
	}
	if _, err := f.Set(config.KeyReviewCadence, ptoml.Int64(90)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if _, err := e.ConfigChange(skill, config.KeyEmitGitattributes, "true", "false", data); err != nil {
		t.Fatalf("ConfigChange: %v", err)
	}

	assertClean(t, root)
}
