package tree_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/tree"
)

// plantRelocationTree lays out the shape the relocating verbs have to reason
// about: a live area with a sub-area, an archived counterpart reached through a
// stub, and a bare directory under archive/ that leads to nothing and is
// therefore content rather than a stub (§1.6, §8.5).
func plantRelocationTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		".para/tree.toml",
		"areas/.para/state.toml",
		"areas/health/.para/state.toml",
		"areas/health/training/.para/state.toml",
		"areas/health/training/tempo/.para/state.toml",
		"areas/health/notes/scratch.md",
		"archive/.para/state.toml",
		"archive/areas/.para/state.toml",
		"archive/areas/health/nutrition/.para/state.toml",
		"archive/areas/leads-nowhere/some-file.md",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("name = \"x\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func at(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func locators(nodes []tree.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Locator.String())
	}
	return out
}

func TestSubtreeVisitsTheEntityThenEverythingBeneathIt(t *testing.T) {
	root := plantRelocationTree(t)

	nodes, err := tree.Subtree(root, at(t, "areas.health"))
	if err != nil {
		t.Fatalf("Subtree: %v", err)
	}
	want := []string{"areas.health", "areas.health.training", "areas.health.training.tempo"}
	got := locators(nodes)
	if len(got) != len(want) {
		t.Fatalf("Subtree: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Subtree[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
	// Content is never descended into (§8.5), so notes/ is not a node.
	for _, n := range got {
		if n == "areas.health.notes" {
			t.Error("Subtree descended into content")
		}
	}
}

func TestSubtreeOfALeafIsJustIt(t *testing.T) {
	root := plantRelocationTree(t)
	nodes, err := tree.Subtree(root, at(t, "areas.health.training.tempo"))
	if err != nil {
		t.Fatalf("Subtree: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("Subtree: got %v, want one node", locators(nodes))
	}
}

func TestSubtreeRefusesTheRoot(t *testing.T) {
	if _, err := tree.Subtree(plantRelocationTree(t), nil); err == nil {
		t.Error("Subtree(root) should be an error — the root is the tree, not a subtree")
	}
}

func TestChildrenIsOneGenerationOnly(t *testing.T) {
	root := plantRelocationTree(t)

	nodes, err := tree.Children(root, at(t, "areas.health"))
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	got := locators(nodes)
	if len(got) != 1 || got[0] != "areas.health.training" {
		t.Errorf("Children: got %v, want [areas.health.training]", got)
	}
}

func TestChildrenSeesAStubThatLeadsSomewhere(t *testing.T) {
	root := plantRelocationTree(t)

	nodes, err := tree.Children(root, at(t, "archive.areas"))
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	got := locators(nodes)
	// health is a stub with an entity beneath it; leads-nowhere is a bare
	// directory with nothing real under it, which §8.5 treats as content.
	if len(got) != 1 || got[0] != "archive.areas.health" {
		t.Fatalf("Children: got %v, want [archive.areas.health]", got)
	}
	if !nodes[0].Stub {
		t.Error("archive.areas.health should be reported as a stub")
	}
}

func TestIsStubAsksOnlyWhetherThereIsAnEntityBehindTheSegment(t *testing.T) {
	root := plantRelocationTree(t)

	cases := []struct {
		loc  string
		want bool
	}{
		{"archive.areas.health", true},            // a directory with no .para/
		{"archive.areas.health.nutrition", false}, // an archived entity
		{"archive.areas.leads-nowhere", true},     // bare, and the walk calls it content
		{"areas.health", false},                   // not under archive/ at all
		{"archive.areas.health.not-there", false}, // no directory
	}
	for _, tc := range cases {
		got, err := tree.IsStub(root, at(t, tc.loc))
		if err != nil {
			t.Fatalf("IsStub(%s): %v", tc.loc, err)
		}
		if got != tc.want {
			t.Errorf("IsStub(%s): got %v, want %v", tc.loc, got, tc.want)
		}
	}
}

func TestDirExistsSeesAStubThatExistsDoesNot(t *testing.T) {
	root := plantRelocationTree(t)

	dir, err := tree.DirExists(root, at(t, "archive.areas.health"))
	if err != nil {
		t.Fatal(err)
	}
	entity, err := tree.Exists(root, at(t, "archive.areas.health"))
	if err != nil {
		t.Fatal(err)
	}
	// The difference §1.6 rests on: a locator segment with no entity behind it.
	if !dir || entity {
		t.Errorf("stub: DirExists=%v Exists=%v, want true/false", dir, entity)
	}
}

func TestEntriesListsEverythingIncludingTruthAndContent(t *testing.T) {
	root := plantRelocationTree(t)

	got, err := tree.Entries(root, at(t, "areas.health"))
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	want := map[string]bool{".para": false, "training": false, "notes": false}
	for _, name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("unexpected entry %q", name)
			continue
		}
		want[name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("Entries did not report %q", name)
		}
	}
}

func TestEntriesOfAMissingDirectoryIsEmpty(t *testing.T) {
	got, err := tree.Entries(plantRelocationTree(t), at(t, "areas.nowhere"))
	if err != nil {
		t.Fatalf("Entries: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Entries: got %v, want none", got)
	}
}

func TestSkillsListsOnlyParaOwnedSkillDirectories(t *testing.T) {
	root := plantRelocationTree(t)
	for _, rel := range []string{
		".agents/skills/para-signups-report/.para/state.toml",
		".agents/skills/para-commit-style/.para/state.toml",
		".agents/skills/para-no-truth/SKILL.md",
		".agents/skills/somebody-elses/.para/state.toml",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("name = \"x\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	nodes, err := tree.Skills(root)
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	got := locators(nodes)
	want := []string{"skills.commit-style", "skills.signups-report"}
	if len(got) != len(want) {
		t.Fatalf("Skills: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Skills[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSkillsWithNoAgentsDirectoryIsEmpty(t *testing.T) {
	got, err := tree.Skills(plantRelocationTree(t))
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Skills: got %v, want none", locators(got))
	}
}
