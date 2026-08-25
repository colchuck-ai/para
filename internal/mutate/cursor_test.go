package mutate_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/rebuild"
)

// TestConfigSetEmitCursorWritesTheRuleAndMirror is §6.2's own transcript,
// mirrored against §26's for Claude: turning the flag on writes
// .para/config.toml, the skill's Cursor rule file, and a mirror link — in one
// command, without a rebuild.
func TestConfigSetEmitCursorWritesTheRuleAndMirror(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)

	res := setClaude(t, e, root, config.KeyEmitCursor, ptoml.Bool(true))

	if !slices.Contains(res.Wrote, ".para/config.toml") {
		t.Errorf("Wrote = %v, want the config file", res.Wrote)
	}
	if !slices.Contains(res.Wrote, ".cursor/rules/para-report.mdc") {
		t.Errorf("Wrote = %v, want the Cursor rule file", res.Wrote)
	}
	if !lstatExists(root, ".cursor/rules/para-report.mdc") {
		t.Error(".cursor/rules/para-report.mdc was not written")
	}
	want := mirror.Change{
		Verb:   mirror.VerbLinked,
		Path:   ".cursor/skills/para-report",
		Target: "../../.agents/skills/para-report",
	}
	if !slices.Equal(res.Mirror, []mirror.Change{want}) {
		t.Errorf("Mirror = %v, want %v", res.Mirror, []mirror.Change{want})
	}
	assertClean(t, root)
}

// TestConfigSetEmitCursorDryRunPreviewsWithoutWriting is
// TestConfigSetEmitClaudeDryRunPreviewsTheWholeSurfaceWithoutWriting's Cursor
// counterpart: the fresh resolver behind refreshCursorSurface must answer
// `config set --dry-run`'s question against the pending value, not the one
// still on disk (para-ato).
func TestConfigSetEmitCursorDryRunPreviewsWithoutWriting(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	before := snapshot(t, root)

	dry := setClaudeDryRun(t, e, root, config.KeyEmitCursor, ptoml.Bool(true))
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("the dry run wrote %v, want nothing", changed)
	}

	real := setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))
	assertEqual(t, "dry run's Wrote", dry.Wrote, real.Wrote)
	if !slices.Equal(dry.Mirror, real.Mirror) {
		t.Errorf("dry run's Mirror = %v, want %v", dry.Mirror, real.Mirror)
	}
}

// TestConfigSetEmitCursorOffSweepsTheSurface is the same command in reverse:
// the rule file and the mirror link both come out, and — unlike CLAUDE.md's
// shared file — the rule file is wholly para's, so it is removed outright
// rather than shortened (R6).
func TestConfigSetEmitCursorOffSweepsTheSurface(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	setClaude(t, e, root, config.KeyEmitCursor, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(false))

	if !slices.Contains(res.Removed, ".cursor/rules/para-report.mdc") {
		t.Errorf("Removed = %v, want the Cursor rule file", res.Removed)
	}
	if lstatExists(root, ".cursor/rules/para-report.mdc") {
		t.Error(".cursor/rules/para-report.mdc survived")
	}
	if want := (mirror.Change{Verb: mirror.VerbRemoved, Path: ".cursor/skills/para-report"}); !slices.Contains(res.Mirror, want) {
		t.Errorf("Mirror = %v, want %v", res.Mirror, want)
	}
	if lstatExists(root, ".cursor") {
		t.Error(".cursor/ survived")
	}
	assertClean(t, root)
}

// TestConfigSetCursorMirrorModeSwitchesInPlace: switching emit.cursor-skills
// is a config change plus a rebuild §6.2 does not need, the same property
// TestConfigSetMirrorModeSwitchesInPlace holds for Claude.
func TestConfigSetCursorMirrorModeSwitchesInPlace(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, config.KeyEmitCursorSkills, ptoml.String("copy"))

	if want := (mirror.Change{Verb: mirror.VerbCopied, Path: ".cursor/skills/para-report"}); !slices.Equal(res.Mirror, []mirror.Change{want}) {
		t.Fatalf("Mirror = %v, want %v", res.Mirror, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor", "skills", "para-report", "SKILL.md")); err != nil {
		t.Errorf("the copy has no SKILL.md: %v", err)
	}
	assertClean(t, root)
}

