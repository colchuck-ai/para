package mutate_test

import (
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

// TestActivityWrittenIncrementallyMatchesAFullRederivation is this package's
// central property, and the one §10's dated drift report depends on: the cheap
// write and the full re-derivation must produce identical bytes for the same
// history.
//
// It is asserted across days — several mutations on several different dates, so
// the incremental path really does splice today's section onto prior ones it
// never recomputes — and across the mutation kinds that write the file: a
// creation, a note, a field change, and a change to `created` itself, which
// moves the "Created." line to a different day and is the case incremental mode
// cannot handle.
func TestActivityWrittenIncrementallyMatchesAFullRederivation(t *testing.T) {
	root := plantTree(t)

	// Each step runs with its own clock, so "today" really moves.
	steps := []struct {
		day string
		do  func(*mutate.Env) error
	}{
		{"2026-03-01", func(e *mutate.Env) error {
			_, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying in one piece."))
			return err
		}},
		{"2026-03-02", func(e *mutate.Env) error {
			_, err := e.Note(loc(t, "areas.health"), "swapped the tempo block for intervals", "")
			return err
		}},
		{"2026-03-04", func(e *mutate.Env) error {
			_, err := e.Set(loc(t, "areas.health"), fields("priority", "high"), "")
			return err
		}},
		{"2026-03-05", func(e *mutate.Env) error {
			_, err := e.Note(loc(t, "areas.health"), "second note the same week", "")
			return err
		}},
		{"2026-03-06", func(e *mutate.Env) error {
			// Moving `created` moves a line between two day sections, one of
			// which incremental mode would never revisit.
			_, err := e.Set(loc(t, "areas.health"), fields("created", "2026-02-20"), "")
			return err
		}},
		{"2026-03-07", func(e *mutate.Env) error {
			_, err := e.Note(loc(t, "areas.health"), "after the backdate", "")
			return err
		}},
	}

	for _, step := range steps {
		day, err := time.ParseInLocation("2006-01-02T15:04:05", step.day+"T09:00:00", pacific)
		if err != nil {
			t.Fatal(err)
		}
		e := mutate.NewEnv(root, clock.Fixed{At: day})
		if err := step.do(e); err != nil {
			t.Fatalf("%s: %v", step.day, err)
		}

		for _, target := range []string{"areas.health", "areas"} {
			assertActivityMatchesFull(t, root, target, step.day)
		}
	}
}

// assertActivityMatchesFull re-derives ACTIVITY.md the way rebuild and doctor
// will — full mode, from every journal file backing it — and compares byte for
// byte against what the write path left on disk (§10, §21.1).
func assertActivityMatchesFull(t *testing.T, root, target, when string) {
	t.Helper()
	l, err := locator.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := l.Path()
	if err != nil {
		t.Fatal(err)
	}
	dir := joinRoot(root, rel)

	state, err := truth.ReadState(dir)
	if err != nil {
		t.Fatalf("ReadState(%s): %v", target, err)
	}
	events, err := journal.ReadAll(truth.LogsDir(dir))
	if err != nil {
		t.Fatalf("ReadAll(%s): %v", target, err)
	}
	kind := kindmeta.KindArea
	if target == "areas" {
		kind = kindmeta.KindContainer
	}

	full, err := render.Activity.Render(render.In{
		Locator: l,
		Kind:    kind,
		State:   state,
		Events:  events,
		Config:  render.DefaultConfig(),
	})
	if err != nil {
		t.Fatalf("full render(%s): %v", target, err)
	}
	if got := read(t, root, rel+"/ACTIVITY.md"); got != string(full) {
		t.Errorf("after %s, %s/ACTIVITY.md written incrementally =\n%q\nbut a full re-derivation gives\n%q", when, target, got, full)
	}
}
