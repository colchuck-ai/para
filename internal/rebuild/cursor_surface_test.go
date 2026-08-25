package rebuild_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/rebuild"
)

func cursorOn(t *testing.T, root string) {
	t.Helper()
	write(t, root, ".para/config.toml", "emit.cursor = true\n")
}

// TestRebuildWritesTheWholeCursorSurface is bo5.6's first task: a skill's
// derived rule file and a mirror entry, in one pass — the Cursor counterpart
// of TestRebuildWritesTheWholeClaudeSurface.
func TestRebuildWritesTheWholeCursorSurface(t *testing.T) {
	root := plantTree(t)
	cursorOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")

	res := run(t, root, rebuild.Options{})

	if !slices.Contains(res.Changed, ".cursor/rules/para-signups-report.mdc") {
		t.Errorf("rebuild did not write the rule file; wrote %v", res.Changed)
	}
	got := read(t, root, ".cursor/rules/para-signups-report.mdc")
	if !strings.Contains(got, "alwaysApply: true") {
		t.Errorf(".mdc = %q, want alwaysApply: true for a skill with no scope", got)
	}
	if !strings.Contains(got, `generated_from: "para-signups-report"`) {
		t.Errorf(".mdc = %q, want a generated_from line naming the skill", got)
	}
	if !strings.Contains(got, ".cursor/skills/para-signups-report/SKILL.md") {
		t.Errorf(".mdc = %q, want it to point at the mirrored skill's SKILL.md", got)
	}

	target, err := os.Readlink(filepath.Join(root, ".cursor", "skills", "para-signups-report"))
	if err != nil {
		t.Fatalf("no symlink-mode Cursor mirror after a rebuild: %v", err)
	}
	wantTarget := "../../.agents/skills/para-signups-report"
	if filepath.ToSlash(target) != wantTarget {
		t.Errorf("mirror target = %q, want %q", target, wantTarget)
	}
	want := mirror.Change{Verb: mirror.VerbLinked, Path: ".cursor/skills/para-signups-report", Target: wantTarget}
	if !slices.Contains(res.Mirror, want) {
		t.Errorf("Result.Mirror = %v, want it to carry %v", res.Mirror, want)
	}

	if second := run(t, root, rebuild.Options{}); len(second.Changed) != 0 || len(second.Mirror) != 0 {
		t.Errorf("a second pass changed %v and %v, want nothing", second.Changed, second.Mirror)
	}
}

// TestRebuildSweepsTheCursorSurfaceWhenItIsTurnedOff mirrors
// TestRebuildSweepsTheSurfaceWhenItIsTurnedOff for §6.2: with emit.cursor off,
// both the rule file and the mirror are residue, and rebuild removes them,
// leaving .cursor/ behind for nothing.
func TestRebuildSweepsTheCursorSurfaceWhenItIsTurnedOff(t *testing.T) {
	root := plantTree(t)
	cursorOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".para/config.toml", "emit.cursor = false\n")
	res := run(t, root, rebuild.Options{})

	if !slices.Contains(res.Removed, ".cursor/rules/para-signups-report.mdc") {
		t.Errorf("rebuild did not remove the rule file; removed %v", res.Removed)
	}
	want := mirror.Change{Verb: mirror.VerbRemoved, Path: ".cursor/skills/para-signups-report"}
	if !slices.Contains(res.Mirror, want) {
		t.Errorf("Result.Mirror = %v, want it to carry %v", res.Mirror, want)
	}
	if exists(t, root, ".cursor") {
		t.Error(".cursor/ survived the sweep")
	}
	if second := run(t, root, rebuild.Options{}); len(second.Changed) != 0 || len(second.Removed) != 0 || len(second.Mirror) != 0 {
		t.Errorf("a second sweep did %v / %v / %v, want nothing", second.Changed, second.Removed, second.Mirror)
	}
}

// TestRebuildSwitchesCursorMirrorModesWithoutResidue is the mode-switch half
// of bo5.6's acceptance criteria, for the Cursor mirror rather than Claude's.
func TestRebuildSwitchesCursorMirrorModesWithoutResidue(t *testing.T) {
	root := plantTree(t)
	cursorOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".para/config.toml", "emit.cursor = true\nemit.cursor-skills = \"copy\"\n")
	res := run(t, root, rebuild.Options{})

	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbCopied {
		t.Fatalf("switching to copy = %v, want one copy", res.Mirror)
	}
	dir := filepath.Join(root, ".cursor", "skills", "para-signups-report")
	if _, err := os.Readlink(dir); err == nil {
		t.Error("the symlink survived the switch to copy mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("copy mode did not write SKILL.md: %v", err)
	}
	if second := run(t, root, rebuild.Options{}); len(second.Mirror) != 0 {
		t.Errorf("a second pass in copy mode did %v, want nothing", second.Mirror)
	}
}

