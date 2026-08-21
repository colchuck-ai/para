package writeset_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/writeset"
)

func plant(t *testing.T, root, rel, content string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func here(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return err == nil
}

func TestRelocateCreatesDirsMovesThenPrunes(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "areas/health/training/.para/state.toml", "name = \"Training\"\n")
	plant(t, root, "areas/health/spare.txt", "mine\n")

	src := filepath.Join(root, "areas", "health", "training")
	dst := filepath.Join(root, "archive", "areas", "health", "training")
	stub := filepath.Join(root, "archive", "areas", "health")
	spare := filepath.Join(root, "areas", "health", "spare.txt")

	ops, err := writeset.Relocate(writeset.Relocation{
		Dirs:   []string{stub},
		Moves:  []writeset.Move{{From: src, To: dst}},
		Prunes: []string{spare},
	})
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}

	if here(t, src) {
		t.Error("the source survived the rename")
	}
	if !here(t, filepath.Join(dst, ".para", "state.toml")) {
		t.Error("the subtree did not arrive")
	}
	if here(t, spare) {
		t.Error("the pruned path survived")
	}

	// The op record is the evidence for what a mutation touched (§2.3), so each
	// kind is reported distinctly and a move carries both ends.
	want := []writeset.OpKind{writeset.OpMkdir, writeset.OpMove, writeset.OpPrune}
	if len(ops) != len(want) {
		t.Fatalf("ops: got %d (%v), want %d", len(ops), ops.Paths(), len(want))
	}
	for i, kind := range want {
		if ops[i].Kind != kind {
			t.Errorf("op %d: got %s, want %s", i, ops[i].Kind, kind)
		}
	}
	if ops[1].From != src {
		t.Errorf("the move op does not record where it came from: %q", ops[1].From)
	}
}

func TestRelocateRefusesAnOccupiedDestination(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "a/keep.txt", "a\n")
	plant(t, root, "b/keep.txt", "b\n")

	ops, err := writeset.Relocate(writeset.Relocation{
		Moves: []writeset.Move{{From: filepath.Join(root, "a"), To: filepath.Join(root, "b")}},
	})
	// rename(2) would silently succeed onto an empty directory and fail onto a
	// non-empty one; the caller has to have decided, so this refuses either way.
	var pe *paraerr.Error
	if !errors.As(err, &pe) || pe.Kind != paraerr.KindConflict {
		t.Fatalf("want a conflict error, got %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops: got %v, want none", ops.Paths())
	}
	if !here(t, filepath.Join(root, "a", "keep.txt")) {
		t.Error("the source was disturbed by a refused move")
	}
}

func TestRelocateSkipsWhatIsAlreadyThereOrAlreadyGone(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "archive", "areas", "health")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	ops, err := writeset.Relocate(writeset.Relocation{
		Dirs:   []string{existing},
		Prunes: []string{filepath.Join(root, "never-existed")},
	})
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	// Neither is an operation anyone performed, and §23 prints what was done.
	if len(ops) != 0 {
		t.Errorf("ops: got %v, want none", ops.Paths())
	}
}

func TestRelocateStopsAtTheFailingMoveAndReportsWhatItDid(t *testing.T) {
	root := t.TempDir()
	first := plant(t, root, "one/keep.txt", "1\n")
	plant(t, root, "two/keep.txt", "2\n")
	blocked := plant(t, root, "dest-two/keep.txt", "x\n")

	ops, err := writeset.Relocate(writeset.Relocation{
		Moves: []writeset.Move{
			{From: filepath.Join(root, "one"), To: filepath.Join(root, "dest-one")},
			{From: filepath.Join(root, "two"), To: filepath.Join(root, "dest-two")},
		},
	})
	if err == nil {
		t.Fatal("want an error on the second move")
	}
	if len(ops) != 1 {
		t.Fatalf("ops: got %v, want the first move only", ops.Paths())
	}
	// Exactly what a crash at that point would have left behind.
	if here(t, first) {
		t.Error("the first move did not happen")
	}
	if !here(t, filepath.Join(root, "two", "keep.txt")) {
		t.Error("the second source was disturbed")
	}
	if !here(t, blocked) {
		t.Error("the blocked destination was disturbed")
	}
}

