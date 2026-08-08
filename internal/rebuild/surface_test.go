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

// claudeLocations is the eight places CLAUDE.md is emitted (§6), spelled out
// here rather than asked of render, so the test would notice the set changing.
var claudeLocations = []string{
	"CLAUDE.md",
	"projects/CLAUDE.md",
	"areas/CLAUDE.md",
	"resources/CLAUDE.md",
	"archive/CLAUDE.md",
	"archive/projects/CLAUDE.md",
	"archive/areas/CLAUDE.md",
	"archive/resources/CLAUDE.md",
}

// plantSkill writes a skill as truth alone, the way plantTree writes the rest of
// the tree: rebuild is what turns it into files.
func plantSkill(t *testing.T, root, id, name string) {
	t.Helper()
	write(t, root, ".agents/skills/para-"+id+"/.para/state.toml",
		"name = \""+name+"\"\ndescription = \"when "+id+"\"\ncreated = \"2026-01-01T00:00:00Z\"\n")
}

func claudeOn(t *testing.T, root string) {
	t.Helper()
	write(t, root, ".para/config.toml", "emit.claude = true\n")
}

func exists(t *testing.T, root, rel string) bool {
	t.Helper()
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// TestRebuildWritesTheWholeClaudeSurface is Phase 13's first task: the eight
// CLAUDE.md pointer files and a mirror per skill, in one pass.
func TestRebuildWritesTheWholeClaudeSurface(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")

	res := run(t, root, rebuild.Options{})

	for _, rel := range claudeLocations {
		if !slices.Contains(res.Changed, rel) {
			t.Errorf("rebuild did not write %s; wrote %v", rel, res.Changed)
		}
	}
	target, err := os.Readlink(filepath.Join(root, ".claude", "skills", "para-signups-report"))
	if err != nil {
		t.Fatalf("no symlink-mode mirror after a rebuild: %v", err)
	}
	// ToSlash, because the link is written through filepath.FromSlash and
	// Windows reads it back as `..\..\.agents\skills\para-signups-report` —
	// the same link in that platform's separator. Change.Target below needs no
	// such conversion: it carries mirror.Target's own slash-separated string,
	// which never touches the filesystem.
	wantTarget := "../../.agents/skills/para-signups-report"
	if filepath.ToSlash(target) != wantTarget {
		t.Errorf("mirror target = %q, want %q", target, wantTarget)
	}
	want := mirror.Change{Verb: mirror.VerbLinked, Path: ".claude/skills/para-signups-report", Target: wantTarget}
	if !slices.Contains(res.Mirror, want) {
		t.Errorf("Result.Mirror = %v, want it to carry %v", res.Mirror, want)
	}

	if second := run(t, root, rebuild.Options{}); len(second.Changed) != 0 || len(second.Mirror) != 0 {
		t.Errorf("a second pass changed %v and %v, want nothing", second.Changed, second.Mirror)
	}
}

// TestRebuildSweepsTheSurfaceWhenItIsTurnedOff is Phase 13's task 3: a config
// change plus a rebuild "removes the old shape and writes the new one, leaving
// no residue" (§6.1). With the surface off, the new shape is nothing at all.
func TestRebuildSweepsTheSurfaceWhenItIsTurnedOff(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".para/config.toml", "emit.claude = false\n")
	res := run(t, root, rebuild.Options{})

	for _, rel := range claudeLocations {
		if !slices.Contains(res.Removed, rel) {
			t.Errorf("rebuild did not remove %s; removed %v", rel, res.Removed)
		}
		if exists(t, root, rel) {
			t.Errorf("%s survived the sweep", rel)
		}
	}
	if exists(t, root, ".claude") {
		t.Error(".claude/ survived the sweep")
	}
	if second := run(t, root, rebuild.Options{}); len(second.Changed) != 0 || len(second.Removed) != 0 {
		t.Errorf("a second sweep changed %v and removed %v, want nothing", second.Changed, second.Removed)
	}
}

