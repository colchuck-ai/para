package render_test

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

var pacific = time.FixedZone("PST", -8*3600)

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.ParseInLocation("2006-01-02T15:04:05", s, pacific)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestActivityRendersTheWorkedExampleShape(t *testing.T) {
	// §3.5's own example, transcribed: title, newest day first, one section
	// per day with events, and days with no events absent.
	in := render.In{
		Locator: loc(t, "projects.acme-migration.objectives"),
		Kind:    kindmeta.KindContainer,
		State:   truth.State{Name: "Objectives", Created: "2026-01-05T08:00:00-08:00"},
		Events: []journal.Event{
			journal.NewChild(ts(t, "2026-01-05T11:00:00"), journal.ChildOpAdded, "q1-growth", "", "", ""),
			journal.NewNote(ts(t, "2026-01-04T17:40:00"), "waiting on the ingest team"),
		},
	}

	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	want := `# Activity

## 2026-01-05
- Added objective **q1-growth**.
- Created.

## 2026-01-04
- Note: waiting on the ingest team.
`
	if string(got) != want {
		t.Errorf("Activity =\n%s\nwant\n%s", got, want)
	}
}

func TestActivityMeasurementLineCarriesDecimalAndProgress(t *testing.T) {
	// §3.5's measurement line: the reading as logged, its decimal, and the
	// progress it represents. The entity is the key-result itself, so the line
	// names the reading rather than repeating its own id — matching §26's
	// activity output, where the locator is a separate column.
	in := render.In{
		Locator: loc(t, "projects.acme.objectives.q1-growth.key-results.signups"),
		Kind:    kindmeta.KindKeyResult,
		State: truth.State{
			Name: "Weekly signups", Type: "ratio",
			Start: "480/9000", Target: "2000/12000",
			Created: "2026-01-01T08:15:00-08:00",
		},
		Events: []journal.Event{
			journal.NewMeasurement(ts(t, "2026-01-03T09:02:11"), "880/11000", ""),
		},
	}

	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	// §4.2's formula is (current − start) / (target − start), which for these
	// values is (0.0800 − 0.0533) / (0.1667 − 0.0533) = 0.2353. The design's
	// own worked lines print 0.47 for the same three values, which is
	// current ÷ target with `start` dropped — the prose defines the formula
	// twice over (§4.2, and again in §16.1's "progress" line) and only the
	// arithmetic in the examples disagrees, so the formula wins and krvalue
	// already implements it.
	want := "- Measured 880/11000 (8.0%) — 24% of target.\n"
	if !contains(string(got), want) {
		t.Errorf("Activity =\n%s\nwant it to contain\n%s", got, want)
	}
}

func TestActivityLinesPerEventKind(t *testing.T) {
	base := render.In{
		Locator: loc(t, "projects.acme"),
		Kind:    kindmeta.KindProject,
		State:   truth.State{Name: "Acme", Created: "2026-03-05T08:00:00-08:00"},
	}

	cases := []struct {
		name  string
		event journal.Event
		want  string
	}{
		{
			"change between two values",
			journal.NewChange(ts(t, "2026-03-05T09:00:00"), "status", "planned", "in-progress", ""),
			"- Changed **status** from planned to in-progress.\n",
		},
		{
			"change carrying a note",
			journal.NewChange(ts(t, "2026-03-05T09:00:00"), "status", "in-progress", "blocked", "waiting on the ingest team"),
			"- Changed **status** from in-progress to blocked — waiting on the ingest team.\n",
		},
		{
			"change from nothing is a set",
			journal.NewChange(ts(t, "2026-03-05T09:00:00"), "due", "", "2026-09-30", ""),
			"- Set **due** to 2026-09-30.\n",
		},
		{
			"change to nothing is an unset",
			journal.NewChange(ts(t, "2026-03-05T09:00:00"), "due", "2026-09-30", "", ""),
			"- Unset **due** (was 2026-09-30).\n",
		},
		{
			"note already ending in a period does not gain a second",
			journal.NewNote(ts(t, "2026-03-05T09:00:00"), "waiting on the ingest team."),
			"- Note: waiting on the ingest team.\n",
		},
		{
			"a multi-line note is flattened to one line",
			journal.NewNote(ts(t, "2026-03-05T09:00:00"), "first line\nsecond line"),
			"- Note: first line second line.\n",
		},
		{
			"child added names the child's kind",
			journal.NewChild(ts(t, "2026-03-05T09:00:00"), journal.ChildOpAdded, "objectives", "", "", ""),
			"- Added **objectives**.\n",
		},
		{
			"child moved names both locators",
			journal.NewChild(ts(t, "2026-03-05T09:00:00"), journal.ChildOpMoved, "q1", "projects.acme.objectives.q1", "projects.beta.objectives.q1", ""),
			"- Moved **q1** from projects.acme.objectives.q1 to projects.beta.objectives.q1.\n",
		},
		{
			"child archived",
			journal.NewChild(ts(t, "2026-03-05T09:00:00"), journal.ChildOpArchived, "objectives", "", "", ""),
			"- Archived **objectives**.\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := base
			in.Events = []journal.Event{tc.event}
			got, err := render.Activity.Render(in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !contains(string(got), tc.want) {
				t.Errorf("Activity =\n%s\nwant it to contain\n%s", got, tc.want)
			}
		})
	}
}

