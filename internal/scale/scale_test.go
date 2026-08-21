package scale_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/scale"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/view"
)

// This file is the plan's Phase 14 task 5: a scale check on a generated tree of
// a few thousand entities, asserting that write cost is constant in the size of
// the tree, that read paths do not open journals they do not need, and timing
// `doctor` and `list`.
//
// Two of the three are properties rather than measurements, and are asserted as
// such. The timings are printed rather than asserted: a threshold that passes on
// a developer's machine and fails on a loaded CI runner is a flaky test wearing
// a performance test's clothes, and the number that matters — whether cost grows
// with the tree — is exactly the thing the two properties pin down.

func fixedNow(t *testing.T) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, "2026-03-05T17:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// plantAndRebuild plants a tree of the given shape and renders it, returning the
// root and how long the rebuild took.
func plantAndRebuild(t *testing.T, s scale.Shape) (string, time.Duration) {
	t.Helper()
	root := t.TempDir()
	if err := scale.Plant(root, s); err != nil {
		t.Fatalf("planting: %v", err)
	}
	start := time.Now()
	res, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	elapsed := time.Since(start)
	if len(res.Changed) == 0 {
		t.Fatal("rebuild wrote nothing over a freshly planted tree")
	}
	return root, elapsed
}

// small is the control: the same *shape* as the large tree in every respect but
// size, so the only variable between the two runs is how many entities there
// are. Matching the journal seeding matters — a project with no journal file
// opens one named for now, and a project with one appends to it, which is a
// different path for a reason that has nothing to do with scale.
func small() scale.Shape {
	s := scale.Default()
	s.Projects = 4
	s.Areas = 1
	s.SubAreasPerArea = 1
	s.Resources = 1
	s.Skills = 1
	s.ArchivedPerBucket = 1
	s.ProjectsWithJournal = 4
	return s
}

// TestWriteCostIsConstantInTreeSize is §2.3's invariant, stated as an
// experiment: "nothing walks a subtree on write, nothing walks to root on
// write". The same mutation is run against a tiny tree and against one that is
// two orders of magnitude bigger, and the two must touch the same files.
//
// Comparing the *paths* rather than counting them is what makes this a real
// test. A count could stay the same while the set changed, and the failure this
// is guarding against — a mutation that reaches one file further for every
// sibling or ancestor there is — shows up in the set first.
//
// Each case gets its own subject so that one tree can serve all of them. Running
// them against a fresh pair would be cleaner in the abstract and would cost a
// minute of wall clock per case for no more evidence: none of these mutations
// changes what any of the others writes.
func TestWriteCostIsConstantInTreeSize(t *testing.T) {
	large := scale.Default()
	smallRoot, _ := plantAndRebuild(t, small())
	largeRoot, _ := plantAndRebuild(t, large)

	cases := []struct {
		name string
		run  func(*mutate.Env) (mutate.Result, error)
	}{
		{
			name: "add a project",
			run: func(e *mutate.Env) (mutate.Result, error) {
				return e.Add(loc(t, "projects.brand-new"), fieldsFor("Brand new", "A new project."))
			},
		},
		{
			name: "note on a project",
			run: func(e *mutate.Env) (mutate.Result, error) {
				return e.Note(loc(t, "projects.p0001"), "something happened", "", false)
			},
		},
		{
			name: "measure a key-result",
			run: func(e *mutate.Env) (mutate.Result, error) {
				return e.Measure(loc(t, "projects.p0002.objectives.o0.key-results.k0"), "500/1000", "", "")
			},
		},
		{
			name: "set a field",
			run: func(e *mutate.Env) (mutate.Result, error) {
				return e.Set(loc(t, "projects.p0003"), fields(kindmeta.FieldPriority, "high"), "")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			smallRes, err := tc.run(mutate.NewEnv(smallRoot, clock.Fixed{At: fixedNow(t)}))
			if err != nil {
				t.Fatalf("on the small tree: %v", err)
			}
			largeRes, err := tc.run(mutate.NewEnv(largeRoot, clock.Fixed{At: fixedNow(t)}))
			if err != nil {
				t.Fatalf("on the large tree: %v", err)
			}

			if got, want := strings.Join(largeRes.Wrote, "\n"), strings.Join(smallRes.Wrote, "\n"); got != want {
				t.Errorf("a tree of %d entities wrote a different file set from one of %d:\n%s\nwant\n%s",
					large.Entities(), small().Entities(), got, want)
			}
			t.Logf("%s touched %d files on both trees", tc.name, len(smallRes.Wrote))
		})
	}
}