// TestConfigSetOfAnotherKeyLeavesTheCursorSurfaceAlone: the refresh is
// triggered by the two keys that decide the Cursor surface, not by every
// config write.
func TestConfigSetOfAnotherKeyLeavesTheCursorSurfaceAlone(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, "project.stale-after", ptoml.Int64(30))

	if len(res.Mirror) != 0 || len(res.Removed) != 0 {
		t.Errorf("an unrelated key touched the Cursor surface: %v / %v", res.Mirror, res.Removed)
	}
	if slices.Contains(res.Wrote, ".cursor/rules/para-report.mdc") {
		t.Error("an unrelated key rewrote the Cursor rule file")
	}
}

// TestAddingASkillWritesItsCursorRule is
// TestAddingASkillKeepsEveryClaudeMdCorrect's Cursor counterpart: a new
// skill's rule file is not a shared pointer file with an import list to keep
// current, but it still owes the new skill a rule file and a mirror entry the
// moment it exists.
func TestAddingASkillWritesItsCursorRule(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))

	res, err := env(t, root).Add(loc(t, "skills.commit-style"),
		fields("name", "Commit style", "description", "when writing a commit message"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !slices.Contains(res.Wrote, ".cursor/rules/para-commit-style.mdc") {
		t.Errorf("Wrote = %v, want the new skill's Cursor rule file", res.Wrote)
	}
	if !slices.Contains(res.Mirror, mirror.Change{
		Verb: mirror.VerbLinked, Path: ".cursor/skills/para-commit-style",
		Target: "../../.agents/skills/para-commit-style",
	}) {
		t.Errorf("Mirror = %v, want a link for the new skill", res.Mirror)
	}
	assertClean(t, root)
}

// TestAddDryRunOnASkillWithCursorSurfaceOnPreviewsTheReach is
// TestAddDryRunOnASkillWithClaudeSurfaceOnPreviewsTheWholeReach's Cursor
// counterpart: a rehearsed add must report the same rule file and mirror
// link the immediately following real add produces, even though the skill it
// names does not exist on disk yet.
func TestAddDryRunOnASkillWithCursorSurfaceOnPreviewsTheReach(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))
	before := snapshot(t, root)

	f := fields("name", "Commit style", "description", "when writing a commit message")
	dry, err := env(t, root).AddDryRun(loc(t, "skills.commit-style"), f)
	if err != nil {
		t.Fatalf("AddDryRun: %v", err)
	}
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("AddDryRun wrote %v, want nothing", changed)
	}

	real, err := env(t, root).Add(loc(t, "skills.commit-style"), f)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertEqual(t, "AddDryRun's Wrote", dry.Wrote, real.Wrote)
	if !slices.Equal(dry.Mirror, real.Mirror) {
		t.Errorf("AddDryRun's Mirror = %v, want %v", dry.Mirror, real.Mirror)
	}
	if !slices.Contains(dry.Wrote, ".cursor/rules/para-commit-style.mdc") {
		t.Errorf("AddDryRun's Wrote = %v, want the Cursor rule file", dry.Wrote)
	}
}