// TestRebuildSwitchesMirrorModesWithoutResidue is the other half of task 3.
func TestRebuildSwitchesMirrorModesWithoutResidue(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".para/config.toml", "emit.claude = true\nemit.claude-skills = \"copy\"\n")
	res := run(t, root, rebuild.Options{})

	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbCopied {
		t.Fatalf("switching to copy = %v, want one copy", res.Mirror)
	}
	dir := filepath.Join(root, ".claude", "skills", "para-signups-report")
	if _, err := os.Readlink(dir); err == nil {
		t.Error("the symlink survived the switch to copy mode")
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("copy mode did not write SKILL.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".para")); !os.IsNotExist(err) {
		t.Error("copy mode mirrored .para/, which §6.1 says it never does")
	}
	if second := run(t, root, rebuild.Options{}); len(second.Mirror) != 0 {
		t.Errorf("a second pass in copy mode did %v, want nothing", second.Mirror)
	}
}

// TestRebuildPrunesTheMirrorOfASkillThatIsGone is task 4's repair half: the
// finding `doctor` calls `orphan-mirror` (§10).
func TestRebuildPrunesTheMirrorOfASkillThatIsGone(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	plantSkill(t, root, "commit-style", "Commit style")
	run(t, root, rebuild.Options{})

	if err := os.RemoveAll(filepath.Join(root, ".agents", "skills", "para-commit-style")); err != nil {
		t.Fatal(err)
	}
	res := run(t, root, rebuild.Options{})

	want := mirror.Change{Verb: mirror.VerbRemoved, Path: ".claude/skills/para-commit-style"}
	if !slices.Contains(res.Mirror, want) {
		t.Errorf("Result.Mirror = %v, want it to carry %v", res.Mirror, want)
	}
	if exists(t, root, ".claude/skills/para-commit-style") {
		t.Error("the orphaned mirror survived")
	}
	if !exists(t, root, ".claude/skills/para-signups-report") {
		t.Error("pruning one mirror took another with it")
	}
}

