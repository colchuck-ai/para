package mirror_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/render"
)

// cursorCfg is cfg's Cursor-target twin (§6.2).
func cursorCfg(enabled bool, mode string) render.Config {
	c := render.DefaultConfig()
	c.EmitCursor = enabled
	c.EmitCursorSkills = mode
	return c
}

func TestCursorSymlinkModeLinksEverySkill(t *testing.T) {
	root := plant(t, "commit-style", "signups-report")
	c := cursorCfg(true, render.MirrorSymlink)

	changes := sync(t, root, mirror.Cursor, c, "commit-style", "signups-report")
	want := []string{
		"linked .cursor/skills/para-commit-style",
		"linked .cursor/skills/para-signups-report",
	}
	if !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}

	target, err := os.Readlink(filepath.Join(root, ".cursor", "skills", "para-signups-report"))
	if err != nil {
		t.Fatalf("the mirror is not a symlink: %v", err)
	}
	if want := "../../.agents/skills/para-signups-report"; filepath.ToSlash(target) != want {
		t.Errorf("link target = %q, want %q", target, want)
	}

	if changes := sync(t, root, mirror.Cursor, c, "commit-style", "signups-report"); len(changes) != 0 {
		t.Errorf("second sync = %v, want nothing", paths(changes))
	}
}

func TestCursorCopyModeCopiesTheSkillWithoutItsPara(t *testing.T) {
	root := plant(t, "signups-report")
	c := cursorCfg(true, render.MirrorCopy)

	changes := sync(t, root, mirror.Cursor, c, "signups-report")
	if want := []string{"copied .cursor/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}

	dir := filepath.Join(root, ".cursor", "skills", "para-signups-report")
	if _, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md was not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".para")); !os.IsNotExist(err) {
		t.Error(".para/ was mirrored; §6.2 says it never is")
	}

	if changes := sync(t, root, mirror.Cursor, c, "signups-report"); len(changes) != 0 {
		t.Errorf("second sync = %v, want nothing", paths(changes))
	}
}

func TestCursorTurningTheSurfaceOffSweepsTheMirror(t *testing.T) {
	root := plant(t, "signups-report")
	sync(t, root, mirror.Cursor, cursorCfg(true, render.MirrorSymlink), "signups-report")

	issues, err := mirror.Inspect(root, mirror.Cursor, cursorCfg(false, render.MirrorSymlink), []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".cursor/skills/para-signups-report " + string(mirror.StateResidue)}
	if !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect with emit.cursor off = %v, want %v", states(issues), want)
	}
	if issues[0].Detail != "should not exist; emit.cursor is off" {
		t.Errorf("Detail = %q, want the emit.cursor sentence", issues[0].Detail)
	}

	changes := sync(t, root, mirror.Cursor, cursorCfg(false, render.MirrorSymlink), "signups-report")
	if want := []string{"removed .cursor/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor")); !os.IsNotExist(err) {
		t.Error(".cursor/ survived the sweep")
	}
}

func TestCursorASkillThatIsGoneLeavesAnOrphanMirror(t *testing.T) {
	root := plant(t, "signups-report", "commit-style")
	c := cursorCfg(true, render.MirrorSymlink)
	sync(t, root, mirror.Cursor, c, "signups-report", "commit-style")

	if err := os.RemoveAll(filepath.Join(root, ".agents", "skills", "para-commit-style")); err != nil {
		t.Fatal(err)
	}
	issues, err := mirror.Inspect(root, mirror.Cursor, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".cursor/skills/para-commit-style " + string(mirror.StateOrphan)}; !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect = %v, want %v", states(issues), want)
	}
}

func TestCursorALinkThatDoesNotResolveIsBroken(t *testing.T) {
	root := plant(t, "signups-report")
	c := cursorCfg(true, render.MirrorSymlink)
	sync(t, root, mirror.Cursor, c, "signups-report")

	path := filepath.Join(root, ".cursor", "skills", "para-signups-report")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.agents/skills/para-nowhere", path); err != nil {
		t.Fatal(err)
	}

	issues, err := mirror.Inspect(root, mirror.Cursor, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".cursor/skills/para-signups-report " + string(mirror.StateBroken)}; !slices.Equal(states(issues), want) {
		t.Errorf("Inspect = %v, want %v", states(issues), want)
	}
}

// TestClaudeAndCursorMirrorsAreIndependent covers §6.2's "independent of
// emit.claude — both may be on at once, and neither implies the other":
// turning one target on must not write, read, or report on the other's
// directory.
func TestClaudeAndCursorMirrorsAreIndependent(t *testing.T) {
	root := plant(t, "signups-report")

	claudeChanges := sync(t, root, mirror.Claude, cfg(true, render.MirrorSymlink), "signups-report")
	if want := []string{"linked .claude/skills/para-signups-report"}; !slices.Equal(paths(claudeChanges), want) {
		t.Fatalf("Claude sync = %v, want %v", paths(claudeChanges), want)
	}
	if _, err := os.Stat(filepath.Join(root, ".cursor")); !os.IsNotExist(err) {
		t.Error("enabling the Claude surface alone created .cursor/")
	}

	cursorIssues, err := mirror.Inspect(root, mirror.Cursor, cursorCfg(false, render.MirrorSymlink), []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cursorIssues) != 0 {
		t.Errorf("Cursor Inspect with the Claude surface on and its own flag off = %v, want none", states(cursorIssues))
	}

	cursorChanges := sync(t, root, mirror.Cursor, cursorCfg(true, render.MirrorSymlink), "signups-report")
	if want := []string{"linked .cursor/skills/para-signups-report"}; !slices.Equal(paths(cursorChanges), want) {
		t.Fatalf("Cursor sync = %v, want %v", paths(cursorChanges), want)
	}

	claudeIssues, err := mirror.Inspect(root, mirror.Claude, cfg(true, render.MirrorSymlink), []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(claudeIssues) != 0 {
		t.Errorf("Claude Inspect after turning Cursor on too = %v, want none", states(claudeIssues))
	}
}