func TestActivityWithinADayIsNewestFirst(t *testing.T) {
	// §16.3: log's direction matches ACTIVITY.md's "so the two never read in
	// opposite orders". One direction, throughout.
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Created: "2026-01-01"},
		Events: []journal.Event{
			journal.NewNote(ts(t, "2026-03-05T09:00:00"), "morning"),
			journal.NewNote(ts(t, "2026-03-05T17:00:00"), "evening"),
		},
	}

	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := `# Activity

## 2026-03-05
- Note: evening.
- Note: morning.

## 2026-01-01
- Created.
`
	if string(got) != want {
		t.Errorf("Activity =\n%s\nwant\n%s", got, want)
	}
}

func TestActivityIncrementalLeavesPriorDaysByteForByte(t *testing.T) {
	// §3.5: prior days are already written and are never recomputed. The
	// proof is a prior section with bytes no renderer would produce — if the
	// incremental path touched it, this test would normalise it away.
	existing := []byte(`# Activity

## 2026-03-04
- Something a future para version words differently.

## 2026-03-01
- Created.
`)
	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Created: "2026-03-01"},
		Events:   []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "today")},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": existing},
	}

	got, err := render.ActivityRenderer{Days: []string{"2026-03-05"}}.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := `# Activity

## 2026-03-05
- Note: today.

## 2026-03-04
- Something a future para version words differently.

## 2026-03-01
- Created.
`
	if string(got) != want {
		t.Errorf("incremental Activity =\n%s\nwant\n%s", got, want)
	}
}

func TestActivityIncrementalReplacesTodaysSectionRatherThanAppendingToIt(t *testing.T) {
	existing := []byte(`# Activity

## 2026-03-05
- Note: first.

## 2026-03-01
- Created.
`)
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Created: "2026-03-01"},
		Events: []journal.Event{
			journal.NewNote(ts(t, "2026-03-05T09:00:00"), "first"),
			journal.NewNote(ts(t, "2026-03-05T10:00:00"), "second"),
		},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": existing},
	}

	got, err := render.ActivityRenderer{Days: []string{"2026-03-05"}}.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := `# Activity

## 2026-03-05
- Note: second.
- Note: first.

## 2026-03-01
- Created.
`
	if string(got) != want {
		t.Errorf("incremental Activity =\n%s\nwant\n%s", got, want)
	}
}

func TestActivityIncrementalOnAnAbsentFileMatchesFull(t *testing.T) {
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Created: "2026-03-05"},
		Events:  []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "first")},
	}

	full, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("full Render: %v", err)
	}
	incremental, err := render.ActivityRenderer{Days: []string{"2026-03-05"}}.Render(in)
	if err != nil {
		t.Fatalf("incremental Render: %v", err)
	}
	if string(full) != string(incremental) {
		t.Errorf("incremental =\n%s\nfull =\n%s", incremental, full)
	}
}

