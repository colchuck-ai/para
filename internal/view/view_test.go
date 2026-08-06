package view_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/view"
)

// now is the instant every test in this package reads, 2026-03-05 17:00Z. The
// tree is built through the real write path at this same instant, so `created`
// and today coincide unless a test backdates something with --at.
func now() time.Time { return time.Date(2026, time.March, 5, 17, 0, 0, 0, time.UTC) }

func at(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parsing %q: %v", s, err)
	}
	return parsed
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// plantTree writes the smallest tree a read can run against: the root marker
// and the four buckets, as mutate's own tests plant it.
func plantTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".para/tree.toml":                    "schema = 1\n",
		"projects/.para/state.toml":          "name = \"Projects\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"areas/.para/state.toml":             "name = \"Areas\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"resources/.para/state.toml":         "name = \"Resources\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/.para/state.toml":           "name = \"Archive\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/projects/.para/state.toml":  "name = \"Archived projects\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/areas/.para/state.toml":     "name = \"Archived areas\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/resources/.para/state.toml": "name = \"Archived resources\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		".agents/skills/.keep":               "",
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

func writer(t *testing.T, root string) *mutate.Env {
	t.Helper()
	return mutate.NewEnv(root, clock.Fixed{At: now()})
}

func reader(t *testing.T, root string) *view.Env {
	t.Helper()
	return view.NewEnv(root, clock.Fixed{At: now()})
}

// fields builds an add/set argument from key/value pairs, splitting the two
// list fields on commas the way the flag parser does.
func fields(kv ...string) mutate.Fields {
	var f mutate.Fields
	for i := 0; i+1 < len(kv); i += 2 {
		field := kindmeta.Field(kv[i])
		if field == kindmeta.FieldTags || field == kindmeta.FieldScope {
			f.SetList(field, strings.Split(kv[i+1], ","))
			continue
		}
		f.Set(field, kv[i+1])
	}
	return f
}

func add(t *testing.T, e *mutate.Env, l string, kv ...string) {
	t.Helper()
	if _, err := e.Add(loc(t, l), fields(kv...)); err != nil {
		t.Fatalf("Add(%s): %v", l, err)
	}
}

func set(t *testing.T, e *mutate.Env, l string, kv ...string) {
	t.Helper()
	if _, err := e.Set(loc(t, l), fields(kv...), ""); err != nil {
		t.Fatalf("Set(%s): %v", l, err)
	}
}

