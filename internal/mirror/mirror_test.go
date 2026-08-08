package mirror_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/render"
)

// plant writes a tree holding the named skills under .agents/skills/, each with
// a SKILL.md, a .para/state.toml that must never be mirrored (§6.1), and a
// scripts/ subdirectory carrying an executable — the shape §6.1 says an import
// cannot express and a mirror therefore has to.
func plant(t *testing.T, ids ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, id := range ids {
		dir := filepath.Join(root, ".agents", "skills", "para-"+id)
		write(t, filepath.Join(dir, "SKILL.md"), "---\nname: "+id+"\n---\n\nBody.\n", 0o644)
		write(t, filepath.Join(dir, ".para", "state.toml"), "name = \""+id+"\"\n", 0o644)
		write(t, filepath.Join(dir, "scripts", "run.sh"), "#!/bin/sh\necho "+id+"\n", 0o755)
	}
	return root
}

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func cfg(enabled bool, mode string) render.Config {
	c := render.DefaultConfig()
	c.EmitClaude = enabled
	c.EmitClaudeSkills = mode
	return c
}

// sync inspects and repairs in one step, which is what every caller does, and
// returns what the repair reported.
func sync(t *testing.T, root string, c render.Config, skills ...string) []mirror.Change {
	t.Helper()
	issues, err := mirror.Inspect(root, c, skills, nil)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	changes, err := mirror.Repair(root, c, issues)
	if err != nil {
		t.Fatalf("Repair: %v", err)
	}
	return changes
}

func states(issues []mirror.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Path+" "+string(i.State))
	}
	return out
}

func paths(changes []mirror.Change) []string {
	out := make([]string, 0, len(changes))
	for _, c := range changes {
		out = append(out, string(c.Verb)+" "+c.Path)
	}
	return out
}

func TestInspectReportsNothingWhenTheSurfaceIsOffAndAbsent(t *testing.T) {
	root := plant(t, "signups-report")

	issues, err := mirror.Inspect(root, cfg(false, render.MirrorSymlink), []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("Inspect on a tree that never turned the surface on = %v, want none", states(issues))
	}
}

func TestSymlinkModeLinksEverySkill(t *testing.T) {
	root := plant(t, "commit-style", "signups-report")
	c := cfg(true, render.MirrorSymlink)

	changes := sync(t, root, c, "commit-style", "signups-report")
	want := []string{
		"linked .claude/skills/para-commit-style",
		"linked .claude/skills/para-signups-report",
	}
	if !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}

	target, err := os.Readlink(filepath.Join(root, ".claude", "skills", "para-signups-report"))
	if err != nil {
		t.Fatalf("the mirror is not a symlink: %v", err)
	}
	// Relative, so the link survives the tree being cloned or moved. Compared
	// through ToSlash because the link is written through FromSlash: on Windows
	// os.Readlink hands back `..\..\.agents\skills\para-signups-report`, which
	// is the same link spelled in the separator that platform uses.
	if want := "../../.agents/skills/para-signups-report"; filepath.ToSlash(target) != want {
		t.Errorf("link target = %q, want %q", target, want)
	}

	// Idempotent: a second pass finds nothing to do, which is what makes
	// `rebuild; rebuild` a no-op (§21.1).
	//
	// This is the assertion that catches a write and a read disagreeing about
	// separators — Inspect saw `..\..\…` where Target says `../../…`, called a
	// link para had just created stale, and left doctor red on every Windows
	// tree forever. It cannot fail on Unix, where FromSlash and ToSlash are both
	// the identity, so the Windows CI job is what actually exercises it. That is
	// the job's whole reason for existing (§6.1).
	if changes := sync(t, root, c, "commit-style", "signups-report"); len(changes) != 0 {
		t.Errorf("second sync = %v, want nothing", paths(changes))
	}
}