// TestActivityIncrementalAndFullAgreeOverGeneratedHistories is this phase's
// central property (implementation plan, Phase 6): replaying a history one
// mutation at a time through the incremental path must land on exactly the
// bytes a full re-derivation produces. Without it, §10's dated drift report
// would be reporting the renderer's own disagreement with itself rather than
// real drift.
func TestActivityIncrementalAndFullAgreeOverGeneratedHistories(t *testing.T) {
	for seed := int64(0); seed < 40; seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			history := generateHistory(t, rng)

			base := render.In{
				Locator: loc(t, "projects.acme.objectives.q1"),
				Kind:    kindmeta.KindObjective,
				State: truth.State{
					Name:    "Grow signups",
					Created: "2026-03-01T08:00:00-08:00",
				},
			}

			// Step zero is `add`, which writes ACTIVITY.md for the first time
			// and so must re-derive the day `created` names (§18.1, and
			// CreatedDay's doc).
			path := "projects/acme/objectives/q1/ACTIVITY.md"
			current, err := render.ActivityRenderer{Days: []string{render.CreatedDay(base)}}.Render(base)
			if err != nil {
				t.Fatalf("creation step: %v", err)
			}

			// Then each step appends one event and re-derives only the days
			// that event fell on, exactly as a mutation does.
			for i := range history {
				step := base
				step.Events = history[:i+1]
				step.Existing = map[string][]byte{path: current}

				current, err = render.ActivityRenderer{Days: render.DaysOf(history[i : i+1])}.Render(step)
				if err != nil {
					t.Fatalf("incremental step %d: %v", i, err)
				}
			}

			whole := base
			whole.Events = history
			full, err := render.Activity.Render(whole)
			if err != nil {
				t.Fatalf("full Render: %v", err)
			}

			if string(current) != string(full) {
				t.Errorf("replayed incrementally =\n%s\nfull re-derivation =\n%s", current, full)
			}
		})
	}
}

// generateHistory builds a chronological run of events across several days,
// mixing the kinds an objective can carry so that the day sections it produces
// are non-trivial: several events on one day, days with a single event, and gaps
// of days with none. It spans enough events that a real journal would have
// rotated partway through, which is the case the equivalence property most
// needs to cover.
//
// No measurement events: they land only on a key-result (§3.1), and a
// key-result renders in full mode anyway (see ActivityRenderer's doc), so a
// measurement here would be testing a combination that cannot occur.
func generateHistory(t *testing.T, rng *rand.Rand) []journal.Event {
	t.Helper()

	n := 5 + rng.Intn(20)
	at := ts(t, "2026-03-01T08:00:00")
	events := make([]journal.Event, 0, n)
	for i := 0; i < n; i++ {
		at = at.Add(time.Duration(1+rng.Intn(40)) * time.Hour)
		switch rng.Intn(4) {
		case 0:
			events = append(events, journal.NewNote(at, fmt.Sprintf("note %d", i)))
		case 1:
			events = append(events, journal.NewNote(at, fmt.Sprintf("note %d\nwith a second line", i)))
		case 2:
			events = append(events, journal.NewChange(at, "due", "", "2026-09-30", ""))
		default:
			events = append(events, journal.NewChild(at, journal.ChildOpAdded, fmt.Sprintf("child-%d", i), "", "", ""))
		}
	}
	return events
}

func TestActivityRejectsAnExistingFileThatIsNotNewestFirst(t *testing.T) {
	// Newest-first is the invariant the splice relies on; a file that violates
	// it would silently reorder history, so it is refused and named instead.
	existing := []byte("# Activity\n\n## 2026-03-01\n- Created.\n\n## 2026-03-04\n- Note: out of order.\n")
	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Created: "2026-03-01"},
		Events:   []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "today")},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": existing},
	}

	if _, err := (render.ActivityRenderer{Days: []string{"2026-03-05"}}).Render(in); err == nil {
		t.Error("Render accepted an out-of-order ACTIVITY.md, want an error")
	}
}

