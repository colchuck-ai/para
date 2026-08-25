package doctor_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/rebuild"
)

// TestOrphanCursorRule is TestOrphanRule's §6.2 counterpart: a derived Cursor
// rule file whose `generated_from` skill is gone.
func TestOrphanCursorRule(t *testing.T) {
	root := cleanTree(t)
	write(t, root, ".cursor/rules/para-gone.mdc",
		"---\ngenerated_from: \"para-gone\"\n---\nUse the **Gone** skill.\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindOrphanRule)
	if len(got) != 1 || !strings.Contains(got[0], "skill.gone") {
		t.Fatalf("orphan-rule findings = %v, want one naming skill.gone", got)
	}
}

// cursorTree is claudeTree's Cursor counterpart: the Cursor IDE surface
// turned on and rebuilt, so its rule file and mirror are there to be broken
// — and independent of emit.claude, which this tree leaves off.
func cursorTree(t *testing.T) string {
	t.Helper()
	root := cleanTree(t)
	write(t, root, ".para/config.toml", "emit.cursor = true\n")
	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rep := run(t, root, doctor.Options{}); !rep.Clean() {
		t.Fatalf("a tree with the Cursor surface rebuilt is not clean: %v", lines(rep))
	}
	return root
}

// TestCursorMirrorFindings is TestMirrorFindings' §6.2 counterpart: the same
// two mirror findings, and the same plain-file case a core.symlinks=false
// checkout leaves behind, reported against .cursor/skills/ instead of
// .claude/skills/.
func TestCursorMirrorFindings(t *testing.T) {
	t.Run("orphan-mirror", func(t *testing.T) {
		root := cursorTree(t)
		mkdir(t, root, ".cursor/skills/para-gone")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindOrphanMirror)
		if len(got) != 1 || !strings.Contains(got[0], "skill.gone") {
			t.Fatalf("orphan-mirror findings = %v, want one naming skill.gone", got)
		}
	})

	t.Run("a symlink that does not resolve", func(t *testing.T) {
		root := cursorTree(t)
		link := filepath.Join(root, ".cursor", "skills", "para-report")
		if err := os.Remove(link); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("../../.agents/skills/para-nowhere", link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		rep := run(t, root, doctor.Options{})

		if got := findings(rep, doctor.KindBrokenLink); len(got) != 1 {
			t.Fatalf("broken-link findings = %v, want one", got)
		}
	})

	t.Run("a link materialised as a plain file", func(t *testing.T) {
		root := cursorTree(t)
		if err := os.Remove(filepath.Join(root, ".cursor", "skills", "para-report")); err != nil {
			t.Fatal(err)
		}
		write(t, root, ".cursor/skills/para-report", "../../.agents/skills/para-report")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindBrokenLink)
		if len(got) != 1 || !strings.Contains(got[0], "core.symlinks=false") {
			t.Fatalf("broken-link findings = %v, want one naming the checkout", got)
		}
	})

	t.Run("a mirror in the wrong mode is stale", func(t *testing.T) {
		root := cursorTree(t)
		write(t, root, ".para/config.toml", "emit.cursor = true\nemit.cursor-skills = \"copy\"\n")

		rep := run(t, root, doctor.Options{})

		got := findings(rep, doctor.KindStaleProjection)
		if len(got) != 1 || !strings.Contains(got[0], "symlink where a copy belongs") {
			t.Fatalf("stale-projection findings = %v, want one naming the mode", got)
		}
	})
}

// TestCursorSurfaceResidue is TestSurfaceResidue's §6.2 counterpart: with
// `emit.cursor` off, both the mirror entry and the skill's own rule file are
// residue — one finding each, correctly naming `emit.cursor` rather than
// borrowing CLAUDE.md's "emit.claude is off" sentence.
func TestCursorSurfaceResidue(t *testing.T) {
	root := cursorTree(t)
	write(t, root, ".para/config.toml", "emit.cursor = false\n")

	rep := run(t, root, doctor.Options{})

	got := findings(rep, doctor.KindStaleProjection)
	want := []string{
		".cursor/rules/para-report.mdc: should not exist; emit.cursor is off",
		".cursor/skills/para-report: should not exist; emit.cursor is off",
	}
	if !slices.Equal(got, want) {
		t.Errorf("stale-projection findings =\n%v\nwant\n%v", got, want)
	}
	assertOnly(t, rep, doctor.KindStaleProjection)

	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if rep := run(t, root, doctor.Options{}); !rep.Clean() {
		t.Errorf("rebuild did not sweep the residue: %v", lines(rep))
	}
}

// TestScopedDoctorSkipsCursorProjections mirrors the Claude surface's own
// rule: the derived rule and the mirror sit outside any entity's subtree, so
// a scoped scan cannot reach them and must not report them (§6.2, §10).
func TestScopedDoctorSkipsCursorProjections(t *testing.T) {
	root := cursorTree(t)
	mkdir(t, root, ".cursor/skills/para-gone")
	write(t, root, ".cursor/rules/para-gone.mdc",
		"---\ngenerated_from: \"para-gone\"\n---\nUse the **Gone** skill.\n")

	rep := run(t, root, doctor.Options{Scope: loc(t, "skills.report")})

	if !rep.Clean() {
		t.Errorf("a scoped scan reported %v, want it to skip tree-wide Cursor projections", lines(rep))
	}
}
