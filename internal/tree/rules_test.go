package tree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/tree"
)

// TestRules covers §5.3's ownership rule: para owns the para- prefixed rule
// files in .agents/rules/ and nothing else in that directory.
func TestRules(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agents", "rules")
	if err := os.MkdirAll(filepath.Join(dir, "para-nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{
		"para-signups-report.md",
		"para-archivist.md",
		"my-own-rule.md",      // yours: para never reads or writes it
		"para-notes.txt",      // not a rule file
		"para-nested/keep.md", // a directory, not a rule
	} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := tree.Rules(root)
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	want := []string{"para-archivist.md", "para-signups-report.md"}
	if len(got) != len(want) {
		t.Fatalf("Rules = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Rules = %v, want %v", got, want)
		}
	}
}

// TestRulesMissingDirectory covers the tree that has never had a skill: no
// .agents/rules/ at all is no rules and no error.
func TestRulesMissingDirectory(t *testing.T) {
	got, err := tree.Rules(t.TempDir())
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("Rules = %v, want none", got)
	}
}