func TestDaysOfDeduplicatesAndSortsNewestFirst(t *testing.T) {
	got := render.DaysOf([]journal.Event{
		journal.NewNote(ts(t, "2026-03-01T09:00:00"), "a"),
		journal.NewNote(ts(t, "2026-03-05T09:00:00"), "b"),
		journal.NewNote(ts(t, "2026-03-01T17:00:00"), "c"),
	})
	want := []string{"2026-03-05", "2026-03-01"}
	if len(got) != len(want) {
		t.Fatalf("DaysOf = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DaysOf = %v, want %v", got, want)
		}
	}
}

func TestActivityGroupsByTheEventsOwnOffset(t *testing.T) {
	// A day is the date in the event's own recorded offset (§15.1 stores one),
	// not in the host's zone — which is what keeps grouping reproducible on a
	// machine in a different timezone from the one that wrote the event.
	tokyo := time.FixedZone("JST", 9*3600)
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Created: "2026-03-01"},
		Events: []journal.Event{
			// The same instant, in two zones: 23:00 Pacific on the 4th is
			// 16:00 Tokyo on the 5th.
			journal.NewNote(time.Date(2026, 3, 5, 16, 0, 0, 0, tokyo), "tokyo"),
		},
	}

	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !contains(string(got), "## 2026-03-05\n- Note: tokyo.\n") {
		t.Errorf("Activity =\n%s\nwant the event grouped under its own offset's day", got)
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func TestActivityIncrementalKeepsPriorDaysFromACRLFFile(t *testing.T) {
	// A checkout with git's core.autocrlf on hands back CRLF. ACTIVITY.md is
	// wholly generated (§2.2), so its line endings carry nothing — but a parser
	// that failed to recognise a CRLF day header would drop every prior day on
	// the next mutation, which is silent history loss.
	existing := strings.ReplaceAll(
		"# Activity\n\n## 2026-03-04\n- Note: yesterday.\n\n## 2026-03-01\n- Created.\n",
		"\n", "\r\n")
	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Created: "2026-03-01"},
		Events:   []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "today")},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": []byte(existing)},
	}

	got, err := render.ActivityRenderer{Days: []string{"2026-03-05"}}.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := `# Activity

## 2026-03-05
- Note: today.

## 2026-03-04
- Note: yesterday.

## 2026-03-01
- Created.
`
	if string(got) != want {
		t.Errorf("incremental Activity =\n%q\nwant\n%q", got, want)
	}
}

func TestActivityRefusesToSilentlyDropContentItCannotParse(t *testing.T) {
	// §26's own drift example is `echo "hand-edited" >> ACTIVITY.md`. Appended
	// at the end it lands inside the last day's section and survives; placed
	// before the first heading there is nowhere for it to go, and dropping it
	// would be the one failure mode that loses history.
	existing := []byte("# Activity\n\nhand-edited\n\n## 2026-03-01\n- Created.\n")
	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Created: "2026-03-01"},
		Events:   []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "today")},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": []byte(existing)},
	}

	_, err := render.ActivityRenderer{Days: []string{"2026-03-05"}}.Render(in)
	if err == nil {
		t.Fatal("Render silently accepted unparseable content, want an error")
	}
	if !strings.Contains(err.Error(), "para rebuild") {
		t.Errorf("error = %q, want it to name the repair", err)
	}
}

func TestActivityIncrementalAcceptsAFreshlyCreatedFile(t *testing.T) {
	// The title alone is what `add` leaves behind for an entity with no events
	// yet, so it must not trip the guard above.
	in := render.In{
		Locator:  loc(t, "areas.health"),
		Kind:     kindmeta.KindArea,
		State:    truth.State{Name: "Health", Created: "2026-03-01"},
		Events:   []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "today")},
		Existing: map[string][]byte{"areas/health/ACTIVITY.md": []byte("# Activity\n")},
	}
	if _, err := (render.ActivityRenderer{Days: []string{"2026-03-05"}}).Render(in); err != nil {
		t.Errorf("Render on a title-only file: %v", err)
	}
}