// TestRemovingASkillPrunesItsCursorMirror is
// TestRemovingASkillPrunesItsMirror's Cursor counterpart (§18.2, §6.2).
func TestRemovingASkillPrunesItsCursorMirror(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))

	e := env(t, root)
	plan, err := e.PlanRemove(loc(t, "skills.report"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	res, err := plan.Apply(false)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if lstatExists(root, ".cursor/skills/para-report") {
		t.Error("the Cursor mirror of a removed skill survived")
	}
	if !slices.Contains(res.Mirror, (mirror.Change{Verb: mirror.VerbRemoved, Path: ".cursor/skills/para-report"})) {
		t.Errorf("Mirror = %v, want the Cursor mirror removed", res.Mirror)
	}
	if lstatExists(root, ".cursor/rules/para-report.mdc") {
		t.Error("the removed skill's Cursor rule file survived")
	}
	assertClean(t, root)
}

// TestEditingASkillRefreshesACopiedCursorMirror: in copy mode the Cursor
// mirror holds the skill's own files too, so any mutation of the skill
// leaves it drifted exactly as the Claude one does (§6.2, §6.1).
func TestEditingASkillRefreshesACopiedCursorMirror(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))
	setClaude(t, env(t, root), root, config.KeyEmitCursorSkills, ptoml.String("copy"))

	if _, err := env(t, root).Set(loc(t, "skills.report"),
		fields("description", "when asked for the weekly number"), ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, ".cursor", "skills", "para-report", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "weekly number") {
		t.Errorf("the copied SKILL.md =\n%s\nwant the edit carried through", got)
	}
	assertClean(t, root)
}

// TestSkillMutationOnATreeWithTheCursorSurfaceOffWritesNothingExtra: the
// refresh must not manufacture a surface nobody asked for, the same "off by
// default" property §6.1's own test holds.
func TestSkillMutationOnATreeWithTheCursorSurfaceOffWritesNothingExtra(t *testing.T) {
	root := treeWithSkill(t)

	res, err := env(t, root).Note(loc(t, "skills.report"), "still useful", "", false)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}

	for _, path := range res.Wrote {
		if strings.HasSuffix(path, ".mdc") {
			t.Errorf("a skill mutation wrote %s with emit.cursor off", path)
		}
	}
	if lstatExists(root, ".cursor") {
		t.Error(".cursor/ was created with emit.cursor off")
	}
}

// TestBothSurfacesRefreshTogetherWhenBothAreOn is §6.2's independence claim
// made concrete: emit.claude and emit.cursor are two separate concerns that
// may both be on, and a single skill mutation must refresh both rather than
// whichever syncSurface asks about first.
func TestBothSurfacesRefreshTogetherWhenBothAreOn(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))

	res, err := env(t, root).Add(loc(t, "skills.commit-style"),
		fields("name", "Commit style", "description", "when writing a commit message"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	for _, rel := range claudeLocations {
		if !slices.Contains(res.Wrote, rel) {
			t.Errorf("Wrote = %v, want it to carry %s", res.Wrote, rel)
		}
	}
	if !slices.Contains(res.Wrote, ".cursor/rules/para-commit-style.mdc") {
		t.Errorf("Wrote = %v, want the Cursor rule file", res.Wrote)
	}
	if !slices.Contains(res.Mirror, mirror.Change{
		Verb: mirror.VerbLinked, Path: ".claude/skills/para-commit-style",
		Target: "../../.agents/skills/para-commit-style",
	}) {
		t.Errorf("Mirror = %v, want a Claude mirror link", res.Mirror)
	}
	if !slices.Contains(res.Mirror, mirror.Change{
		Verb: mirror.VerbLinked, Path: ".cursor/skills/para-commit-style",
		Target: "../../.agents/skills/para-commit-style",
	}) {
		t.Errorf("Mirror = %v, want a Cursor mirror link", res.Mirror)
	}
	assertClean(t, root)
}

// TestCursorSurfaceRefreshLeavesRebuildNothingToDo is the property every
// write-path change has to keep: write-through is complete, so rebuild after
// it is a no-op (§2.3, §2.4) — the same test claude_test.go runs for the
// Claude surface, run here against the Cursor one.
func TestCursorSurfaceRefreshLeavesRebuildNothingToDo(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitCursor, ptoml.Bool(true))
	if _, err := env(t, root).Add(loc(t, "skills.commit-style"),
		fields("name", "Commit style", "description", "when writing a commit message")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	res, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !res.Empty() {
		t.Errorf("rebuild after the refresh did %v / %v / %v, want nothing", res.Changed, res.Removed, res.Mirror)
	}
}
