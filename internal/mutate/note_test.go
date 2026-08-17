package mutate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/mutate"
)

// addSignups builds §26's key-result: start 480/9000, target 2000/12000, due
// 2026-09-30 — the one whose arithmetic §4.2 works through.
func addSignups(t *testing.T, e *mutate.Env) string {
	t.Helper()
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1-growth"), fields("name", "Grow signups", "description", "Move the top of the funnel.")); err != nil {
		t.Fatal(err)
	}
	kr := "projects.acme.objectives.q1-growth.key-results.signups"
	if _, err := e.Add(loc(t, kr), fields(
		"name", "Weekly signups", "type", "ratio",
		"start", "480/9000", "target", "2000/12000", "due", "2026-09-30",
	)); err != nil {
		t.Fatal(err)
	}
	return kr
}

// TestNoteAppendsAndRewritesActivity: a note writes one line in one journal and
// re-derives one file (§3.3's exact boundary).
func TestNoteAppendsAndRewritesActivity(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying in one piece.")); err != nil {
		t.Fatal(err)
	}

	res, err := e.Note(loc(t, "areas.health"), "swapped the tempo block for intervals", "", false)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	assertEqual(t, "wrote", res.Wrote, []string{
		"areas/health/.para/logs/20260305T170000Z.jsonl",
		"areas/health/ACTIVITY.md",
	})

	want := "# Activity\n\n## 2026-03-05\n- Note: swapped the tempo block for intervals.\n- Created.\n"
	if got := read(t, root, "areas/health/ACTIVITY.md"); got != want {
		t.Errorf("ACTIVITY.md =\n%q\nwant\n%q", got, want)
	}
}

// TestNoteNoAttentionRecordsWithTheFlagSet is para-c3u: --no-attention records
// the note exactly as a normal one (§18.6) but flags the journal event so
// Attention (§3.6) skips it — see view.TestAttentionKindAndNoteNameWhatSetTheClock
// for the clock consequence end to end.
func TestNoteNoAttentionRecordsWithTheFlagSet(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying in one piece.")); err != nil {
		t.Fatal(err)
	}

	res, err := e.Note(loc(t, "areas.health"), "retired the old beads IDs from the ported entries", "", true)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	if !res.NoteRecorded {
		t.Error("NoteRecorded = false, want true even with --no-attention")
	}
	assertEqual(t, "wrote", res.Wrote, []string{
		"areas/health/.para/logs/20260305T170000Z.jsonl",
		"areas/health/ACTIVITY.md",
	})

	got := read(t, root, "areas/health/.para/logs/20260305T170000Z.jsonl")
	if !strings.Contains(got, `"no_attention":true`) {
		t.Errorf("journal line = %q, want no_attention:true", got)
	}

	// The note still appears in ACTIVITY.md exactly as an ordinary one would:
	// --no-attention changes what the clock reads, not what happened.
	want := "# Activity\n\n## 2026-03-05\n- Note: retired the old beads IDs from the ported entries.\n- Created.\n"
	if got := read(t, root, "areas/health/ACTIVITY.md"); got != want {
		t.Errorf("ACTIVITY.md =\n%q\nwant\n%q", got, want)
	}
}

// TestNoteAtBackdatesInTheLocalOffset is §15.1: what you type is local, what is
// stored is UTC. A bare date is midnight local, which in -08:00 is 08:00Z.
func TestNoteAtBackdatesInTheLocalOffset(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Health.")); err != nil {
		t.Fatal(err)
	}

	if _, err := e.Note(loc(t, "areas.health"), "backdated", "2026-03-01", false); err != nil {
		t.Fatalf("Note: %v", err)
	}
	got := read(t, root, "areas/health/.para/logs/20260301T080000Z.jsonl")
	if !strings.HasPrefix(got, `{"at":"2026-03-01T08:00:00Z"`) {
		t.Errorf("journal line = %q, want a UTC instant of 08:00Z", got)
	}
}

func TestNoteInTheFutureIsRefused(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Health.")); err != nil {
		t.Fatal(err)
	}
	_, err := e.Note(loc(t, "areas.health"), "later", "2027-01-01", false)
	errorContains(t, err, "in the future")
}

// TestMeasureDerivesProgressFromTheBaseline is §4.2's worked example: start is
// subtracted from both sides, so 880/11000 against start 480/9000 and target
// 2000/12000 is 0.2353, not 0.48.
func TestMeasureDerivesProgressFromTheBaseline(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	kr := addSignups(t, e)

	res, err := e.Measure(loc(t, kr), "880/11000", "2026-03-03", "")
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if res.Measured == nil || !res.Measured.HasProgress {
		t.Fatalf("measured = %+v, want derived progress", res.Measured)
	}
	if got := res.Measured.Progress; got < 0.2352 || got > 0.2354 {
		t.Errorf("progress = %v, want ≈0.2353 (§4.2)", got)
	}
	if got := res.Measured.Value.Decimal; got < 0.0799 || got > 0.0801 {
		t.Errorf("decimal = %v, want ≈0.0800", got)
	}
	// No at-risk-pace is set anywhere, so the check never fires (§7).
	if res.Measured.Status != krvalue.StatusOnTrack {
		t.Errorf("status = %q, want on-track with no at-risk-pace set", res.Measured.Status)
	}

	assertEqual(t, "wrote", res.Wrote, []string{
		"projects/acme/objectives/q1-growth/key-results/signups/.para/logs/20260303T080000Z.jsonl",
		"projects/acme/objectives/q1-growth/key-results/signups/ACTIVITY.md",
		"projects/acme/objectives/q1-growth/key-results/signups/MEASUREMENTS.csv",
	})

	want := "at,value,decimal,progress,note\n2026-03-03T08:00:00Z,880/11000,0.0800,0.2353,\n"
	if got := read(t, root, "projects/acme/objectives/q1-growth/key-results/signups/MEASUREMENTS.csv"); got != want {
		t.Errorf("MEASUREMENTS.csv =\n%q\nwant\n%q", got, want)
	}
}

