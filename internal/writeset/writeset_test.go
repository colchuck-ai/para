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
		Subjects: []writeset.Subject{{
			Dir:    entity,
			Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "hello")},
			State:  []byte("name = \"Health\"\n"),
			Config: []byte{},
			Projections: []writeset.File{
				{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")},
				{Path: filepath.Join(entity, "ACTIVITY.md"), Bytes: []byte("activity\n")},
			},
		}},
		Parents: []writeset.Subject{{
			Dir:         parent,
			Events:      []journal.Event{journal.NewChild(at(t, "2026-03-05T09:00:00"), journal.ChildOpAdded, "health", "", "", "")},
			Projections: []writeset.File{{Path: filepath.Join(parent, "ACTIVITY.md"), Bytes: []byte("parent activity\n")}},
		}},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := []string{
		filepath.Join(truth.LogsDir(entity), "20260305T170000Z.jsonl"),
		truth.StatePath(entity),
		truth.ConfigPath(entity),
		filepath.Join(entity, "README.md"),
		filepath.Join(entity, "ACTIVITY.md"),
		filepath.Join(truth.LogsDir(parent), "20260305T170000Z.jsonl"),
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
		Subjects: []writeset.Subject{{
			Dir:    entity,
			Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "did the weekly plan")},
			Projections: []writeset.File{
				{Path: filepath.Join(entity, "ACTIVITY.md"), Bytes: []byte("# Activity\n\n## 2026-03-05\n- Note: did the weekly plan.\n")},
			},
		}},
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
		Subjects: []writeset.Subject{{
			Dir:    entity,
			Events: []journal.Event{journal.NewChange(at(t, "2026-03-05T09:00:00"), "name", "Training", "Weekly training", "")},
			State:  []byte("name = \"Weekly training\"\n"),
		}},
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

	ops, err := writeset.Apply(writeset.Mutation{
		Subjects: []writeset.Subject{{Dir: filepath.Join(root, "areas", "health")}},
	})
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
		Subjects: []writeset.Subject{{
			Dir:         root,
			Projections: []writeset.File{{Path: path, Bytes: []byte("new\n")}},
		}},
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
		Subjects: []writeset.Subject{{
			Dir:         entity,
			Dirs:        []string{truth.LogsDir(entity)},
			Events:      []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "first")},
			State:       []byte("name = \"Acme\"\n"),
			Projections: []writeset.File{{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")}},
		}},
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
		t.Error("Apply with no subject returned no error")
	}
	if _, err := writeset.Apply(writeset.Mutation{Subjects: []writeset.Subject{{}}}); err == nil {
		t.Error("Apply with a subject that has no directory returned no error")
	}
}

