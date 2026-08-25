package tree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
)

// TestCursorRules covers §6.2's ownership rule: para owns the para- prefixed
// `.mdc` files in .cursor/rules/ and nothing else in that directory.
func TestCursorRules(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".cursor", "rules")
	if err := os.MkdirAll(filepath.Join(dir, "para-nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"para-signups-report.mdc",
		"para-archivist.mdc",
		"my-own-rule.mdc",      // yours: para never reads or writes it
		"para-archivist.md",    // Claude's extension, not Cursor's — not this listing
		"para-notes.txt",       // not a rule file
		"para-nested/keep.mdc", // a directory, not a rule
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := tree.CursorRules(root)
	if err != nil {
		t.Fatalf("CursorRules: %v", err)
	}
	want := []string{"para-archivist.mdc", "para-signups-report.mdc"}
	if len(got) != len(want) {
		t.Fatalf("CursorRules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CursorRules = %v, want %v", got, want)
		}
	}
}

// TestCursorRulesMissingDirectory covers the tree that has never turned
// emit.cursor on: no .cursor/rules/ at all is no rules and no error.
func TestCursorRulesMissingDirectory(t *testing.T) {
	got, err := tree.CursorRules(t.TempDir())
	if err != nil {
		t.Fatalf("CursorRules: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("CursorRules = %v, want none", got)
	}
}

func TestCursorRuleSkillID(t *testing.T) {
	tests := []struct {
		basename string
		wantID   string
		wantOK   bool
	}{
		{"para-signups-report.mdc", "signups-report", true},
		{"para-archivist.mdc", "archivist", true},
		{"para-.mdc", "", false},
		{"para-archivist.md", "", false}, // Claude's suffix, not Cursor's
		{"my-own-rule.mdc", "", false},
		{"para-archivist.mdc.bak", "", false},
	}
	for _, tt := range tests {
		id, ok := tree.CursorRuleSkillID(tt.basename)
		if id != tt.wantID || ok != tt.wantOK {
			t.Errorf("CursorRuleSkillID(%q) = (%q, %v), want (%q, %v)", tt.basename, id, ok, tt.wantID, tt.wantOK)
		}
	}
}

func TestCursorRulesDir(t *testing.T) {
	if got, want := tree.CursorRulesDir(), ".cursor/rules"; got != want {
		t.Errorf("CursorRulesDir() = %q, want %q", got, want)
	}
}

// TestCursorRuleSkillIDIsTheInverseOfRenderCursorRuleFilename pins
// CursorRuleSkillID's own doc-comment claim: it is the inverse of
// render.CursorRuleFilename. Nothing enforces the two staying in sync short
// of a test that calls both, so this round-trips a handful of ids through
// the render side and back through the tree side.
func TestCursorRuleSkillIDIsTheInverseOfRenderCursorRuleFilename(t *testing.T) {
	for _, id := range []string{"a", "signups-report", "commit-style", "x-y-z"} {
		basename := render.CursorRuleFilename(id)
		got, ok := tree.CursorRuleSkillID(basename)
		if !ok || got != id {
			t.Errorf("CursorRuleSkillID(render.CursorRuleFilename(%q)) = (%q, %v), want (%q, true)", id, got, ok, id)
		}
	}
}