// TestMeasureLeavesTruthAlone: §2.3's prose names state.toml and README.md, but
// a key-result stores no `current` (§2.5), so neither file can have changed and
// neither is rewritten. What a mutation prints must be what it wrote (§23).
func TestMeasureLeavesTruthAlone(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	kr := addSignups(t, e)
	before := snapshot(t, root)

	if _, err := e.Measure(loc(t, kr), "880/11000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}
	for _, path := range changedPaths(t, before, snapshot(t, root)) {
		if strings.HasSuffix(path, "state.toml") || strings.HasSuffix(path, "README.md") {
			t.Errorf("a measurement rewrote %s, which carries no derived value", path)
		}
	}
}

// TestMeasureRefusesADuplicateInstant is §3.1's uniqueness rule, compared at
// the exact stored instant rather than the day — which is why a bare date twice
// collides and a date plus a time does not.
func TestMeasureRefusesADuplicateInstant(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	kr := addSignups(t, e)

	if _, err := e.Measure(loc(t, kr), "880/11000", "2026-03-03", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}
	_, err := e.Measure(loc(t, kr), "900/11000", "2026-03-03", "")
	errorContains(t, err, "a measurement already exists at 2026-03-03T08:00:00Z")
	errorContains(t, err, "2026-03-03T00:00:00-08:00 local")

	// A finer precision on the same day is a different instant, so it lands.
	if _, err := e.Measure(loc(t, kr), "900/11000", "2026-03-03T09:02", ""); err != nil {
		t.Errorf("a measurement at a different instant on the same day was refused: %v", err)
	}
}

func TestMeasureRefusesTheWrongGrammar(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	kr := addSignups(t, e)

	// §26: "value 0.08 is not a ratio (type ratio expects …)".
	_, err := e.Measure(loc(t, kr), "0.08", "", "")
	errorContains(t, err, "value 0.08 is not a ratio")
	errorContains(t, err, "<numerator>/<denominator>")
}

func TestMeasureRefusesANonKeyResult(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	_, err := e.Measure(loc(t, "projects.acme"), "10", "", "")
	errorContains(t, err, "only a key-result takes measurements")
}

// TestMeasureReadsAtRiskPaceThroughTheChain proves the threshold comes from
// §7's resolution rather than from a constant, and that an unset one means the
// check never fires.
func TestMeasureReadsAtRiskPaceThroughTheChain(t *testing.T) {
	root := plantTree(t)
	writeFiles(t, root, map[string]string{
		".para/config.toml": "key-result.at-risk-pace = 0.8\n",
	})
	e := env(t, root)
	kr := addSignups(t, e)

	// Pace is undefined while elapsed is 0 (§4.2), and an undefined pace never
	// reads at-risk — so the key-result is backdated to make part of its window
	// genuinely spent. The reading is the baseline itself, so progress is 0 and
	// pace is 0, which is below the threshold.
	if _, err := e.Set(loc(t, kr), fields("created", "2026-01-01"), ""); err != nil {
		t.Fatalf("Set created: %v", err)
	}
	res, err := e.Measure(loc(t, kr), "480/9000", "", "")
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if res.Measured.Status != krvalue.StatusAtRisk {
		t.Errorf("status = %q, want at-risk with at-risk-pace 0.8 and no progress", res.Measured.Status)
	}
}

// TestEventsAreWrittenToTheSecond pins the precision every timestamp in the
// design is written at (§3.1, §3.4, §4.4, §8.3).
//
// A fractional instant would be a real defect rather than an untidy one: §3.1's
// uniqueness check compares "the exact stored instant", so two readings a
// millisecond apart are distinct events — and MEASUREMENTS.csv renders both to
// the same second, producing a duplicate row that no rebuild could remove,
// because both events are genuinely there.
func TestEventsAreWrittenToTheSecond(t *testing.T) {
	root := plantTree(t)
	fractional := time.Date(2026, time.March, 5, 9, 0, 0, 878271000, pacific)
	e := mutate.NewEnv(root, clock.Fixed{At: fractional})

	kr := addSignups(t, e)
	if _, err := e.Measure(loc(t, kr), "880/11000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}

	// A second reading at the same wall-clock instant is a collision, not a
	// distinct event a microsecond later.
	_, err := e.Measure(loc(t, kr), "900/11000", "", "")
	errorContains(t, err, "a measurement already exists")

	dir := "projects/acme/objectives/q1-growth/key-results/signups"
	if got := read(t, root, dir+"/.para/logs/20260305T170000Z.jsonl"); !strings.Contains(got, `"at":"2026-03-05T17:00:00Z"`) {
		t.Errorf("journal line = %q, want a whole-second instant", got)
	}
	if got := read(t, root, dir+"/.para/state.toml"); !strings.Contains(got, `created = "2026-03-05T17:00:00Z"`) {
		t.Errorf("state.toml = %q, want a whole-second created", got)
	}
}