// archive runs the relocating verb through its plan/apply split, which is the
// only way to reach it.
func archive(t *testing.T, e *mutate.Env, l string) {
	t.Helper()
	plan, err := e.PlanArchive(loc(t, l))
	if err != nil {
		t.Fatalf("PlanArchive(%s): %v", l, err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
}

// configure writes the root's config.toml directly. The `config set` command is
// the CLI's, and these tests are about what resolution answers, not about how
// the file came to say it.
func configure(t *testing.T, root string, body string) {
	t.Helper()
	path := filepath.Join(root, ".para", "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, e *view.Env, l string) view.Entity {
	t.Helper()
	ent, err := e.Load(loc(t, l))
	if err != nil {
		t.Fatalf("Load(%s): %v", l, err)
	}
	return ent
}

// project plants §16.1's worked project, its objective, and its key-result.
func project(t *testing.T, root string) *mutate.Env {
	t.Helper()
	w := writer(t, root)
	add(t, w, "projects.acme-migration",
		"name", "Acme migration", "description", "Rebuild the consumer.",
		"status", "in-progress", "priority", "high", "due", "2026-09-30", "tags", "consumer,kafka")
	add(t, w, "projects.acme-migration.objectives.q1-growth", "name", "Grow signups", "description", "Move the top of the funnel.")
	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.signups",
		"name", "Weekly signups", "type", "ratio", "start", "480/9000", "target", "2000/12000",
		"due", "2026-09-30")
	return w
}

func TestLoadReadsStoredFields(t *testing.T) {
	root := plantTree(t)
	project(t, root)

	got := load(t, reader(t, root), "projects.acme-migration")
	if got.Kind != kindmeta.KindProject {
		t.Errorf("kind: got %s, want project", got.Kind)
	}
	if got.Name() != "Acme migration" {
		t.Errorf("name: got %q", got.Name())
	}
	if got.EffectiveStatus != "in-progress" {
		t.Errorf("status: got %q", got.EffectiveStatus)
	}
	if got.State.Priority != "high" || got.State.Due != "2026-09-30" {
		t.Errorf("stored fields: %+v", got.State)
	}
	if got.Container || got.Archived || got.Terminal || got.Dormant() {
		t.Errorf("flags: %+v", got)
	}
}

// TestAttentionIsTheNewestNoteOrMeasurement is §3.6's clock, read against a real
// journal rather than a slice of events.
func TestAttentionIsTheNewestNoteOrMeasurement(t *testing.T) {
	root := plantTree(t)
	w := project(t, root)
	r := reader(t, root)

	// Created and nothing since: attention is `created` (§3.6).
	if got := load(t, r, "projects.acme-migration").Attention; !got.Equal(now()) {
		t.Errorf("attention with no events: got %s, want created %s", got, now())
	}

	if _, err := w.Note(loc(t, "projects.acme-migration"), "still going", "2026-03-04"); err != nil {
		t.Fatal(err)
	}
	if got := load(t, r, "projects.acme-migration").Attention; !got.Equal(now()) {
		t.Errorf("a backdated note must not move attention backwards: got %s", got)
	}

	// A change event never counts, however recent.
	set(t, w, "projects.acme-migration", "priority", "low")
	if got := load(t, r, "projects.acme-migration").Attention; !got.Equal(now()) {
		t.Errorf("attention after a change: got %s, want %s", got, now())
	}
}

// TestKeyResultDerivation is §4 end to end: baseline, current, progress, pace,
// and the derived status, from a real journal.
func TestKeyResultDerivation(t *testing.T) {
	root := plantTree(t)
	w := project(t, root)
	kr := "projects.acme-migration.objectives.q1-growth.key-results.signups"
	if _, err := w.Measure(loc(t, kr), "880/11000", "2026-01-03", ""); err != nil {
		t.Fatal(err)
	}

	got := load(t, reader(t, root), kr)
	if got.KeyResult == nil {
		t.Fatal("a key-result must carry §4's arithmetic")
	}
	if !got.KeyResult.HasCurrent || got.KeyResult.Current.Raw != "880/11000" {
		t.Errorf("current: %+v", got.KeyResult.Current)
	}
	// The explicit `start` wins over the oldest reading (§4.1), and it is
	// reported as written rather than as a decimal.
	if !got.KeyResult.HasStart || got.KeyResult.Start.Raw != "480/9000" {
		t.Errorf("start: %+v", got.KeyResult.Start)
	}
	// §4.2's formula, (current − start) / (target − start), which the design's
	// own worked lines get wrong by dropping start.
	want := (880.0/11000 - 480.0/9000) / (2000.0/12000 - 480.0/9000)
	if diff := got.KeyResult.Outlook.Progress - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("progress: got %v, want %v", got.KeyResult.Outlook.Progress, want)
	}
	// No `key-result.at-risk-pace` anywhere in this tree, so the check never
	// fires (§7) and the status cannot be at-risk.
	if got.EffectiveStatus != string(krvalue.StatusOnTrack) {
		t.Errorf("status: got %q, want on-track", got.EffectiveStatus)
	}
}

func TestKeyResultBaselineFallsBackToTheOldestReading(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.p", "name", "P", "description", "A project.")
	add(t, w, "projects.p.objectives.o", "name", "O", "description", "An objective.")
	kr := "projects.p.objectives.o.key-results.k"
	add(t, w, kr, "name", "K", "type", "number", "target", "100")

	for _, m := range []struct{ value, at string }{
		{"40", "2026-02-01"}, {"10", "2026-01-01"}, {"60", "2026-03-01"},
	} {
		if _, err := w.Measure(loc(t, kr), m.value, m.at, ""); err != nil {
			t.Fatal(err)
		}
	}

	got := load(t, reader(t, root), kr).KeyResult
	// Oldest by `at`, not by write order: 10 was logged second.
	if !got.HasStart || got.Start.Raw != "10" {
		t.Errorf("baseline: got %+v, want the oldest reading 10", got.Start)
	}
	if got.Current.Raw != "60" {
		t.Errorf("current: got %q, want 60", got.Current.Raw)
	}
	if want := (60.0 - 10) / (100 - 10); got.Outlook.Progress != want {
		t.Errorf("progress: got %v, want %v", got.Outlook.Progress, want)
	}
}

func TestKeyResultWithNoMeasurementYet(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.p", "name", "P", "description", "A project.")
	add(t, w, "projects.p.objectives.o", "name", "O", "description", "An objective.")
	add(t, w, "projects.p.objectives.o.key-results.k",
		"name", "K", "type", "number", "start", "0", "target", "100")

	got := load(t, reader(t, root), "projects.p.objectives.o.key-results.k").KeyResult
	if got.HasCurrent {
		t.Error("a key-result with no reading has no current")
	}
	// Progress is exactly 0 rather than undefined (§4.2), so an untouched
	// key-result can go at-risk instead of sitting quiet.
	if !got.Outlook.HasProgress || got.Outlook.Progress != 0 {
		t.Errorf("progress: %+v", got.Outlook)
	}
}

// TestTerminalAncestorCascade is §1.7: a thing's effective status is its own
// unless any ancestor's is terminal.
func TestTerminalAncestorCascade(t *testing.T) {
	root := plantTree(t)
	w := project(t, root)
	set(t, w, "projects.acme-migration", "status", "done")

	r := reader(t, root)
	objective := load(t, r, "projects.acme-migration.objectives.q1-growth")
	if objective.EffectiveStatus != "done" {
		t.Errorf("objective effective status: got %q, want done", objective.EffectiveStatus)
	}
	if objective.DormantUnder.String() != "projects.acme-migration" {
		t.Errorf("dormant under: got %q", objective.DormantUnder)
	}
	if !objective.Terminal || !objective.Dormant() {
		t.Errorf("a quieted objective is terminal and dormant: %+v", objective)
	}
	// Its own stored status is untouched — the cascade is derived, never
	// written to a descendant (§2.5).
	if objective.State.Status == "done" {
		t.Error("the cascade must not be stored on the descendant")
	}

	// Two levels down, through a container, which can neither quiet nor shield.
	kr := load(t, r, "projects.acme-migration.objectives.q1-growth.key-results.signups")
	if kr.DormantUnder.String() != "projects.acme-migration" || !kr.Terminal {
		t.Errorf("key-result: dormant under %q terminal %v", kr.DormantUnder, kr.Terminal)
	}
}

func TestArchivedIsLocationNotAField(t *testing.T) {
	root := plantTree(t)
	w := project(t, root)
	archive(t, w, "projects.acme-migration")

	got := load(t, reader(t, root), "archive.projects.acme-migration")
	if !got.Archived || !got.Dormant() {
		t.Errorf("archived: %+v", got)
	}
	// Archived is not terminal: §1.6 puts it somewhere else, it does not end it.
	if got.Terminal {
		t.Error("archiving is a place, not a status (§1.6)")
	}
}

func TestLoadRefusesAStub(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "areas.health", "name", "Health", "description", "Staying in one piece.")
	add(t, w, "areas.health.training", "name", "Training", "description", "The weekly plan.")
	archive(t, w, "areas.health.training")

	// archive.areas.health is now a stub: a segment with no entity behind it.
	_, err := reader(t, root).Load(loc(t, "archive.areas.health"))
	if err == nil {
		t.Fatal("loading a stub must fail — there is nothing there to describe")
	}
	if got := err.Error(); got == "" {
		t.Error("the refusal must name the stub")
	}
}