// TestBothCompatibilitySurfacesCoexist is §6.2's "independent of emit.claude —
// both may be on together" as a rebuild behavior, not just a config default:
// turning both surfaces on writes both trees in one pass, and turning one off
// leaves the other untouched.
func TestBothCompatibilitySurfacesCoexist(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "emit.claude = true\nemit.cursor = true\n")
	plantSkill(t, root, "signups-report", "Signups report")

	res := run(t, root, rebuild.Options{})
	if !slices.Contains(res.Changed, "CLAUDE.md") {
		t.Errorf("both-on rebuild did not write CLAUDE.md; wrote %v", res.Changed)
	}
	if !slices.Contains(res.Changed, ".cursor/rules/para-signups-report.mdc") {
		t.Errorf("both-on rebuild did not write the Cursor rule; wrote %v", res.Changed)
	}
	if !exists(t, root, ".claude/skills/para-signups-report") {
		t.Error("the Claude mirror is missing")
	}
	if !exists(t, root, ".cursor/skills/para-signups-report") {
		t.Error("the Cursor mirror is missing")
	}

	write(t, root, ".para/config.toml", "emit.claude = false\nemit.cursor = true\n")
	res = run(t, root, rebuild.Options{})
	if !slices.Contains(res.Removed, "CLAUDE.md") {
		t.Errorf("turning off emit.claude did not remove CLAUDE.md; removed %v", res.Removed)
	}
	if exists(t, root, ".claude") {
		t.Error(".claude/ survived turning emit.claude off")
	}
	if !exists(t, root, ".cursor/rules/para-signups-report.mdc") {
		t.Error("turning off emit.claude took the Cursor rule with it")
	}
	if !exists(t, root, ".cursor/skills/para-signups-report") {
		t.Error("turning off emit.claude took the Cursor mirror with it")
	}
}

// TestScopedRebuildAtASkillSyncsItsCursorMirror mirrors
// TestScopedRebuildAtASkillSyncsItsMirror: the Cursor mirror is a projection
// of the skills that exist rather than of any one entity's subtree, but a
// scope that reaches a skill still has to leave it synced.
func TestScopedRebuildAtASkillSyncsItsCursorMirror(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "emit.cursor = true\nemit.cursor-skills = \"copy\"\n")
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".agents/skills/para-signups-report/.para/state.toml",
		"name = \"Weekly signups\"\ndescription = \"when signups\"\ncreated = \"2026-01-01T00:00:00Z\"\n")

	res, err := rebuild.Run(env(t, root), rebuild.Options{Scope: loc(t, "skills.signups-report")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbCopied {
		t.Fatalf("a scoped rebuild at a skill did %v to the Cursor mirror, want one copy", res.Mirror)
	}
	got, err := os.ReadFile(filepath.Join(root, ".cursor", "skills", "para-signups-report", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Weekly signups") {
		t.Errorf("the Cursor mirror still holds the old SKILL.md:\n%s", got)
	}
	if after := run(t, root, rebuild.Options{}); !after.Empty() {
		t.Errorf("an unscoped rebuild after the scoped one did %v / %v / %v", after.Changed, after.Removed, after.Mirror)
	}
}

// TestCursorSurfaceIsTheConstantCostRefresh is what a skill mutation uses
// (bo5.7): one rule file per skill that exists, derived from that skill's own
// truth rather than a whole-tree walk.
func TestCursorSurfaceIsTheConstantCostRefresh(t *testing.T) {
	root := plantTree(t)
	cursorOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	plantSkill(t, root, "commit-style", "Commit style")

	artifacts, err := env(t, root).CursorSurface()
	if err != nil {
		t.Fatalf("CursorSurface: %v", err)
	}
	var got []string
	for _, a := range artifacts {
		got = append(got, a.Path)
		if !a.Wanted {
			t.Errorf("%s is not wanted with emit.cursor on", a.Path)
		}
	}
	want := []string{".cursor/rules/para-commit-style.mdc", ".cursor/rules/para-signups-report.mdc"}
	if !slices.Equal(got, want) {
		t.Fatalf("CursorSurface = %v, want %v", got, want)
	}
}

// TestCursorSurfaceResidueWithTheKeyOff is WriteCursorRules' off branch used
// on its own, the way `config set emit.cursor false` (bo5.7) would call it
// without a full rebuild.
func TestCursorSurfaceResidueWithTheKeyOff(t *testing.T) {
	root := plantTree(t)
	cursorOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".para/config.toml", "emit.cursor = false\n")
	wrote, removed, err := env(t, root).WriteCursorRules(false)
	if err != nil {
		t.Fatalf("WriteCursorRules: %v", err)
	}
	if len(wrote) != 0 {
		t.Errorf("WriteCursorRules wrote %v with the key off, want nothing", wrote)
	}
	if !slices.Contains(removed, ".cursor/rules/para-signups-report.mdc") {
		t.Errorf("WriteCursorRules removed %v, want the rule file gone", removed)
	}
	if exists(t, root, ".cursor/rules/para-signups-report.mdc") {
		t.Error("the rule file survived WriteCursorRules with the key off")
	}
}
