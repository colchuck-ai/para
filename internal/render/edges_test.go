package render_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

func TestRuleJoinsThreeOrMoreScopeEntries(t *testing.T) {
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State: truth.State{
			Name:        "Wide",
			Description: "when doing anything at all",
			Scope:       []string{"projects", "areas.health", "resources.notes"},
			Created:     "2026-01-01",
		},
	}

	got, err := render.Rule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	want := "When working under `projects/`, `areas/health/`, or `resources/notes/`, use the **Wide** skill"
	if !strings.Contains(string(got), want) {
		t.Errorf("Rule =\n%s\nwant it to contain\n%s", got, want)
	}
}

func TestRuleKeepsScopeEntryOrderAsStored(t *testing.T) {
	// scope is truth, not a set: it replaces wholly on set (§15) and its order
	// is the author's. Sorting it here would make the rule file disagree with
	// state.toml for no gain.
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State: truth.State{
			Name:  "Wide",
			Scope: []string{"resources.notes", "areas.health"},
		},
	}
	got, err := render.Rule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "`resources/notes/` or `areas/health/`") {
		t.Errorf("Rule =\n%s\nwant scope in stored order", got)
	}
}

func TestRuleRendersAnUnresolvableScopeEntryAsWritten(t *testing.T) {
	// A scope entry naming nothing is doctor's `scope-unresolved` finding to
	// report (§5.4). Swallowing an unparseable one here would hide it from the
	// file where a reader could notice it.
	in := render.In{
		Locator: loc(t, "skills.wide"),
		Kind:    kindmeta.KindSkill,
		State:   truth.State{Name: "Wide", Scope: []string{"Not A Locator"}},
	}
	got, err := render.Rule.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "`Not A Locator`") {
		t.Errorf("Rule =\n%s\nwant the entry rendered as written", got)
	}
}

func TestSkillRenderersRefuseANonSkill(t *testing.T) {
	in := render.In{Locator: loc(t, "areas.health"), Kind: kindmeta.KindArea, State: truth.State{Name: "Health"}}
	if _, err := render.Skill.Render(in); err == nil {
		t.Error("Skill.Render on an area returned no error")
	}
	if _, err := render.Rule.Render(in); err == nil {
		t.Error("Rule.Render on an area returned no error")
	}
	if _, err := render.Rule.Path(in); err == nil {
		t.Error("Rule.Path on an area returned no error")
	}
}

func TestReadmeRefusesASubjectWithNoKind(t *testing.T) {
	in := render.In{Locator: loc(t, "areas.health"), Kind: kindmeta.KindUnknown}
	if _, err := render.Readme.Render(in); err == nil {
		t.Error("Readme.Render with no kind returned no error")
	}
}

func TestMeasurementRenderingDegradesOnContradictoryTruth(t *testing.T) {
	// A measurement whose shape contradicts its key-result's type is doctor's
	// `invalid` finding (§10). Rendering must still produce a file — a tree that
	// cannot be rendered cannot be diagnosed.
	cases := []struct {
		name       string
		state      truth.State
		value      string
		wantLine   string
		wantColumn string
	}{
		{
			name:       "a ratio reading against a number key-result",
			state:      truth.State{Name: "N", Type: "number", Target: "480", Created: "2026-03-05"},
			value:      "880/11000",
			wantLine:   "- Measured 880/11000.\n",
			wantColumn: "2026-03-05T09:00:00-08:00,880/11000,0.0000,0.0000,\n",
		},
		{
			name:       "no target at all",
			state:      truth.State{Name: "N", Type: "number", Created: "2026-03-05"},
			value:      "42",
			wantLine:   "- Measured 42.\n",
			wantColumn: "2026-03-05T09:00:00-08:00,42,42.0000,0.0000,\n",
		},
		{
			name:       "a type para does not know",
			state:      truth.State{Name: "N", Type: "duration", Target: "5m", Created: "2026-03-05"},
			value:      "3m",
			wantLine:   "- Measured 3m.\n",
			wantColumn: "2026-03-05T09:00:00-08:00,3m,0.0000,0.0000,\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := render.In{
				Locator: loc(t, "projects.a.objectives.b.key-results.c"),
				Kind:    kindmeta.KindKeyResult,
				State:   tc.state,
				Events:  []journal.Event{journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), tc.value, "")},
			}

			activity, err := render.Activity.Render(in)
			if err != nil {
				t.Fatalf("Activity.Render: %v", err)
			}
			if !strings.Contains(string(activity), tc.wantLine) {
				t.Errorf("Activity =\n%s\nwant it to contain %q", activity, tc.wantLine)
			}

			csv, err := render.Measurements.Render(in)
			if err != nil {
				t.Fatalf("Measurements.Render: %v", err)
			}
			if !strings.Contains(string(csv), tc.wantColumn) {
				t.Errorf("MEASUREMENTS.csv =\n%s\nwant it to contain %q", csv, tc.wantColumn)
			}
		})
	}
}