// TestApplyWritesEveryTruthFileBeforeAnyProjection is the ordering rule for a
// mutation with more than one subject — `add` on a project, which creates the
// project and its objectives/ container in one operation (§18.1).
//
// Interleaving per subject (A's truth, A's projections, B's truth, …) would let
// a crash leave B missing altogether, and a missing container is not the one
// degraded state the design defines a repair for: `rebuild` regenerates
// projections from truth, so it cannot invent truth that was never written.
func TestApplyWritesEveryTruthFileBeforeAnyProjection(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "projects", "acme")
	container := filepath.Join(project, "objectives")

	ops, err := writeset.Apply(writeset.Mutation{
		Subjects: []writeset.Subject{
			{
				Dir:         project,
				Dirs:        []string{truth.LogsDir(project)},
				State:       []byte("name = \"Acme\"\n"),
				Config:      []byte{},
				Projections: []writeset.File{{Path: filepath.Join(project, "README.md"), Bytes: []byte("readme\n")}},
			},
			{
				Dir:         container,
				Dirs:        []string{truth.LogsDir(container)},
				State:       []byte("name = \"Objectives\"\n"),
				Config:      []byte{},
				Projections: []writeset.File{{Path: filepath.Join(container, "README.md"), Bytes: []byte("readme\n")}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Truth first, then the empty scaffolding directory, then the projections.
	// logs/ comes after state.toml rather than before it because a directory is
	// not truth: created first, it would be the one thing a crash at the very
	// first write could leave behind, and an empty directory in a bucket is
	// `untracked` — a finding `rebuild` cannot clear (§0.2, §21.2).
	want := []string{
		truth.StatePath(project),
		truth.ConfigPath(project),
		truth.LogsDir(project),
		truth.StatePath(container),
		truth.ConfigPath(container),
		truth.LogsDir(container),
		filepath.Join(project, "README.md"),
		filepath.Join(container, "README.md"),
	}
	if got := ops.Paths(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("ops =\n  %v\nwant\n  %v", got, want)
	}

	// The empty logs/ directory `add` promises is really there, even though no
	// event has been written into it (§18.1: the journal starts empty).
	for _, dir := range []string{truth.LogsDir(project), truth.LogsDir(container)} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("expected %s to be an empty directory: %v", dir, err)
		}
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
		Subjects: []writeset.Subject{{
			Dir:   entity,
			State: []byte("name = \"Health\"\n"),
			Projections: []writeset.File{
				{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")},
				{Path: filepath.Join(blocker, "ACTIVITY.md"), Bytes: []byte("activity\n")},
			},
		}},
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

// TestPlanReportsWhatApplyWouldWithoutWriting is Plan's central claim: for the
// same Mutation, it reports the identical ops Apply would, and it writes
// nothing at all — not even a directory.
func TestPlanReportsWhatApplyWouldWithoutWriting(t *testing.T) {
	root := t.TempDir()
	entity := filepath.Join(root, "areas", "health")
	parent := filepath.Join(root, "areas")

	before := snapshot(t, root)

	m := writeset.Mutation{
		Subjects: []writeset.Subject{{
			Dir:    entity,
			Dirs:   []string{truth.LogsDir(entity)},
			Events: []journal.Event{journal.NewNote(at(t, "2026-03-05T09:00:00"), "hello")},
			State:  []byte("name = \"Health\"\n"),
			Config: []byte{},
			Projections: []writeset.File{
				{Path: filepath.Join(entity, "README.md"), Bytes: []byte("readme\n")},
				{Path: filepath.Join(entity, "ACTIVITY.md"), Bytes: []byte("activity\n")},
			},
		}},
		Parents: []writeset.Subject{{
			Dir:         parent,
			Events:      []journal.Event{journal.NewChild(at(t, "2026-03-05T09:00:00"), journal.ChildOpAdded, "health", "", "", "")},
			Projections: []writeset.File{{Path: filepath.Join(parent, "ACTIVITY.md"), Bytes: []byte("parent activity\n")}},
		}},
	}

	planned, err := writeset.Plan(m)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}

	if changed := diff(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("Plan wrote %v, want nothing", changed)
	}
	if _, err := os.Stat(truth.LogsDir(entity)); !os.IsNotExist(err) {
		t.Errorf("Plan created %s", truth.LogsDir(entity))
	}

	applied, err := writeset.Apply(m)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Plan has nothing to say about OpMkdir (see planTruth's doc comment: a
	// directory creation is not a file `wrote` ever prints), so the comparison
	// is over the ops that actually land in a printed file list — exactly
	// mutate.wrote's own filter.
	if got, want := writeOps(planned), writeOps(applied); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Plan's write/append ops =\n  %v\nwant Apply's own\n  %v", got, want)
	}
}

// writeOps is a mutation's ops narrowed to the ones a printed file list would
// show — the same filter mutate.wrote applies to Apply's and Plan's output
// alike, reimplemented here rather than imported so this package's tests do
// not reach into mutate.
func writeOps(ops writeset.Ops) []string {
	var out []string
	for _, op := range ops {
		if op.Kind != writeset.OpAppend && op.Kind != writeset.OpWrite {
			continue
		}
		out = append(out, op.Kind.String()+" "+op.Path)
	}
	return out
}

// TestPlanAgreesWithApplyAcrossAMultiEventRotation is TestPlanReportsWhat...
// stress-tested at the Mutation level rather than journal.PlanAppend's own
// unit tests: several events landing in one subject's journal within a single
// planned mutation, at a threshold that forces every one of them into its own
// file, must name the same files a real Apply of an identical mutation would.
func TestPlanAgreesWithApplyAcrossAMultiEventRotation(t *testing.T) {
	root := t.TempDir()
	entity := filepath.Join(root, "projects", "acme")
	base := at(t, "2026-03-05T09:00:00")

	newMutation := func() writeset.Mutation {
		return writeset.Mutation{
			Subjects: []writeset.Subject{{
				Dir: entity,
				Events: []journal.Event{
					journal.NewChange(base, "status", "todo", "in-progress", ""),
					journal.NewChange(base.Add(time.Minute), "priority", "p2", "p1", ""),
					journal.NewChange(base.Add(2*time.Minute), "due", "", "2026-04-01", ""),
				},
				RotateBytes: 1, // every event past the first opens a new file
			}},
		}
	}

	planned, err := writeset.Plan(newMutation())
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(planned) != 3 {
		t.Fatalf("Plan produced %d ops, want 3 (one per event)", len(planned))
	}

	applied, err := writeset.Apply(newMutation())
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got, want := planned.Paths(), applied.Paths(); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Plan.Paths() =\n  %v\nwant Apply's own\n  %v", got, want)
	}
	// Confirms the rotation genuinely happened three ways, not that both
	// sides made the same (possibly wrong) simplifying assumption.
	seen := map[string]bool{}
	for _, p := range applied.Paths() {
		seen[p] = true
	}
	if len(seen) != 3 {
		t.Errorf("Apply landed the three events in %d distinct files, want 3", len(seen))
	}
}

func TestPlanWithNothingToWriteReturnsNoOps(t *testing.T) {
	root := plantTree(t)
	before := snapshot(t, root)

	planned, err := writeset.Plan(writeset.Mutation{
		Subjects: []writeset.Subject{{Dir: filepath.Join(root, "areas", "health")}},
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(planned) != 0 {
		t.Errorf("Plan = %v, want none", planned.Paths())
	}
	if changed := diff(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("Plan of an empty mutation changed %v", changed)
	}
}

func TestPlanRejectsAMutationWithNoDirectory(t *testing.T) {
	if _, err := writeset.Plan(writeset.Mutation{}); err == nil {
		t.Error("Plan with no subject returned no error")
	}
	if _, err := writeset.Plan(writeset.Mutation{Subjects: []writeset.Subject{{}}}); err == nil {
		t.Error("Plan with a subject that has no directory returned no error")
	}
}

// TestApplyRotatesEachSubjectOnItsOwnThreshold pins log.rotate-bytes as a
// per-subject value.
//
// The key is chain-resolved like every other (§3.4, §7), so a project that sets
// its own must not decide when its parent bucket's journal rotates — and a
// single threshold shared across a mutation would make it do exactly that,
// silently, because the parent's own config.toml would never be consulted.
func TestApplyRotatesEachSubjectOnItsOwnThreshold(t *testing.T) {
	root := t.TempDir()
	eager := filepath.Join(root, "projects", "acme")
	patient := filepath.Join(root, "projects")

	first := at(t, "2026-03-05T09:00:00")
	second := at(t, "2026-03-05T10:00:00")
	for _, when := range []time.Time{first, second} {
		if _, err := writeset.Apply(writeset.Mutation{
			Subjects: []writeset.Subject{{
				Dir:         eager,
				Events:      []journal.Event{journal.NewNote(when, "eager")},
				RotateBytes: 1, // every event past the first opens a new file
			}},
			Parents: []writeset.Subject{{
				Dir:    patient,
				Events: []journal.Event{journal.NewChild(when, journal.ChildOpAdded, "acme", "", "", "")},
				// No threshold: journal.DefaultRotateBytes, 4 MiB.
			}},
		}); err != nil {
			t.Fatalf("Apply: %v", err)
		}
	}

	eagerFiles := journalFiles(t, truth.LogsDir(eager))
	if len(eagerFiles) != 2 {
		t.Errorf("the subject's journal has %v, want two files at a 1-byte threshold", eagerFiles)
	}
	patientFiles := journalFiles(t, truth.LogsDir(patient))
	if len(patientFiles) != 1 {
		t.Errorf("the parent's journal has %v, want one file at the default threshold", patientFiles)
	}
}

func journalFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}