func TestCopyModeCopiesTheSkillWithoutItsPara(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorCopy)

	changes := sync(t, root, c, "signups-report")
	if want := []string{"copied .claude/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}

	dir := filepath.Join(root, ".claude", "skills", "para-signups-report")
	if _, err := os.ReadFile(filepath.Join(dir, "SKILL.md")); err != nil {
		t.Errorf("SKILL.md was not copied: %v", err)
	}
	// §6.1: a reachable .claude/skills/para-X/.para/state.toml would satisfy
	// §1.2's entity test, and doctor's deep scan would report a phantom entity.
	if _, err := os.Stat(filepath.Join(dir, ".para")); !os.IsNotExist(err) {
		t.Error(".para/ was mirrored; §6.1 says it never is")
	}
	info, err := os.Stat(filepath.Join(dir, "scripts", "run.sh"))
	if err != nil {
		t.Fatalf("scripts/run.sh was not copied: %v", err)
	}
	// Windows has no execute bit — os.Stat reports 0666 or 0444 for every
	// regular file, and contents() derives Exec the same way, so the copy is
	// written 0644 and this can never hold there. Skipping the assertion rather
	// than the test keeps the rest of copy mode — which is the §6.1 mode
	// Windows is in CI to exercise — covered on the platform that needs it.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Error("the copied script is not executable; a skill that ships scripts ships them runnable")
	}

	if changes := sync(t, root, c, "signups-report"); len(changes) != 0 {
		t.Errorf("second sync = %v, want nothing", paths(changes))
	}
}

// Everything inside a skill's directory is the author's (§5.1), including a
// symlink they put there. A copy carries it as a link, so it keeps meaning what
// they wrote — and so the comparison converges instead of calling the mirror
// stale forever.
func TestCopyModeCarriesASymlinkInsideTheSkill(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorCopy)
	inside := filepath.Join(root, ".agents", "skills", "para-signups-report")
	if err := os.Symlink("run.sh", filepath.Join(inside, "scripts", "latest.sh")); err != nil {
		t.Skipf("this filesystem does not support symlinks: %v", err)
	}

	sync(t, root, c, "signups-report")
	target, err := os.Readlink(filepath.Join(root, ".claude", "skills", "para-signups-report", "scripts", "latest.sh"))
	if err != nil {
		t.Fatalf("the copy dropped the skill's own symlink: %v", err)
	}
	if target != "run.sh" {
		t.Errorf("copied link target = %q, want %q", target, "run.sh")
	}
	if changes := sync(t, root, c, "signups-report"); len(changes) != 0 {
		t.Errorf("second sync = %v, want nothing", paths(changes))
	}
}

func TestCopyModeReportsAnEditedCopyAsStale(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorCopy)
	sync(t, root, c, "signups-report")

	edited := filepath.Join(root, ".claude", "skills", "para-signups-report", "SKILL.md")
	write(t, edited, "hand-edited\n", 0o644)

	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".claude/skills/para-signups-report " + string(mirror.StateStale)}
	if !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect = %v, want %v", states(issues), want)
	}

	sync(t, root, c, "signups-report")
	got, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == "hand-edited\n" {
		t.Error("the repair did not overwrite the edited copy")
	}
}

func TestCopyModeReportsAnExtraFileAsStale(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorCopy)
	sync(t, root, c, "signups-report")

	extra := filepath.Join(root, ".claude", "skills", "para-signups-report", "notes.md")
	write(t, extra, "mine\n", 0o644)

	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/skills/para-signups-report " + string(mirror.StateStale)}; !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect = %v, want %v", states(issues), want)
	}

	sync(t, root, c, "signups-report")
	if _, err := os.Stat(extra); !os.IsNotExist(err) {
		t.Error("the repair left a file the skill does not have")
	}
}

func TestSwitchingModesLeavesNoResidue(t *testing.T) {
	root := plant(t, "signups-report")
	sync(t, root, cfg(true, render.MirrorSymlink), "signups-report")

	changes := sync(t, root, cfg(true, render.MirrorCopy), "signups-report")
	if want := []string{"copied .claude/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("switching to copy = %v, want %v", paths(changes), want)
	}
	dir := filepath.Join(root, ".claude", "skills", "para-signups-report")
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("the copy did not replace the symlink")
	}

	changes = sync(t, root, cfg(true, render.MirrorSymlink), "signups-report")
	if want := []string{"linked .claude/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("switching back to symlink = %v, want %v", paths(changes), want)
	}
	if _, err := os.Readlink(dir); err != nil {
		t.Errorf("the symlink did not replace the copy: %v", err)
	}
}