// TestOverdue is §17's filter and §4.3's `missed` sharing one deadline: a bare
// `due` date is the whole day, so nothing is late while its day is running.
func TestOverdue(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.today", "name", "Today", "description", "Due today.", "due", "2026-03-05")
	// Created before its own deadline: the write path refuses a `created` that
	// postdates `due` (§15), which is the same whole-day reading from the other
	// side.
	add(t, w, "projects.yesterday", "name", "Yesterday", "description", "Due yesterday.",
		"created", "2026-03-01", "due", "2026-03-04")
	add(t, w, "projects.later", "name", "Later", "description", "Due later.", "due", "2026-09-30")

	r := reader(t, root)
	if got := load(t, r, "projects.today"); got.Overdue() {
		t.Error("due today is not overdue until today is over")
	}
	if got := load(t, r, "projects.yesterday"); !got.Overdue() {
		t.Error("due yesterday is overdue")
	}
	if got := load(t, r, "projects.later"); got.Overdue() {
		t.Error("due in September is not overdue in March")
	}

	// Terminal is what "open" excludes (§17).
	set(t, w, "projects.yesterday", "status", "done")
	if got := load(t, r, "projects.yesterday"); got.Overdue() {
		t.Error("something done is not overdue, it is done")
	}
}

func TestDaysSinceAndUntilCountCalendarDays(t *testing.T) {
	r := reader(t, plantTree(t))
	cases := []struct {
		at         string
		since      int
		until      int
		whatItPins string
	}{
		{at: "2026-03-05T00:00:01Z", since: 0, until: 0, whatItPins: "earlier the same day is 0 days ago"},
		{at: "2026-03-04T23:59:59Z", since: 1, until: -1, whatItPins: "a minute earlier, but the day before"},
		{at: "2026-02-02T12:00:00Z", since: 31, until: -31, whatItPins: "§16.1's 31 days ago"},
		{at: "2026-09-02T00:00:00Z", since: -181, until: 181, whatItPins: "§16.1's in 181 days"},
	}
	for _, tc := range cases {
		when := at(t, tc.at)
		if got := r.DaysSince(when); got != tc.since {
			t.Errorf("DaysSince(%s): got %d, want %d (%s)", tc.at, got, tc.since, tc.whatItPins)
		}
		if got := r.DaysUntil(when); got != tc.until {
			t.Errorf("DaysUntil(%s): got %d, want %d (%s)", tc.at, got, tc.until, tc.whatItPins)
		}
	}
}