func TestMeasurementLinesPerType(t *testing.T) {
	// §4.1: a ratio's decimal is worth printing because the reading is not one.
	// A number already is its own decimal, and a boolean has nothing to say.
	cases := []struct {
		name  string
		state truth.State
		value string
		want  string
	}{
		{
			"number",
			truth.State{Name: "N", Type: "number", Start: "0", Target: "480", Created: "2026-03-05"},
			"240",
			"- Measured 240 — 50% of target.\n",
		},
		{
			"boolean",
			truth.State{Name: "N", Type: "boolean", Target: "true", Created: "2026-03-05"},
			"true",
			"- Measured true — 100% of target.\n",
		},
		{
			"a ratio with no explicit start baselines on its first reading",
			truth.State{Name: "N", Type: "ratio", Target: "2000/12000", Created: "2026-03-05"},
			"880/11000",
			"- Measured 880/11000 (8.0%) — 0% of target.\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := render.In{
				Locator: loc(t, "projects.a.objectives.b.key-results.c"),
				Kind:    kindmeta.KindKeyResult,
				State:   tc.state,
				Events:  []journal.Event{journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), tc.value, "")},
			}
			got, err := render.Activity.Render(in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(string(got), tc.want) {
				t.Errorf("Activity =\n%s\nwant it to contain %q", got, tc.want)
			}
		})
	}
}

func TestMeasurementProgressIsUnclamped(t *testing.T) {
	// §4.2: overshoot reads above 1 and a regression below baseline reads
	// negative, because both are true and both are worth seeing.
	in := render.In{
		Locator: loc(t, "projects.a.objectives.b.key-results.c"),
		Kind:    kindmeta.KindKeyResult,
		State:   truth.State{Name: "N", Type: "number", Start: "100", Target: "200", Created: "2026-03-05"},
		Events: []journal.Event{
			journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), "50", ""),
			journal.NewMeasurement(ts(t, "2026-03-06T09:00:00"), "250", ""),
		},
	}

	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"- Measured 50 — -50% of target.\n", "- Measured 250 — 150% of target.\n"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("Activity =\n%s\nwant it to contain %q", got, want)
		}
	}
}

func TestActivityRefusesAnUnknownEventKind(t *testing.T) {
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Health", Created: "2026-03-05"},
		Events:  []journal.Event{{At: ts(t, "2026-03-05T09:00:00"), Kind: journal.Kind("invented")}},
	}
	if _, err := render.Activity.Render(in); err == nil {
		t.Error("Activity.Render accepted an unknown event kind")
	}
}

func TestActivityOnASubjectWithNoCreatedHasNoCreatedLine(t *testing.T) {
	// An archive stub has no .para/ at all (§1.6) and so no created field. It
	// owns no ACTIVITY.md either, but rendering must not invent a section for a
	// day it cannot name.
	in := render.In{
		Locator: loc(t, "areas.health"),
		Kind:    kindmeta.KindArea,
		Events:  []journal.Event{journal.NewNote(ts(t, "2026-03-05T09:00:00"), "orphaned")},
	}
	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(string(got), "Created.") {
		t.Errorf("Activity =\n%s\nwant no created line", got)
	}
	if render.CreatedDay(in) != "" {
		t.Errorf("CreatedDay = %q, want empty", render.CreatedDay(in))
	}
}

func TestActivityOnAnEmptyHistoryIsJustTheTitle(t *testing.T) {
	in := render.In{Locator: loc(t, "areas.health"), Kind: kindmeta.KindArea}
	got, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if string(got) != "# Activity\n" {
		t.Errorf("Activity = %q, want just the title", got)
	}
}

func TestStateListReturnsTheListFields(t *testing.T) {
	s := truth.State{Tags: []string{"a"}, Scope: []string{"projects"}}
	if got := s.List(kindmeta.FieldTags); len(got) != 1 || got[0] != "a" {
		t.Errorf("List(tags) = %v, want [a]", got)
	}
	if got := s.List(kindmeta.FieldScope); len(got) != 1 || got[0] != "projects" {
		t.Errorf("List(scope) = %v, want [projects]", got)
	}
	if got := s.List(kindmeta.FieldName); got != nil {
		t.Errorf("List(name) = %v, want nil", got)
	}
}

