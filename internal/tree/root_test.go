package tree_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
)

func writeTreeMarker(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".para", "tree.toml"), []byte("schema = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeStateFile(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".para", "state.toml"), []byte("name = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindDiscoversRootFromEntity(t *testing.T) {
	root := t.TempDir()
	writeTreeMarker(t, root)
	entity := filepath.Join(root, "projects", "acme")
	writeStateFile(t, entity)

	got, err := tree.Find(entity)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != root {
		t.Errorf("Find(%q) = %q, want %q", entity, got, root)
	}
}

func TestFindDiscoversRootFromContent(t *testing.T) {
	root := t.TempDir()
	writeTreeMarker(t, root)
	content := filepath.Join(root, "areas", "health", "scans")
	if err := os.MkdirAll(content, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := tree.Find(content)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != root {
		t.Errorf("Find(%q) = %q, want %q", content, got, root)
	}
}

// TestFindDiscoversRootFromInsideSkill is the §8.1 ".agents-break" case: the
// .para/ chain is not contiguous under .agents/, so a naive "walk up looking
// for any .para/" would stop at the skill's own .para/state.toml. Looking
// specifically for tree.toml sidesteps that trap.
func TestFindDiscoversRootFromInsideSkill(t *testing.T) {
	root := t.TempDir()
	writeTreeMarker(t, root)
	skill := filepath.Join(root, ".agents", "skills", "para-signups-report")
	writeStateFile(t, skill)
	nested := filepath.Join(skill, "scripts")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := tree.Find(nested)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != root {
		t.Errorf("Find(%q) = %q, want %q", nested, got, root)
	}
}

func TestFindErrorsOutsideAnyTree(t *testing.T) {
	t.Setenv("PARA_HOME", "")
	dir := t.TempDir()

	_, err := tree.Find(dir)
	if err == nil {
		t.Fatal("Find should error when no tree.toml is found up to the filesystem root")
	}
	var perr *paraerr.Error
	if !errors.As(err, &perr) || perr.Kind != paraerr.KindNotFound {
		t.Errorf("Find error = %v, want KindNotFound", err)
	}
}

func TestFindHonorsParaHomeOverride(t *testing.T) {
	root := t.TempDir()
	writeTreeMarker(t, root)
	elsewhere := t.TempDir()
	t.Setenv("PARA_HOME", root)

	got, err := tree.Find(elsewhere)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got != root {
		t.Errorf("Find honoring PARA_HOME = %q, want %q", got, root)
	}
}

func TestFindParaHomeMissingMarkerErrors(t *testing.T) {
	notATree := t.TempDir()
	t.Setenv("PARA_HOME", notATree)

	_, err := tree.Find(t.TempDir())
	if err == nil {
		t.Fatal("Find should error when PARA_HOME has no .para/tree.toml")
	}
	var perr *paraerr.Error
	if !errors.As(err, &perr) || perr.Kind != paraerr.KindNotFound {
		t.Errorf("Find error = %v, want KindNotFound", err)
	}
}