// TestStaleNamesItsThresholdAndSource is §16.1's requirement that a resolved
// knob says where it came from, and §20's that a skill reads a different key.
func TestStaleNamesItsThresholdAndSource(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.p", "name", "P", "description", "A project.")
	add(t, w, "skills.s", "name", "S", "description", "when asked")

	r := reader(t, root)

	// Nothing set anywhere: §7 says the check never fires.
	th, stale, err := r.Stale(load(t, r, "projects.p"))
	if err != nil {
		t.Fatal(err)
	}
	if th.Found || stale {
		t.Errorf("an unset threshold never fires: %+v stale=%v", th, stale)
	}

	configure(t, root, "[project]\nstale-after = 14\n\n[review]\ncadence = 30\n")

	r = reader(t, root)
	th, stale, err = r.Stale(load(t, r, "projects.p"))
	if err != nil {
		t.Fatal(err)
	}
	if th.Key != "project.stale-after" || th.Value != 14 {
		t.Errorf("threshold: %+v", th)
	}
	if th.From != ".para/config.toml" {
		t.Errorf("provenance: got %q, want .para/config.toml", th.From)
	}
	if stale {
		t.Error("created today is not stale after 14 days")
	}

	// §20: a skill's threshold is review.cadence, not stale-after.
	th, _, err = r.Stale(load(t, r, "skills.s"))
	if err != nil {
		t.Fatal(err)
	}
	if th.Key != "review.cadence" || th.Value != 30 {
		t.Errorf("a skill reads review.cadence: %+v", th)
	}
}