func TestActivityRefusesIncrementalModeForAKeyResult(t *testing.T) {
	// Every measurement line ends in "N% of target", a function of start, target,
	// and the oldest reading — all of which a later mutation can change, moving
	// lines on days incremental mode would never revisit. Refusing costs nothing
	// because the same mutation's MEASUREMENTS.csv rewrite already needs the whole
	// history (§2.3, §4.4).
	in := render.In{
		Locator: loc(t, "projects.a.objectives.b.key-results.c"),
		Kind:    kindmeta.KindKeyResult,
		State:   truth.State{Name: "N", Type: "number", Target: "480", Created: "2026-03-05"},
		Events:  []journal.Event{journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), "42", "")},
	}
	if _, err := (render.ActivityRenderer{Days: []string{"2026-03-05"}}).Render(in); err == nil {
		t.Error("incremental Render on a key-result returned no error")
	}
	if _, err := render.Activity.Render(in); err != nil {
		t.Errorf("full Render on a key-result: %v", err)
	}
}

func TestChangingTargetIsVisibleInEveryPriorDay(t *testing.T) {
	// The reason the refusal above exists: `set --target` rewrites the progress
	// on readings from days already written. Full mode gets it right; there is no
	// incremental mode to get it wrong.
	events := []journal.Event{
		journal.NewMeasurement(ts(t, "2026-03-01T09:00:00"), "0", ""),
		journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), "60", ""),
	}
	before := render.In{
		Locator: loc(t, "projects.a.objectives.b.key-results.c"),
		Kind:    kindmeta.KindKeyResult,
		State:   truth.State{Name: "N", Type: "number", Start: "0", Target: "120", Created: "2026-03-01"},
		Events:  events,
	}
	after := before
	after.State.Target = "60"

	got, err := render.Activity.Render(before)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "- Measured 60 — 50% of target.\n") {
		t.Fatalf("Activity =\n%s\nwant 50%% against target 120", got)
	}

	got, err = render.Activity.Render(after)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(got), "- Measured 60 — 100% of target.\n") {
		t.Errorf("Activity =\n%s\nwant 100%% against target 60", got)
	}
}

func TestProgressAtTheBaselineOfADecreasingKeyResultIsNotNegativeZero(t *testing.T) {
	// A key-result counting down is legal — §4.2 says direction falls out of the
	// arithmetic, and only start == target is rejected. Its denominator is
	// negative, so the first reading at the baseline gives 0 / -150 = -0.0, which
	// formats as "-0%" and "-0.0000" unless collapsed.
	in := render.In{
		Locator: loc(t, "projects.a.objectives.b.key-results.p99"),
		Kind:    kindmeta.KindKeyResult,
		State:   truth.State{Name: "p99", Type: "number", Start: "400", Target: "250", Created: "2026-03-05"},
		Events:  []journal.Event{journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), "400", "")},
	}

	activity, err := render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Activity.Render: %v", err)
	}
	if strings.Contains(string(activity), "-0%") {
		t.Errorf("Activity =\n%s\nwant no negative zero", activity)
	}
	if !strings.Contains(string(activity), "- Measured 400 — 0% of target.\n") {
		t.Errorf("Activity =\n%s\nwant \"0%% of target\"", activity)
	}

	csv, err := render.Measurements.Render(in)
	if err != nil {
		t.Fatalf("Measurements.Render: %v", err)
	}
	if strings.Contains(string(csv), "-0.0000") {
		t.Errorf("MEASUREMENTS.csv =\n%s\nwant no negative zero", csv)
	}

	// And a real regression still reads negative, which is the point of §4.2's
	// refusal to clamp.
	in.Events = append(in.Events, journal.NewMeasurement(ts(t, "2026-03-06T09:00:00"), "500", ""))
	activity, err = render.Activity.Render(in)
	if err != nil {
		t.Fatalf("Activity.Render: %v", err)
	}
	if !strings.Contains(string(activity), "- Measured 500 — -67% of target.\n") {
		t.Errorf("Activity =\n%s\nwant a negative reading for a regression", activity)
	}
}

func TestActivityDropsADayHeadingWithNoLinesUnderIt(t *testing.T) {
	// Full mode never emits an empty section (§3.5: days with no events are
	// absent), so carrying one through the incremental path would be a permanent
	// difference between the two modes.
	existing := []byte("# Activity\n\n## 2026-03-04\n\n## 2026-03-01\n- Created.\n")
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
	want := "# Activity\n\n## 2026-03-05\n- Note: today.\n\n## 2026-03-01\n- Created.\n"
	if string(got) != want {
		t.Errorf("incremental Activity =\n%q\nwant\n%q", got, want)
	}
}

func TestActivityRepairsASectionMissingItsFinalNewline(t *testing.T) {
	// A truncated file would otherwise re-render as "- x## 2026-03-01", which is
	// not a file either mode would produce.
	existing := []byte("# Activity\n\n## 2026-03-04\n- Note: yesterday.\n\n## 2026-03-01\n- Created.")
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
	want := "# Activity\n\n## 2026-03-05\n- Note: today.\n\n## 2026-03-04\n- Note: yesterday.\n\n## 2026-03-01\n- Created.\n"
	if string(got) != want {
		t.Errorf("incremental Activity =\n%q\nwant\n%q", got, want)
	}
}