// TestReadPathsOpenOnlyTheJournalsTheyNeed is the other half of the cost claim,
// and it is asserted without instrumenting anything: every journal the command
// should not touch is poisoned with a line that is not an event, so a read path
// that opened one would fail rather than merely be slower.
//
// That is a stronger assertion than a counter would give. A counter says how
// many files were opened; this says *which*, and it fails loudly rather than
// drifting when somebody adds a read that looks harmless.
func TestReadPathsOpenOnlyTheJournalsTheyNeed(t *testing.T) {
	root, _ := plantAndRebuild(t, scale.Shape{
		Projects: 5, ObjectivesPerProj: 1, KeyResultsPerObj: 1,
		Areas: 5, Resources: 5, MeasurementsPerKR: 2, NotesPerProject: 2, ProjectsWithJournal: 5,
	})

	// Every journal outside projects.p0000's own subtree becomes unreadable, so
	// a read that strayed would fail rather than merely cost more.
	subtree := filepath.Join(root, "projects", "p0000")
	spared := filepath.Join(subtree, ".para", "logs")
	poisoned := poisonJournalsOutside(t, root, subtree)
	if poisoned == 0 {
		t.Fatal("nothing was poisoned; the fixture has no journals to poison")
	}

	env := view.NewEnv(root, clock.Fixed{At: fixedNow(t)})

	// `show` reads the subject's own journal for §3.6's attention — and nothing
	// else's, not a sibling's and not an ancestor's.
	if _, err := env.Load(loc(t, "projects.p0000")); err != nil {
		t.Errorf("show on projects.p0000 read a journal outside it: %v", err)
	}

	// A scoped `list` reads the journals under the locator and stops there,
	// which is §17's answer to "--match reads every journal": a locator is the
	// scoping answer, and it has to actually scope the reads.
	if _, err := query.List(env, query.Options{Scope: loc(t, "projects.p0000")}); err != nil {
		t.Errorf("a scoped list read a journal outside its scope: %v", err)
	}

	// `path` answers from the locator and the filesystem (§14) and opens no
	// journal at all, its own included — so it survives with everything
	// poisoned.
	alsoSpared := poisonJournal(t, spared)
	if _, err := tree.ResolvePath(root, loc(t, "projects.p0001")); err != nil {
		t.Errorf("resolving a locator read a journal: %v", err)
	}

	// And with its own journal poisoned, `show` fails — which is what proves the
	// two assertions above are about *which* journals were read rather than
	// about code paths that read none.
	if _, err := env.Load(loc(t, "projects.p0000")); err == nil {
		t.Error("show did not read the subject's own journal, so the assertions above prove nothing")
	}
	restore(t, alsoSpared)
}

// TestScaleTimings prints what `doctor`, `rebuild` and `list` cost over a tree of
// a few thousand entities. The numbers go in the README (§24's table is about
// counts; this is about seconds).
func TestScaleTimings(t *testing.T) {
	if testing.Short() {
		t.Skip("the scale timings build a few-thousand-entity tree")
	}
	s := scale.Default()
	root, rebuildTime := plantAndRebuild(t, s)

	start := time.Now()
	second, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	idempotentTime := time.Since(start)
	if !second.Empty() {
		t.Errorf("the second rebuild over %d entities was not a no-op: %v", s.Entities(), second.Changed)
	}

	env := view.NewEnv(root, clock.Fixed{At: fixedNow(t)})

	start = time.Now()
	rep, err := doctor.Run(env, doctor.Options{})
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	doctorTime := time.Since(start)
	if !rep.Clean() {
		var lines []string
		for i, f := range rep.Findings {
			if i == 5 {
				lines = append(lines, fmt.Sprintf("… and %d more", len(rep.Findings)-5))
				break
			}
			lines = append(lines, string(f.Kind)+" "+f.Path+" — "+f.Detail)
		}
		t.Fatalf("a planted-and-rebuilt tree is not clean:\n%s", strings.Join(lines, "\n"))
	}

	start = time.Now()
	listed, err := query.List(env, query.Options{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	listTime := time.Since(start)

	start = time.Now()
	scoped, err := query.List(env, query.Options{Scope: loc(t, "projects.p0000")})
	if err != nil {
		t.Fatalf("scoped list: %v", err)
	}
	scopedTime := time.Since(start)

	t.Logf("tree: %d entities (%d rows listed)", s.Entities(), len(listed.Entities))
	t.Logf("rebuild (cold, writes everything): %s", rebuildTime.Round(time.Millisecond))
	t.Logf("rebuild (idempotent, writes nothing): %s", idempotentTime.Round(time.Millisecond))
	t.Logf("doctor (whole tree): %s", doctorTime.Round(time.Millisecond))
	t.Logf("list (whole tree, %d rows): %s", len(listed.Entities), listTime.Round(time.Millisecond))
	t.Logf("list (one project, %d rows): %s", len(scoped.Entities), scopedTime.Round(time.Millisecond))
}

// fieldsFor is the name/description every kind requires (§15).
func fieldsFor(name, description string) mutate.Fields {
	var f mutate.Fields
	f.Set(kindmeta.FieldName, name)
	f.Set(kindmeta.FieldDescription, description)
	return f
}

func fields(field kindmeta.Field, value string) mutate.Fields {
	var f mutate.Fields
	f.Set(field, value)
	return f
}

// poisonJournalsOutside writes a line that is not a journal event into every
// logs/ directory outside the given subtree, and reports how many it poisoned.
//
// The whole subtree is spared rather than one directory, because a scoped read
// is entitled to every journal *under* the locator — §21.1's "under" is
// inclusive and a project's objectives are inside it. Sparing only the project's
// own journal would make the test assert something the design does not claim.
func poisonJournalsOutside(t *testing.T, root, subtree string) int {
	t.Helper()
	count := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || filepath.Base(path) != "logs" {
			return err
		}
		if path == subtree || strings.HasPrefix(path, subtree+string(filepath.Separator)) {
			return nil
		}
		poisonJournal(t, path)
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return count
}

// poisonJournal makes one journal unreadable and returns the file it wrote, so a
// caller can take it away again.
func poisonJournal(t *testing.T, logs string) string {
	t.Helper()
	path := filepath.Join(logs, "29990101T000000Z.jsonl")
	if err := os.WriteFile(path, []byte("this is not a journal event\n"), 0o644); err != nil {
		t.Fatalf("poisoning %s: %v", logs, err)
	}
	return path
}

func restore(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing %s: %v", path, err)
	}
}
