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

// TestNoteAppendsAndRewritesActivity: a note writes one line in one journal,
// re-derives ACTIVITY.md, and — as of §28.4's write-through — caches the
// attention it just moved into state.toml.
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
	if got := read(t, root, "areas/health/.para/state.toml"); !strings.Contains(got, `attention = "2026-03-05T17:00:00Z"`) {
		t.Errorf("state.toml = %q, want the note's instant cached as attention (§28.4)", got)
	}
}

// TestNoteDryRunWritesNothingButReportsWhatNoteWould is para-ato: a rehearsal
// must report the identical file list a real Note returns, and touch nothing
// on disk while doing it.
func TestNoteDryRunWritesNothingButReportsWhatNoteWould(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying in one piece.")); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)

	dry, err := e.NoteDryRun(loc(t, "areas.health"), "swapped the tempo block for intervals", "", false)
	if err != nil {
		t.Fatalf("NoteDryRun: %v", err)
	}
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("NoteDryRun wrote %v, want nothing", changed)
	}

	real, err := e.Note(loc(t, "areas.health"), "swapped the tempo block for intervals", "", false)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	assertEqual(t, "NoteDryRun's Wrote", dry.Wrote, real.Wrote)
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
	// state.toml was already cached by Add with attention=created; this note
	// lands at the same instant, so write-through has nothing to rewrite.
	assertEqual(t, "wrote", res.Wrote, []string{
		"areas/health/.para/logs/20260305T170000Z.jsonl",
		"areas/health/ACTIVITY.md",
	})
	if got := read(t, root, "areas/health/.para/state.toml"); !strings.Contains(got, `attention = "2026-03-05T17:00:00Z"`) {
		t.Errorf("state.toml = %q, want attention backfilled to created, not the no-attention note's instant", got)
	}

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

// TestNoteNoAttentionLeavesAnAlreadyCachedAttentionAlone proves §28.4's
// write-through does not fold a --no-attention note into the cache just
// because it is the newest event: a later no-attention note must not move
// state.toml's attention past the ordinary note that last set it.
func TestNoteNoAttentionLeavesAnAlreadyCachedAttentionAlone(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying in one piece.")); err != nil {
		t.Fatal(err)
	}
	// Backdated so both notes below can land after created (this test's fixed
	// now, 2026-03-05) without being refused as being in the future.
	if _, err := e.Set(loc(t, "areas.health"), fields("created", "2026-01-01"), ""); err != nil {
		t.Fatalf("Set created: %v", err)
	}
	if _, err := e.Note(loc(t, "areas.health"), "an ordinary note", "2026-03-01", false); err != nil {
		t.Fatalf("Note: %v", err)
	}

	// Later than the ordinary note above by `at` — if write-through folded a
	// --no-attention event into the cache just for being newest, this would
	// wrongly move attention to 2026-03-04.
	res, err := e.Note(loc(t, "areas.health"), "retired the old beads IDs", "2026-03-04", true)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}
	for _, path := range res.Wrote {
		if strings.HasSuffix(path, "state.toml") {
			t.Errorf("wrote %v, want state.toml untouched since attention did not move", res.Wrote)
		}
	}
	if got := read(t, root, "areas/health/.para/state.toml"); !strings.Contains(got, `attention = "2026-03-01T08:00:00Z"`) {
		t.Errorf("state.toml = %q, want attention still at the ordinary note's instant", got)
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

// TestMeasureLeavesReadmeAloneButCachesAttention: a key-result stores no
// `current` in its README frontmatter (§2.5), so that file cannot have
// changed and is not rewritten. state.toml is the one truth file that does
// change, as of §28.4's write-through reversal: a measurement is one of the
// two events that move attention (§3.6), so the reading's own instant is
// exactly what gets cached.
func TestMeasureLeavesReadmeAloneButCachesAttention(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	kr := addSignups(t, e)
	// Backdated so the reading (2026-03-03) lands after created and can
	// actually move attention forward from it — Add's created otherwise
	// defaults to this test's fixed now (2026-03-05), which the reading may
	// not follow without being refused as being in the future.
	if _, err := e.Set(loc(t, kr), fields("created", "2026-01-01"), ""); err != nil {
		t.Fatalf("Set created: %v", err)
	}
	before := snapshot(t, root)

	res, err := e.Measure(loc(t, kr), "880/11000", "2026-03-03", "")
	if err != nil {
		t.Fatalf("Measure: %v", err)
	}
	for _, path := range changedPaths(t, before, snapshot(t, root)) {
		if strings.HasSuffix(path, "README.md") {
			t.Errorf("a measurement rewrote %s, which carries no derived value", path)
		}
	}
	dir := "projects/acme/objectives/q1-growth/key-results/signups"
	statePath := dir + "/.para/state.toml"
	found := false
	for _, path := range res.Wrote {
		found = found || path == statePath
	}
	if !found {
		t.Errorf("wrote %v, want %s (§28.4 caches the reading's attention)", res.Wrote, statePath)
	}
	if got := read(t, root, statePath); !strings.Contains(got, `attention = "2026-03-03T08:00:00Z"`) {
		t.Errorf("state.toml = %q, want the reading's own instant cached as attention", got)
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