func TestStaleFiresPastTheThreshold(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.p", "name", "P", "description", "A project.", "created", "2026-02-02")
	configure(t, root, "[project]\nstale-after = 14\n")

	r := reader(t, root)
	ent := load(t, r, "projects.p")
	if got := r.DaysSince(ent.Attention); got != 31 {
		t.Fatalf("days since attention: got %d, want 31", got)
	}
	_, stale, err := r.Stale(ent)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Error("31 days past a 14-day threshold is stale")
	}
}

// TestSkillsReaching is §5.2's containment rule and §16.1's skills line.
func TestSkillsReaching(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.acme-migration", "name", "Acme", "description", "A project.")
	add(t, w, "areas.health", "name", "Health", "description", "An area.")
	add(t, w, "areas.healthcare", "name", "Healthcare", "description", "Another area.")
	add(t, w, "skills.scoped", "name", "Scoped", "description", "when asked", "scope", "projects")
	add(t, w, "skills.narrow", "name", "Narrow", "description", "when asked", "scope", "areas.health")
	add(t, w, "skills.everywhere", "name", "Everywhere", "description", "when asked")

	r := reader(t, root)
	cases := []struct {
		loc  string
		want []string
	}{
		{"projects.acme-migration", []string{"skills.everywhere", "skills.scoped"}},
		{"areas.health", []string{"skills.everywhere", "skills.narrow"}},
		// Containment is by segment, never by string prefix: areas.health does
		// not cover areas.healthcare.
		{"areas.healthcare", []string{"skills.everywhere"}},
	}
	for _, tc := range cases {
		got, err := r.SkillsReaching(loc(t, tc.loc))
		if err != nil {
			t.Fatalf("SkillsReaching(%s): %v", tc.loc, err)
		}
		var names []string
		for _, s := range got {
			names = append(names, s.Locator.String())
		}
		if len(names) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.loc, names, tc.want)
			continue
		}
		for i := range names {
			if names[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.loc, names, tc.want)
				break
			}
		}
	}

	// The entry that reached it is reported, so "why is this rule in my
	// context" is answerable (§13.1, §16.1).
	got, err := r.SkillsReaching(loc(t, "projects.acme-migration"))
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range got {
		switch s.Locator.String() {
		case "skills.scoped":
			if s.Via != "projects" || s.WholeTree {
				t.Errorf("scoped: via %q wholeTree %v", s.Via, s.WholeTree)
			}
		case "skills.everywhere":
			if !s.WholeTree || s.Via != "" {
				t.Errorf("unscoped: via %q wholeTree %v", s.Via, s.WholeTree)
			}
		}
	}
}

// TestDayCountsFollowTheCalendarTheyArePrintedOn is `--local`'s other half. An
// attention of 2026-03-05T02:00Z is 2026-03-04 in Los Angeles, and a line
// reading "attention 2026-03-04  today" would be two answers to one question.
func TestDayCountsFollowTheCalendarTheyArePrintedOn(t *testing.T) {
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skipf("zoneinfo unavailable: %v", err)
	}
	r := reader(t, plantTree(t)) // now is 2026-03-05T17:00:00Z, which is 09:00 Pacific
	when := at(t, "2026-03-05T02:00:00Z")

	if got := r.DaysSinceIn(when, time.UTC); got != 0 {
		t.Errorf("UTC: got %d days ago, want 0 — both instants fall on 03-05", got)
	}
	if got := r.DaysSinceIn(when, pacific); got != 1 {
		t.Errorf("Pacific: got %d days ago, want 1 — 02:00Z is the 4th there", got)
	}

	// The default is UTC, because a threshold is committed and every reader of
	// one tree must get one answer (§3.5's argument).
	if got := r.DaysSince(when); got != r.DaysSinceIn(when, time.UTC) {
		t.Error("DaysSince must be the UTC count")
	}
	if got := r.DaysUntilIn(when, pacific); got != -1 {
		t.Errorf("DaysUntilIn: got %d, want -1", got)
	}
}