// TestRebuildRepairsAHostileCheckout is Phase 13's task 5: a
// core.symlinks=false checkout materialises each link as a plain file holding
// its target path, and `rebuild` is the repair (§6.1).
func TestRebuildRepairsAHostileCheckout(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	path := filepath.Join(root, ".claude", "skills", "para-signups-report")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("../../.agents/skills/para-signups-report"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := run(t, root, rebuild.Options{})
	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbLinked {
		t.Fatalf("Result.Mirror = %v, want one link", res.Mirror)
	}
	if _, err := os.Readlink(path); err != nil {
		t.Errorf("rebuild did not restore the link: %v", err)
	}
}

// TestRebuildDryRunTouchesNothing: --dry-run "lists what would change and
// changes nothing" (§21.1), and the surface is no exception.
func TestRebuildDryRunLeavesTheSurfaceAlone(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")

	res := run(t, root, rebuild.Options{DryRun: true})

	if !slices.Contains(res.Changed, "CLAUDE.md") {
		t.Errorf("--dry-run did not list CLAUDE.md; listed %v", res.Changed)
	}
	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbLinked {
		t.Fatalf("--dry-run listed %v, want the one link it would make", res.Mirror)
	}
	if exists(t, root, "CLAUDE.md") {
		t.Error("--dry-run wrote CLAUDE.md")
	}
	if exists(t, root, ".claude") {
		t.Error("--dry-run created .claude/")
	}
}

// TestRebuildScopedLeavesTheMirrorAlone: the mirror lives in no entity's
// subtree, so `rebuild projects` cannot reach it — the same rule that keeps a
// scoped `doctor` from reporting tree-wide artifacts (§5.3, §6.1).
func TestRebuildScopedLeavesTheMirrorAlone(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")

	res, err := rebuild.Run(env(t, root), rebuild.Options{Scope: loc(t, "projects")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Mirror) != 0 {
		t.Errorf("a scoped rebuild touched the mirror: %v", res.Mirror)
	}
	// It does still own the CLAUDE.md inside its scope, which is a file the
	// scope contains rather than a tree-wide artifact.
	if !slices.Contains(res.Changed, "projects/CLAUDE.md") {
		t.Errorf("`rebuild projects` did not write projects/CLAUDE.md; wrote %v", res.Changed)
	}
}

// TestDryRunPredictsTheMirrorWorkTheRealRunDoes is the property --dry-run
// exists for, and the one copy mode nearly broke: the mirror is synced *after*
// the subject loop, so a dry run comparing the copy against a SKILL.md it has
// not rewritten yet finds them equal and predicts less work than the run does.
func TestDryRunPredictsTheMirrorWorkTheRealRunDoes(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "emit.claude = true\nemit.claude-skills = \"copy\"\n")
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	// Truth changes, so the skill's own SKILL.md is stale — and so, in copy
	// mode, is the mirror that holds a copy of it.
	write(t, root, ".agents/skills/para-signups-report/.para/state.toml",
		"name = \"Weekly signups\"\ndescription = \"when signups\"\ncreated = \"2026-01-01T00:00:00Z\"\n")

	dry := run(t, root, rebuild.Options{DryRun: true})
	real := run(t, root, rebuild.Options{})

	if !slices.Equal(dry.Changed, real.Changed) {
		t.Errorf("--dry-run would rewrite %v; the run rewrote %v", dry.Changed, real.Changed)
	}
	if len(dry.Mirror) != len(real.Mirror) {
		t.Errorf("--dry-run predicted %v; the run did %v", dry.Mirror, real.Mirror)
	}
	for i := range dry.Mirror {
		if dry.Mirror[i].Path != real.Mirror[i].Path {
			t.Errorf("--dry-run predicted %v; the run did %v", dry.Mirror, real.Mirror)
		}
	}
}

// TestScopedRebuildAtASkillSyncsItsMirror: the mirror is not a tree-wide
// artifact when the scope is a skill — it is that skill's own projection — and
// skipping it left `rebuild skills.x` finishing successfully with `doctor` red.
func TestScopedRebuildAtASkillSyncsItsMirror(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "emit.claude = true\nemit.claude-skills = \"copy\"\n")
	plantSkill(t, root, "signups-report", "Signups report")
	run(t, root, rebuild.Options{})

	write(t, root, ".agents/skills/para-signups-report/.para/state.toml",
		"name = \"Weekly signups\"\ndescription = \"when signups\"\ncreated = \"2026-01-01T00:00:00Z\"\n")

	res, err := rebuild.Run(env(t, root), rebuild.Options{Scope: loc(t, "skills.signups-report")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Mirror) != 1 || res.Mirror[0].Verb != mirror.VerbCopied {
		t.Fatalf("a scoped rebuild at a skill did %v to the mirror, want one copy", res.Mirror)
	}
	got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "para-signups-report", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Weekly signups") {
		t.Errorf("the mirror still holds the old SKILL.md:\n%s", got)
	}
	// And an unscoped pass afterwards has nothing left to do.
	if after := run(t, root, rebuild.Options{}); !after.Empty() {
		t.Errorf("an unscoped rebuild after the scoped one did %v / %v / %v", after.Changed, after.Removed, after.Mirror)
	}
}

// TestClaudeSurfaceIsTheConstantCostRefresh is what a skill mutation uses: the
// eight CLAUDE.md files and nothing else, without loading a single journal.
func TestClaudeSurfaceIsTheConstantCostRefresh(t *testing.T) {
	root := plantTree(t)
	claudeOn(t, root)
	plantSkill(t, root, "signups-report", "Signups report")

	artifacts, err := env(t, root).ClaudeSurface()
	if err != nil {
		t.Fatalf("ClaudeSurface: %v", err)
	}
	var got []string
	for _, a := range artifacts {
		got = append(got, a.Path)
	}
	if !slices.Equal(got, claudeLocations) {
		t.Fatalf("ClaudeSurface = %v, want the eight locations %v", got, claudeLocations)
	}
	for _, a := range artifacts {
		if !a.Wanted {
			t.Errorf("%s is not wanted with emit.claude on", a.Path)
		}
		if !slices.Contains(a.Derived, '@') {
			t.Errorf("%s = %q, want a pointer file of @ imports", a.Path, a.Derived)
		}
	}
}