func TestTurningTheSurfaceOffSweepsTheMirror(t *testing.T) {
	root := plant(t, "signups-report")
	sync(t, root, cfg(true, render.MirrorSymlink), "signups-report")

	issues, err := mirror.Inspect(root, cfg(false, render.MirrorSymlink), []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{".claude/skills/para-signups-report " + string(mirror.StateResidue)}
	if !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect with emit.claude off = %v, want %v", states(issues), want)
	}

	changes := sync(t, root, cfg(false, render.MirrorSymlink), "signups-report")
	if want := []string{"removed .claude/skills/para-signups-report"}; !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}
	// The directories go too: §26's `init` promises "no CLAUDE.md, no .claude/",
	// and a tree that turned the surface off should be back to that shape.
	if _, err := os.Stat(filepath.Join(root, ".claude")); !os.IsNotExist(err) {
		t.Error(".claude/ survived the sweep")
	}
}

func TestASkillThatIsGoneLeavesAnOrphanMirror(t *testing.T) {
	root := plant(t, "signups-report", "commit-style")
	c := cfg(true, render.MirrorSymlink)
	sync(t, root, c, "signups-report", "commit-style")

	if err := os.RemoveAll(filepath.Join(root, ".agents", "skills", "para-commit-style")); err != nil {
		t.Fatal(err)
	}
	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/skills/para-commit-style " + string(mirror.StateOrphan)}; !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect = %v, want %v", states(issues), want)
	}

	changes := sync(t, root, c, "signups-report")
	if want := []string{"removed .claude/skills/para-commit-style"}; !slices.Equal(paths(changes), want) {
		t.Errorf("Repair = %v, want %v", paths(changes), want)
	}
	// The surviving skill's link is untouched, so pruning one mirror is not a
	// rewrite of the rest.
	if _, err := os.Readlink(filepath.Join(root, ".claude", "skills", "para-signups-report")); err != nil {
		t.Errorf("the surviving mirror was disturbed: %v", err)
	}
}

// A core.symlinks=false checkout — the Windows default — materialises each link
// as a plain file holding its target path (§6.1). It is the one mirror failure
// nobody caused, so it gets its own state and its own sentence.
func TestAPlainFileWhereALinkBelongsIsBroken(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorSymlink)
	sync(t, root, c, "signups-report")

	path := filepath.Join(root, ".claude", "skills", "para-signups-report")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	write(t, path, "../../.agents/skills/para-signups-report", 0o644)

	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/skills/para-signups-report " + string(mirror.StateBroken)}; !slices.Equal(states(issues), want) {
		t.Fatalf("Inspect = %v, want %v", states(issues), want)
	}

	sync(t, root, c, "signups-report")
	if _, err := os.Readlink(path); err != nil {
		t.Errorf("rebuild did not repair the hostile checkout: %v", err)
	}
}

func TestALinkThatDoesNotResolveIsBroken(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorSymlink)
	sync(t, root, c, "signups-report")

	path := filepath.Join(root, ".claude", "skills", "para-signups-report")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../.agents/skills/para-nowhere", path); err != nil {
		t.Fatal(err)
	}

	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/skills/para-signups-report " + string(mirror.StateBroken)}; !slices.Equal(states(issues), want) {
		t.Errorf("Inspect = %v, want %v", states(issues), want)
	}
}

func TestSomethingWithoutTheParaPrefixIsNeverTouched(t *testing.T) {
	root := plant(t, "signups-report")
	c := cfg(true, render.MirrorSymlink)
	sync(t, root, c, "signups-report")

	mine := filepath.Join(root, ".claude", "skills", "my-own-skill", "SKILL.md")
	write(t, mine, "mine\n", 0o644)

	// Not reported when the surface is on...
	issues, err := mirror.Inspect(root, c, []string{"signups-report"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 0 {
		t.Errorf("Inspect = %v, want none: a name without the para- prefix is not para's (§6.1)", states(issues))
	}

	// ...nor swept when it is turned off, and its presence is what keeps
	// .claude/skills/ from being pruned.
	sync(t, root, cfg(false, render.MirrorSymlink), "signups-report")
	if _, err := os.Stat(mine); err != nil {
		t.Errorf("para removed something that was not its own: %v", err)
	}
}

func TestAMissingMirrorIsReported(t *testing.T) {
	root := plant(t, "signups-report", "commit-style")
	c := cfg(true, render.MirrorSymlink)
	sync(t, root, c, "signups-report")

	issues, err := mirror.Inspect(root, c, []string{"signups-report", "commit-style"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{".claude/skills/para-commit-style " + string(mirror.StateMissing)}; !slices.Equal(states(issues), want) {
		t.Errorf("Inspect = %v, want %v", states(issues), want)
	}
}
