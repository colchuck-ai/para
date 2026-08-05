package writeset_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/writeset"
)

var pacific = time.FixedZone("PST", -8*3600)

func at(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02T15:04:05", s, pacific)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestApplyWritesTruthBeforeProjections(t *testing.T) {
	// §0.2's ordering, asserted on the record rather than on the resulting
	// bytes: journal, then state.toml, then projections, then the parent.
	root := t.TempDir()
	entity := filepath.Join(root, "areas", "health")
	parent := filepath.Join(root, "areas")

	ops, err := writeset.Apply(writeset.Mutation{
		Dir:    entity,
		Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "hello")},
		State:  []byte("name = \"Health\"\n"),
		Projections: []writeset.File{
			{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")},
			{Path: filepath.Join(entity, "ACTIVITY.md"), Bytes: []byte("activity\n")},
		},
		Parent: &writeset.Parent{
			Dir:      parent,
			Events:   []journal.Event{journal.NewChild(at(t, "2026-03-05T09:00:00"), journal.ChildOpAdded, "health", "", "", "")},
			Activity: writeset.File{Path: filepath.Join(parent, "ACTIVITY.md"), Bytes: []byte("parent activity\n")},
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{
		truth.LogsDir(entity),
		truth.StatePath(entity),
		filepath.Join(entity, "README.md"),
		filepath.Join(entity, "ACTIVITY.md"),
		truth.LogsDir(parent),
		filepath.Join(parent, "ACTIVITY.md"),
	}
	if got := ops.Paths(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ops =\n  %v\nwant\n  %v", got, want)
	}
	if ops[0].Kind != writeset.OpAppend {
		t.Errorf("first op = %v, want an append to the journal", ops[0].Kind)
	}
}

// TestApplyTouchesOnlyTheMutationsOwnFiles is §2.3's invariant, pinned: "a
// mutation touches exactly one entity's files, plus its parent's journal and
// ACTIVITY.md if and only if containment changed. Nothing walks a subtree on
// write. Nothing walks to root on write."
//
// It is asserted by snapshotting every file in the tree and diffing, rather
// than by counting the ops Apply reports — a bug that walked the tree would
// report its own extra writes honestly, but a bug that walked it *through
// another package* would not, and the filesystem is the only witness both
// share.
func TestApplyTouchesOnlyTheMutationsOwnFiles(t *testing.T) {
	root := plantTree(t)
	before := snapshot(t, root)

	entity := filepath.Join(root, "areas", "health", "training")
	ops, err := writeset.Apply(writeset.Mutation{
		Dir:    entity,
		Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "did the weekly plan")},
		Projections: []writeset.File{
			{Path: filepath.Join(entity, "ACTIVITY.md"), Bytes: []byte("# Activity\n\n## 2026-03-05\n- Note: did the weekly plan.\n")},
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	changed := diff(t, before, snapshot(t, root))
	want := []string{
		"areas/health/training/.para/logs/20260305T170000Z.jsonl",
		"areas/health/training/ACTIVITY.md",
	}
	sort.Strings(want)
	if strings.Join(changed, "\n") != strings.Join(want, "\n") {
		t.Errorf("a note on one entity changed\n  %v\nwant only\n  %v", changed, want)
	}
	if len(ops) != 2 {
		t.Errorf("ops = %v, want two", ops.Paths())
	}
}

func TestApplyWithoutAContainmentChangeLeavesTheParentAlone(t *testing.T) {
	// §3.3's boundary is exact: containment changed, or nothing is written at
	// the parent. A field change on a child is not a containment change.
	root := plantTree(t)
	before := snapshot(t, root)

	entity := filepath.Join(root, "areas", "health", "training")
	if _, err := writeset.Apply(writeset.Mutation{
		Dir:    entity,
		Events: []journal.Event{journal.NewChange(at(t, "2026-03-05T09:00:00"), "name", "Training", "Weekly training", "")},
		State:  []byte("name = \"Weekly training\"\n"),
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, path := range diff(t, before, snapshot(t, root)) {
		if !strings.HasPrefix(path, "areas/health/training/") {
			t.Errorf("a field change on training touched %s", path)
		}
	}
}

func TestApplyWritesNothingForAMutationWithNothingToWrite(t *testing.T) {
	// §15: setting a field to the value it already holds writes nothing — no
	// event, no projection rewrite. Expressed here as the degenerate mutation,
	// so the rule has a floor to stand on before Phase 8 builds `set` on it.
	root := plantTree(t)
	before := snapshot(t, root)

	ops, err := writeset.Apply(writeset.Mutation{Dir: filepath.Join(root, "areas", "health")})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops = %v, want none", ops.Paths())
	}
	if changed := diff(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("an empty mutation changed %v", changed)
	}
}

func TestApplyIsAtomicPerFile(t *testing.T) {
	// A reader sees the whole old file or the whole new one, never a prefix —
	// which means no temp file is left behind and the target is never truncated
	// in place.
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := writeset.Apply(writeset.Mutation{
		Dir:         root,
		Projections: []writeset.File{{Path: path, Bytes: []byte("new\n")}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Errorf("README.md = %q, want %q", got, "new\n")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".para-") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
}

func TestApplyCreatesTheDirectoriesItsFilesNeed(t *testing.T) {
	// `add` is one Apply, not a mkdir pass and then a write pass.
	root := t.TempDir()
	entity := filepath.Join(root, "projects", "acme")

	if _, err := writeset.Apply(writeset.Mutation{
		Dir:         entity,
		Events:      []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "first")},
		State:       []byte("name = \"Acme\"\n"),
		Projections: []writeset.File{{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")}},
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	for _, path := range []string{
		truth.StatePath(entity),
		filepath.Join(entity, "README.md"),
		filepath.Join(truth.LogsDir(entity), "20260305T170000Z.jsonl"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
}

func TestApplyRejectsAMutationWithNoDirectory(t *testing.T) {
	if _, err := writeset.Apply(writeset.Mutation{}); err == nil {
		t.Error("Apply with no directory returned no error")
	}
}

func TestApplyStopsAtTheFailingWriteAndReportsWhatItDid(t *testing.T) {
	// The ops record is what a crash-consistency harness reads (Phase 14), so a
	// partial Apply must report exactly the prefix that landed.
	root := t.TempDir()
	entity := filepath.Join(root, "areas", "health")

	// A projection whose "directory" is an existing regular file cannot be
	// written, and cannot be mkdir'd either.
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ops, err := writeset.Apply(writeset.Mutation{
		Dir:   entity,
		State: []byte("name = \"Health\"\n"),
		Projections: []writeset.File{
			{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")},
			{Path: filepath.Join(blocker, "ACTIVITY.md"), Bytes: []byte("activity\n")},
		},
	})
	if err == nil {
		t.Fatal("Apply returned no error for an unwritable projection")
	}
	want := []string{truth.StatePath(entity), filepath.Join(entity, "README.md")}
	if got := ops.Paths(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ops =\n  %v\nwant the prefix that landed\n  %v", got, want)
	}
}

// plantTree writes a small tree by hand: several entities, each with truth, a
// journal, and projections, plus content para does not own. Planting it directly
// rather than through `add` keeps this test independent of Phase 8.
func plantTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	files := map[string]string{
		".para/tree.toml":                           "schema = 1\n",
		".para/logs/20260301T080000.jsonl":          "",
		"README.md":                                 "---\nkind: \"tree\"\n---\n",
		"ACTIVITY.md":                               "# Activity\n",
		"areas/.para/state.toml":                    "name = \"Areas\"\n",
		"areas/README.md":                           "---\nkind: \"container\"\n---\n",
		"areas/ACTIVITY.md":                         "# Activity\n",
		"areas/health/.para/state.toml":             "name = \"Health\"\n",
		"areas/health/README.md":                    "---\nkind: \"area\"\n---\n",
		"areas/health/ACTIVITY.md":                  "# Activity\n",
		"areas/health/training/.para/state.toml":    "name = \"Training\"\n",
		"areas/health/training/README.md":           "---\nkind: \"area\"\n---\n",
		"areas/health/training/ACTIVITY.md":         "# Activity\n",
		"areas/health/training/notes/plan.md":       "mine, untracked\n",
		"projects/.para/state.toml":                 "name = \"Projects\"\n",
		"projects/acme/.para/state.toml":            "name = \"Acme\"\n",
		"projects/acme/ACTIVITY.md":                 "# Activity\n",
		"projects/acme/objectives/.para/state.toml": "name = \"Objectives\"\n",
		".agents/rules/para-x.md":                   "---\ngenerated_from: \"para-x\"\n---\nrule\n",
		".agents/skills/para-x/.para/state.toml":    "name = \"X\"\n",
		".agents/skills/para-x/SKILL.md":            "---\nname: \"X\"\n---\n",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// snapshot records every file in the tree by content, keyed by its
// slash-separated path relative to root.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// diff reports the paths whose content differs between two snapshots, including
// paths present in only one of them.
func diff(t *testing.T, before, after map[string]string) []string {
	t.Helper()
	var changed []string
	for path, content := range after {
		if prior, ok := before[path]; !ok || prior != content {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}