func TestRelocationEmpty(t *testing.T) {
	if !(writeset.Relocation{}).Empty() {
		t.Error("a zero Relocation should be empty")
	}
	if (writeset.Relocation{Prunes: []string{"x"}}).Empty() {
		t.Error("a Relocation with a prune is not empty")
	}
}

// TestPlanRelocateReportsWhatRelocateWouldWithoutTouchingAnything is
// PlanRelocate's whole contract (para-ato): the same ops Relocate would report,
// on a tree Relocate never touches.
func TestPlanRelocateReportsWhatRelocateWouldWithoutTouchingAnything(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "areas/health/training/.para/state.toml", "name = \"Training\"\n")
	plant(t, root, "areas/health/spare.txt", "mine\n")

	src := filepath.Join(root, "areas", "health", "training")
	dst := filepath.Join(root, "archive", "areas", "health", "training")
	stub := filepath.Join(root, "archive", "areas", "health")
	spare := filepath.Join(root, "areas", "health", "spare.txt")

	r := writeset.Relocation{
		Dirs:   []string{stub},
		Moves:  []writeset.Move{{From: src, To: dst}},
		Prunes: []string{spare},
	}

	planned, err := writeset.PlanRelocate(r)
	if err != nil {
		t.Fatalf("PlanRelocate: %v", err)
	}

	if !here(t, src) {
		t.Error("PlanRelocate renamed the source")
	}
	if !here(t, spare) {
		t.Error("PlanRelocate pruned a path")
	}
	if here(t, stub) {
		t.Error("PlanRelocate created a directory")
	}

	applied, err := writeset.Relocate(r)
	if err != nil {
		t.Fatalf("Relocate: %v", err)
	}
	if got, want := planned, applied; len(got) != len(want) {
		t.Fatalf("PlanRelocate reported %d ops, want %d matching Relocate's own", len(got), len(want))
	}
	for i := range applied {
		if planned[i] != applied[i] {
			t.Errorf("op %d: PlanRelocate reported %+v, want %+v", i, planned[i], applied[i])
		}
	}
}

// TestPlanRelocateSkipsWhatIsAlreadyThereOrAlreadyGone mirrors
// TestRelocateSkipsWhatIsAlreadyThereOrAlreadyGone: a rehearsal must name no
// operation for a directory already present or a prune already absent, the
// same as a real run.
func TestPlanRelocateSkipsWhatIsAlreadyThereOrAlreadyGone(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "archive", "areas", "health")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}

	ops, err := writeset.PlanRelocate(writeset.Relocation{
		Dirs:   []string{existing},
		Prunes: []string{filepath.Join(root, "never-existed")},
	})
	if err != nil {
		t.Fatalf("PlanRelocate: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops: got %v, want none", ops.Paths())
	}
}

// TestPlanRelocateRefusesAnOccupiedDestination is
// TestRelocateRefusesAnOccupiedDestination's rehearsal twin: the refusal is a
// decision about the tree's current shape, not about writing, so a dry run
// must refuse identically.
func TestPlanRelocateRefusesAnOccupiedDestination(t *testing.T) {
	root := t.TempDir()
	plant(t, root, "a/keep.txt", "a\n")
	plant(t, root, "b/keep.txt", "b\n")

	ops, err := writeset.PlanRelocate(writeset.Relocation{
		Moves: []writeset.Move{{From: filepath.Join(root, "a"), To: filepath.Join(root, "b")}},
	})
	var pe *paraerr.Error
	if !errors.As(err, &pe) || pe.Kind != paraerr.KindConflict {
		t.Fatalf("want a conflict error, got %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("ops: got %v, want none", ops.Paths())
	}
	if !here(t, filepath.Join(root, "a", "keep.txt")) {
		t.Error("the source was disturbed by a refused move")
	}
}
