package review_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/review"
	"github.com/colchuck-ai/para/internal/view"
)

// now is the instant every test below reviews at. The fixture's dates are
// chosen relative to it, so a day count in an assertion is arithmetic the
// reader can check rather than a number copied from a run.
func now() time.Time { return time.Date(2026, time.March, 5, 17, 0, 0, 0, time.UTC) }

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	if s == "" {
		return nil
	}
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

func plantTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range map[string]string{
		".para/tree.toml":                    "schema = 1\n",
		"projects/.para/state.toml":          "name = \"Projects\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"areas/.para/state.toml":             "name = \"Areas\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"resources/.para/state.toml":         "name = \"Resources\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/.para/state.toml":           "name = \"Archive\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/projects/.para/state.toml":  "name = \"Archived projects\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/areas/.para/state.toml":     "name = \"Archived areas\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		"archive/resources/.para/state.toml": "name = \"Archived resources\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		".agents/skills/.keep":               "",
	} {
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
	if _, err := e.Set(loc(t, l), fields(kv...), "the reason"); err != nil {
		t.Fatalf("Set(%s): %v", l, err)
	}
}

// configure merges §7 knobs into the root's config.toml through the real codec,
// so a test never asserts against a file para could not have written.
func configure(t *testing.T, root string, kv ...string) {
	t.Helper()
	path := filepath.Join(root, ".para", "config.toml")
	f, err := config.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		spec, ok := config.Lookup(kv[i])
		if !ok {
			t.Fatalf("%q is not a config key", kv[i])
		}
		v, err := spec.Parse(kv[i+1])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Set(kv[i], v); err != nil {
			t.Fatal(err)
		}
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixture is the tree every test below reviews.
//
// Every `created` is explicit and every entity's attention is therefore known:
// nothing here notes or measures except where the test is about the clock, so
// attention is `created` (§3.6) and every day count in an assertion is
// arithmetic on the dates rather than a number copied from a run.
//
// `key-result.at-risk-pace` is deliberately left unset. An unmeasured
// key-result's progress is defined to be 0 (§4.2), so configuring the knob here
// would put every key-result in `--behind` and make the other groups' counts
// depend on a threshold they are not about.
func fixture(t *testing.T) string {
	t.Helper()
	root := plantTree(t)
	w := writer(t, root)

	// 63 days before now, and stale under a 30-day threshold.
	add(t, w, "areas.fitness", "name", "Fitness", "description", "Staying in one piece.",
		"created", "2026-01-01")
	add(t, w, "areas.fitness.training", "name", "Training", "description", "The weekly plan.",
		"created", "2026-01-01")
	// 8 days before now, and not stale under the same threshold.
	add(t, w, "areas.admin", "name", "Admin", "description", "Paperwork.", "created", "2026-02-25")

	add(t, w, "projects.acme-migration", "name", "Acme migration", "description", "Rebuild the consumer.",
		"status", "in-progress", "created", "2026-01-05", "due", "2026-09-30")
	add(t, w, "projects.acme-migration.objectives.q1-growth", "name", "Grow signups",
		"description", "Move the top of the funnel.",
		"status", "in-progress", "created", "2026-01-05")
	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.signups",
		"name", "Weekly signups", "type", "ratio", "start", "480/9000",
		"target", "2000/12000", "created", "2026-01-05", "due", "2026-09-30")

	configure(t, root,
		"area.stale-after", "30",
		"project.stale-after", "30",
		"objective.stale-after", "30",
		"key-result.stale-after", "30",
		"review.cadence", "30")
	return root
}

func run(t *testing.T, root string, opts review.Options) review.Result {
	t.Helper()
	res, err := review.Run(reader(t, root), opts)
	if err != nil {
		t.Fatalf("review.Run: %v", err)
	}
	return res
}

// sectionItems returns the named group's items.
func sectionItems(t *testing.T, res review.Result, g review.Group) []review.Item {
	t.Helper()
	for _, s := range res.Sections {
		if s.Group == g {
			return s.Items
		}
	}
	return nil
}

// section returns the named group's locators, in the order Run put them.
func section(t *testing.T, res review.Result, g review.Group) []string {
	t.Helper()
	for _, s := range res.Sections {
		if s.Group != g {
			continue
		}
		out := make([]string, 0, len(s.Items))
		for _, item := range s.Items {
			out = append(out, item.Entity.Locator.String())
		}
		return out
	}
	return nil
}

func assertLocators(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestGroupsInSpecOrder pins §20's table order, which is the order the output
// prints and therefore not an implementation detail.
func TestGroupsInSpecOrder(t *testing.T) {
	want := []review.Group{
		review.GroupStale, review.GroupBlocked, review.GroupOverdue,
		review.GroupBehind, review.GroupSkills, review.GroupSuppressed,
	}
	got := review.Groups()
	if len(got) != len(want) {
		t.Fatalf("Groups() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("Groups() = %v, want %v", got, want)
		}
	}
}

// TestStaleFiresOffTheKindsOwnThreshold is §20's first row: no note or
// measurement within `<kind>.stale-after` (§3.6).
func TestStaleFiresOffTheKindsOwnThreshold(t *testing.T) {
	root := fixture(t)
	got := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})

	// areas.admin is 8 days old and survives; everything created in early
	// January does not. Order is distance past the threshold, furthest first,
	// so the two 63-day areas come before the 59-day project chain.
	assertLocators(t, section(t, got, review.GroupStale),
		"areas.fitness",
		"areas.fitness.training",
		"projects.acme-migration",
		"projects.acme-migration.objectives.q1-growth",
		"projects.acme-migration.objectives.q1-growth.key-results.signups")

	// The threshold travels with the item, because a group that mixes kinds
	// mixes knobs: an area reads area.stale-after and a project project.stale-after.
	for _, s := range got.Sections {
		for _, item := range s.Items {
			if !item.Threshold.Found {
				t.Errorf("%s: no threshold reported", item.Entity.Locator)
			}
			if want := item.Entity.Kind.String() + ".stale-after"; item.Threshold.Key != want {
				t.Errorf("%s: threshold key = %q, want %q", item.Entity.Locator, item.Threshold.Key, want)
			}
		}
	}
}

// TestStaleFiresOffLinkStaleAfter is para-nd3: link joined staleAfterKinds
// (internal/config/keys.go), so a link reads its own link.stale-after knob
// exactly like any other kind — kept in its own tree rather than fixture()'s,
// so the many tests reading that shared fixture's exact locator lists never
// have to widen for a kind they are not about.
func TestStaleFiresOffLinkStaleAfter(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.acme", "name", "Acme", "description", "Rebuild.", "created", "2026-01-05")
	add(t, w, "projects.acme.links.jira-epic", "type", "jira-epic", "ref", "PROJ-1",
		"direction", "output", "created", "2026-01-05")
	configure(t, root, "link.stale-after", "30")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	assertLocators(t, section(t, got, review.GroupStale), "projects.acme.links.jira-epic")

	items := sectionItems(t, got, review.GroupStale)
	if items[0].Threshold.Key != "link.stale-after" {
		t.Errorf("threshold key = %q, want link.stale-after", items[0].Threshold.Key)
	}
}

// TestKindFilterReachesALink is para-nd3's `review link` fix: Options.Kind
// narrows the walk to one addressable kind, the same way query.List already
// lets `list` do it — confirming review.Run itself needs no change beyond
// threading the field through, since classify/staleItem never look at kind
// beyond what config.StaleKey already answers.
func TestKindFilterReachesALink(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "projects.acme", "name", "Acme", "description", "Rebuild.", "created", "2026-01-05")
	add(t, w, "projects.acme.links.jira-epic", "type", "jira-epic", "ref", "PROJ-1",
		"direction", "output", "created", "2026-01-05")
	add(t, w, "projects.acme.objectives.q1", "name", "Q1", "description", "Grow.",
		"created", "2026-01-05")
	configure(t, root, "link.stale-after", "30", "objective.stale-after", "30")

	got := run(t, root, review.Options{Kind: kindmeta.KindLink, Only: []review.Group{review.GroupStale}})
	assertLocators(t, section(t, got, review.GroupStale), "projects.acme.links.jira-epic")
}

// TestStaleNeverContainsASkill is §20's "skills are reached by --skills and
// nothing else": a skill's threshold is review.cadence, and putting it in
// --stale would mean one group reading two different knobs.
func TestStaleNeverContainsASkill(t *testing.T) {
	root := fixture(t)
	add(t, writer(t, root), "skills.commit-style", "name", "Commit style",
		"description", "when writing a commit message", "created", "2026-01-01")

	got := run(t, root, review.Options{})
	for _, name := range section(t, got, review.GroupStale) {
		if strings.HasPrefix(name, "skills.") {
			t.Errorf("%s is in --stale; skills belong to --skills alone (§20)", name)
		}
	}
	assertLocators(t, section(t, got, review.GroupSkills), "skills.commit-style")
}

// TestSkillsFiresOffReviewCadence: the group exists because §7 has the key, so
// the key is what has to decide it — and an unset cadence means the check never
// fires (§7).
func TestSkillsFiresOffReviewCadence(t *testing.T) {
	root := fixture(t)
	add(t, writer(t, root), "skills.commit-style", "name", "Commit style",
		"description", "when writing a commit message", "created", "2026-01-01")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupSkills}})
	items := got.Sections[0].Items
	if len(items) != 1 {
		t.Fatalf("got %d skills, want 1", len(items))
	}
	if items[0].Threshold.Key != "review.cadence" {
		t.Errorf("threshold key = %q, want review.cadence", items[0].Threshold.Key)
	}
	if items[0].Days != 63 {
		t.Errorf("days = %d, want 63", items[0].Days)
	}

	configure(t, root, "review.cadence", "365")
	if quiet := run(t, root, review.Options{Only: []review.Group{review.GroupSkills}}); len(quiet.Sections) != 0 {
		t.Errorf("a cadence of 365 days must quiet a 63-day-old skill, got %v", quiet.Sections)
	}
}

// TestAnUnsetThresholdNeverFires is §7's rule: "unset everywhere means the check
// never fires", which is why the three thresholds deliberately have no default.
func TestAnUnsetThresholdNeverFires(t *testing.T) {
	root := plantTree(t)
	add(t, writer(t, root), "areas.fitness", "name", "Fitness",
		"description", "Staying in one piece.", "created", "2020-01-01")

	got := run(t, root, review.Options{})
	if len(got.Sections) != 0 {
		t.Errorf("nothing is configured, so nothing can be past a threshold; got %v", got.Sections)
	}
}

// TestBlockedHasNoTimer is §20's second row, stated as "**No timer** — blocked
// is always listed".
func TestBlockedHasNoTimer(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	// Created moments ago, so no staleness timer could possibly have run out.
	add(t, w, "projects.website", "name", "Website", "description", "A refresh.", "status", "planned")
	set(t, w, "projects.website", "status", "blocked")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupBlocked}})
	assertLocators(t, section(t, got, review.GroupBlocked), "projects.website")
	if th := got.Sections[0].Items[0].Threshold; th.Found {
		t.Errorf("blocked reported a threshold %q; it has no timer (§20)", th.Key)
	}
}

// TestBlockedReadsEffectiveStatus: a blocked objective under a dropped project
// is not blocked any more, it is dropped, and §1.7's cascade is what says so.
func TestBlockedReadsEffectiveStatus(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	set(t, w, "projects.acme-migration.objectives.q1-growth", "status", "blocked")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupBlocked}})
	assertLocators(t, section(t, got, review.GroupBlocked),
		"projects.acme-migration.objectives.q1-growth")

	set(t, w, "projects.acme-migration", "status", "dropped")
	quiet := run(t, root, review.Options{Only: []review.Group{review.GroupBlocked}})
	if len(quiet.Sections) != 0 {
		t.Errorf("a blocked objective under a dropped project is dropped (§1.7); got %v", quiet.Sections)
	}
}

// TestOverdueIsOpenAndPastDue, and a blown deadline is unhideable: `missed` is
// deliberately not terminal (§1.7), so no default hiding removes it.
func TestOverdueIsUnhideable(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	add(t, w, "projects.late", "name", "Late", "description", "A deadline that went by.",
		"created", "2026-01-01", "due", "2026-02-01")
	add(t, w, "projects.later", "name", "Later", "description", "An earlier one.",
		"created", "2026-01-01", "due", "2026-01-15")

	// Without --all, which is the point: a deadline that went by cannot be
	// quieted by anything short of closing the thing.
	got := run(t, root, review.Options{Only: []review.Group{review.GroupOverdue}})
	assertLocators(t, section(t, got, review.GroupOverdue), "projects.later", "projects.late")

	// Closing it is the one thing that does. `done` is terminal (§1.7).
	set(t, w, "projects.later", "status", "done")
	set(t, w, "projects.late", "status", "done")
	quiet := run(t, root, review.Options{Only: []review.Group{review.GroupOverdue}})
	if len(quiet.Sections) != 0 {
		t.Errorf("a done project is not overdue, it is done; got %v", quiet.Sections)
	}
}

// TestAMissedKeyResultIsStillReviewed is the same rule one level down and the
// phase's stated done-when: a key-result past its due date derives `missed`
// (§4.3), `missed` is not terminal (§1.7), so nothing hides it.
func TestAMissedKeyResultIsStillReviewed(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.launch",
		"name", "Launch", "type", "boolean", "target", "true",
		"created", "2026-01-05", "due", "2026-02-01")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupOverdue}})
	assertLocators(t, section(t, got, review.GroupOverdue),
		"projects.acme-migration.objectives.q1-growth.key-results.launch")

	ent := got.Sections[0].Items[0].Entity
	if ent.EffectiveStatus != "missed" {
		t.Errorf("effective status = %q, want missed", ent.EffectiveStatus)
	}
	if ent.Terminal {
		t.Error("missed must not be terminal (§1.7) — a blown deadline stays visible")
	}
}

// TestBehindIsPaceBelowAtRiskPace is §20's fourth row, and the threshold has to
// be the same one §4.3 reads or a key-result could read `at-risk` and not be
// listed as behind.
func TestBehindIsPaceBelowAtRiskPace(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	configure(t, root, "key-result.at-risk-pace", "0.80")
	// Two months into a nine-month key-result, barely moved: pace well below 0.80.
	if _, err := w.Measure(loc(t, "projects.acme-migration.objectives.q1-growth.key-results.signups"),
		"500/9000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}

	got := run(t, root, review.Options{Only: []review.Group{review.GroupBehind}})
	assertLocators(t, section(t, got, review.GroupBehind),
		"projects.acme-migration.objectives.q1-growth.key-results.signups")

	item := got.Sections[0].Items[0]
	if item.Threshold.Key != "key-result.at-risk-pace" {
		t.Errorf("threshold key = %q, want key-result.at-risk-pace", item.Threshold.Key)
	}
	if item.Entity.EffectiveStatus != "at-risk" {
		t.Errorf("effective status = %q, want at-risk — --behind and §4.3 read one knob",
			item.Entity.EffectiveStatus)
	}
	if item.HasDays {
		t.Error("--behind measures a pace, not a count of days")
	}
}

// TestBehindIsOrderedByDistanceBelowTheThreshold is §20's "ordered within a
// group by distance past the threshold", for the one group whose distance is
// not a number of days.
func TestBehindIsOrderedByDistanceBelowTheThreshold(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	configure(t, root, "key-result.at-risk-pace", "0.80")
	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.churn",
		"name", "Churn", "type", "number", "start", "0", "target", "100",
		"created", "2026-01-05", "due", "2026-09-30")
	kr := "projects.acme-migration.objectives.q1-growth.key-results."
	// signups has moved a little; churn has not moved at all, so its pace is 0
	// and it is further below the threshold.
	if _, err := w.Measure(loc(t, kr+"signups"), "700/10000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if _, err := w.Measure(loc(t, kr+"churn"), "0", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}

	got := run(t, root, review.Options{Only: []review.Group{review.GroupBehind}})
	assertLocators(t, section(t, got, review.GroupBehind), kr+"churn", kr+"signups")

	items := got.Sections[0].Items
	if !(items[0].Over > items[1].Over) {
		t.Errorf("over = %v then %v, want the furthest below first", items[0].Over, items[1].Over)
	}
}

// TestContainersNeverAppearInAnyGroup is §20's third bullet: "a row you can
// neither act on nor set is the same noise §16.2 keeps out of `list`".
func TestContainersNeverAppearInAnyGroup(t *testing.T) {
	root := fixture(t)
	got := run(t, root, review.Options{})
	for _, s := range got.Sections {
		for _, item := range s.Items {
			if item.Entity.Container {
				t.Errorf("%s in %s: containers never appear in any group (§20)", item.Entity.Locator, s.Group)
			}
		}
	}
	if got.Total == 0 {
		t.Fatal("the fixture must produce findings, or this test proves nothing")
	}
}

// TestTerminalAndArchivedAreExcludedUnlessAll is §20's exclusion rule, and the
// one place `review` reads the tree differently from `list`: archived things are
// somewhere else for `list` (§16.2) and brought back by `--all` here.
func TestTerminalAndArchivedAreExcludedUnlessAll(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	add(t, w, "projects.old", "name", "Old", "description", "Finished long ago.",
		"created", "2026-01-01")
	plan, err := w.PlanArchive(loc(t, "projects.old"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	// Stale by 33 days, and terminal, so only one of the two flags in §20's
	// sentence is doing the work here.
	add(t, w, "projects.shelved", "name", "Shelved", "description", "Not happening.",
		"created", "2026-01-01")
	set(t, w, "projects.shelved", "status", "dropped")

	for _, s := range run(t, root, review.Options{}).Sections {
		for _, item := range s.Items {
			if item.Entity.Locator.String() == "projects.shelved" {
				t.Errorf("a dropped project is terminal and excluded unless --all (§20)")
			}
		}
	}

	for _, name := range section(t, run(t, root, review.Options{}), review.GroupStale) {
		if strings.HasPrefix(name, "archive.") {
			t.Errorf("%s: archived things are excluded unless --all (§20)", name)
		}
	}

	all := section(t, run(t, root, review.Options{All: true}), review.GroupStale)
	for _, want := range []string{"archive.projects.old", "projects.shelved"} {
		if !slices.Contains(all, want) {
			t.Errorf("--all must bring back %s; got %v", want, all)
		}
	}
}

// TestATerminalStatusQuietsEverythingBeneathIt: §1.7's cascade is what decides
// exclusion, so a whole subtree goes quiet when its project is dropped rather
// than each entity needing its own terminal status.
func TestATerminalStatusQuietsEverythingBeneathIt(t *testing.T) {
	root := fixture(t)
	set(t, writer(t, root), "projects.acme-migration", "status", "done")

	for _, s := range run(t, root, review.Options{}).Sections {
		for _, item := range s.Items {
			if strings.HasPrefix(item.Entity.Locator.String(), "projects.acme-migration") {
				t.Errorf("%s in %s: a done project quiets what is beneath it (§1.7)", item.Entity.Locator, s.Group)
			}
		}
	}
}

// TestScopeIncludesTheEntityYouNamed. `list` is a listing of contents and `show`
// is how you see the thing you named (§16.1); `review <locator>` is a question
// about a region, and the root of the region is in the region.
func TestScopeIncludesTheEntityYouNamed(t *testing.T) {
	root := fixture(t)
	got := section(t, run(t, root, review.Options{Scope: loc(t, "areas.fitness")}), review.GroupStale)
	assertLocators(t, got, "areas.fitness", "areas.fitness.training")
}

// TestScopeNamingNothingIsAnError, for the reason Phase 10 gave: an empty
// result reads as "you have nothing to review there", which is a different and
// wrong answer to a mistyped locator.
func TestScopeNamingNothingIsAnError(t *testing.T) {
	root := fixture(t)
	if _, err := review.Run(reader(t, root), review.Options{Scope: loc(t, "projects.nope")}); err == nil {
		t.Error("a scope naming nothing must be an error")
	}
}

// TestEmptyGroupsAreOmitted keeps the output honest: a heading with no rows
// beneath it says there is a group to look at when there is not.
func TestEmptyGroupsAreOmitted(t *testing.T) {
	root := fixture(t)
	got := run(t, root, review.Options{})
	for _, s := range got.Sections {
		if len(s.Items) == 0 {
			t.Errorf("%s: an empty group must not be a section", s.Group)
		}
	}
	if section(t, got, review.GroupBlocked) != nil {
		t.Error("nothing in the fixture is blocked")
	}
}

// TestLimitTruncatesEachGroupAndTotalsSaySo. §20 groups by reason, so a limit
// that spent itself on the first group would silence whole reasons — and §23
// requires the counts to explain the truncation either way.
func TestLimitTruncatesEachGroup(t *testing.T) {
	root := fixture(t)
	got := run(t, root, review.Options{Limit: 2})

	stale := got.Sections[0]
	if stale.Group != review.GroupStale {
		t.Fatalf("first section = %s, want stale", stale.Group)
	}
	if len(stale.Items) != 2 {
		t.Errorf("got %d items, want 2", len(stale.Items))
	}
	if stale.Total != 5 {
		t.Errorf("total = %d, want 5 — the count before --limit", stale.Total)
	}
	if got.Total != 5 || got.Shown != 2 {
		t.Errorf("total/shown = %d/%d, want 5/2", got.Total, got.Shown)
	}
}

// TestOneEntityCanHaveTwoReasons: the groups are reasons, not a partition, and
// something both stale and overdue is listed under each.
func TestOneEntityCanHaveTwoReasons(t *testing.T) {
	root := fixture(t)
	add(t, writer(t, root), "projects.late", "name", "Late",
		"description", "A deadline that went by.", "created", "2026-01-01", "due", "2026-02-01")

	got := run(t, root, review.Options{})
	for _, g := range []review.Group{review.GroupStale, review.GroupOverdue} {
		found := false
		for _, name := range section(t, got, g) {
			if name == "projects.late" {
				found = true
			}
		}
		if !found {
			t.Errorf("projects.late is missing from %s", g)
		}
	}
}

// TestTimerGroupsAreOrderedByDistancePastTheThreshold is §20's ordering clause
// for the groups whose distance is a number of days.
//
// Distance past the threshold, not raw age: two kinds with different thresholds
// in one `stale` group are not comparable by age at all, which is the case this
// pins. The older area is *less* far past its own knob than the younger project
// is past its.
func TestTimerGroupsAreOrderedByDistancePastTheThreshold(t *testing.T) {
	root := plantTree(t)
	w := writer(t, root)
	add(t, w, "areas.fitness", "name", "Fitness", "description", "Staying in one piece.",
		"created", "2026-01-01") // 63 days old, 3 past a 60-day knob
	add(t, w, "projects.website", "name", "Website", "description", "A refresh.",
		"created", "2026-02-01") // 32 days old, 22 past a 10-day knob
	configure(t, root, "area.stale-after", "60", "project.stale-after", "10")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	assertLocators(t, section(t, got, review.GroupStale), "projects.website", "areas.fitness")

	items := got.Sections[0].Items
	if items[0].Over != 22 || items[1].Over != 3 {
		t.Errorf("over = %v, %v; want 22, 3", items[0].Over, items[1].Over)
	}
	// And the raw ages run the other way, so the ordering could not have come
	// from the age alone.
	if !(items[0].Days < items[1].Days) {
		t.Errorf("days = %d, %d; the older thing must be second here", items[0].Days, items[1].Days)
	}
}

// TestOverdueIsOrderedByHowFarPastTheDeadline, and a tie falls back to locator
// so the output is stable enough to be a golden file (§0.2).
func TestOverdueIsOrderedByHowFarPastTheDeadline(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	add(t, w, "projects.b-late", "name", "B", "description", "Late.",
		"created", "2026-01-01", "due", "2026-02-01")
	add(t, w, "projects.a-later", "name", "A", "description", "Later.",
		"created", "2026-01-01", "due", "2026-01-15")
	add(t, w, "projects.a-tied", "name", "A tied", "description", "Same deadline as b-late.",
		"created", "2026-01-01", "due", "2026-02-01")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupOverdue}})
	assertLocators(t, section(t, got, review.GroupOverdue),
		"projects.a-later", "projects.a-tied", "projects.b-late")
}

// TestSkillsFiresOffReviewCadenceOrdering covers the fifth group's ordering,
// which shares staleItem with --stale but reads a different knob.
func TestSkillsAreOrderedByDistancePastTheCadence(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	add(t, w, "skills.old", "name", "Old", "description", "when doing the old thing",
		"created", "2026-01-01")
	add(t, w, "skills.newer", "name", "Newer", "description", "when doing the newer thing",
		"created", "2026-01-20")

	got := run(t, root, review.Options{Only: []review.Group{review.GroupSkills}})
	assertLocators(t, section(t, got, review.GroupSkills), "skills.old", "skills.newer")
}

// TestASkillReachesNoGroupButSkills is §20's bullet in full. `--stale` bars
// skills by name; the other three are barred by what a skill *is* — no status,
// no due date, and no key-result arithmetic (§1.7, §15) — and that is worth
// pinning, because the reason is structural rather than written down in
// classify.
func TestASkillReachesNoGroupButSkills(t *testing.T) {
	root := fixture(t)
	add(t, writer(t, root), "skills.commit-style", "name", "Commit style",
		"description", "when writing a commit message", "created", "2026-01-01")
	configure(t, root, "key-result.at-risk-pace", "0.80")

	got := run(t, root, review.Options{})
	for _, s := range got.Sections {
		if s.Group == review.GroupSkills {
			continue
		}
		for _, item := range s.Items {
			if item.Entity.Kind == kindmeta.KindSkill {
				t.Errorf("%s reached %s; skills are reached by --skills alone (§20)",
					item.Entity.Locator, s.Group)
			}
		}
	}
	// The skill is genuinely present in the review, or the loop above proves
	// nothing about it.
	assertLocators(t, section(t, got, review.GroupSkills), "skills.commit-style")
}

// TestAnItemsPaceIsSafeToAskFor: the `behind` group's display reads the pace
// through Item.Pace rather than through Entity.KeyResult, so an item from any
// other group answers false instead of panicking.
func TestAnItemsPaceIsSafeToAskFor(t *testing.T) {
	root := fixture(t)
	got := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	for _, item := range got.Sections[0].Items {
		if item.Entity.Kind == kindmeta.KindKeyResult {
			continue
		}
		if _, ok := item.Pace(); ok {
			t.Errorf("%s is not a key-result and must have no pace", item.Entity.Locator)
		}
	}
}

// TestSuppressedExcludesStaleAndSkills is §20/§28: an unexpired suppression
// silences --stale and --skills but not --overdue or --blocked.
func TestSuppressedExcludesStaleAndSkills(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	configure(t, root, "project.stale-after", "30", "review.cadence", "30")
	if _, err := w.Suppress(loc(t, "projects.acme-migration"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	stale := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	for _, item := range sectionItems(t, stale, review.GroupStale) {
		if item.Entity.Locator.String() == "projects.acme-migration" {
			t.Errorf("projects.acme-migration is in --stale; suppressions exclude it (§28)")
		}
	}

	add(t, w, "skills.old", "name", "Old", "description", "when doing the old thing", "created", "2026-01-01")
	if _, err := w.Suppress(loc(t, "skills.old"), "2027-06-01", "on hold"); err != nil {
		t.Fatalf("Suppress skill: %v", err)
	}
	skills := run(t, root, review.Options{Only: []review.Group{review.GroupSkills}})
	for _, item := range sectionItems(t, skills, review.GroupSkills) {
		if item.Entity.Locator.String() == "skills.old" {
			t.Errorf("skills.old is in --skills; suppressions exclude it (§28)")
		}
	}

	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.launch",
		"name", "Launch", "type", "boolean", "target", "true", "created", "2026-01-05", "due", "2026-02-01")

	overdue := run(t, root, review.Options{Only: []review.Group{review.GroupOverdue}})
	foundOverdue := false
	for _, item := range sectionItems(t, overdue, review.GroupOverdue) {
		if item.Entity.Locator.String() == "projects.acme-migration.objectives.q1-growth.key-results.launch" {
			foundOverdue = true
		}
	}
	if !foundOverdue {
		t.Error("launch key-result is overdue and must still appear under --overdue")
	}
}

// TestSuppressedListsActiveSuppressionsSoonestFirst is §20's sixth group:
// every unexpired suppression, entities and skills, ordered soonest-until-first.
func TestSuppressedListsActiveSuppressionsSoonestFirst(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	if _, err := w.Suppress(loc(t, "projects.acme-migration"), "2027-06-01", "paused longer"); err != nil {
		t.Fatalf("Suppress project: %v", err)
	}
	add(t, w, "skills.old", "name", "Old", "description", "when doing the old thing", "created", "2026-01-01")
	if _, err := w.Suppress(loc(t, "skills.old"), "2027-03-01", "paused shorter"); err != nil {
		t.Fatalf("Suppress skill: %v", err)
	}

	got := run(t, root, review.Options{Only: []review.Group{review.GroupSuppressed}})
	items := sectionItems(t, got, review.GroupSuppressed)
	locs := make([]string, len(items))
	for i, item := range items {
		locs[i] = item.Entity.Locator.String()
	}
	assertLocators(t, locs, "skills.old", "projects.acme-migration")
	if locs[0] != "skills.old" {
		t.Errorf("first suppressed item = %s, want skills.old (until 2027-03-01, soonest first)", locs[0])
	}
}

// TestSuppressedFallsBackToTheJournalWhenCacheIsAbsent is §28.4's upgrade
// case: a suppress event in the journal must exclude --stale even when
// state.toml has not yet been backfilled with [suppression].
func TestSuppressedFallsBackToTheJournalWhenCacheIsAbsent(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	configure(t, root, "project.stale-after", "30")
	if _, err := w.Suppress(loc(t, "projects.acme-migration"), "2027-03-01", "paused"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	// Strip the cache write-through just performed, leaving the journal intact.
	current, err := os.ReadFile(filepath.Join(root, "projects/acme-migration/.para/state.toml"))
	if err != nil {
		t.Fatal(err)
	}
	without, _, ok := strings.Cut(string(current), "\n[suppression]")
	if !ok {
		t.Fatalf("state.toml has no [suppression] to strip:\n%s", current)
	}
	if err := os.WriteFile(filepath.Join(root, "projects/acme-migration/.para/state.toml"), []byte(without+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	for _, item := range sectionItems(t, stale, review.GroupStale) {
		if item.Entity.Locator.String() == "projects.acme-migration" {
			t.Errorf("projects.acme-migration is in --stale; journal fallback must honor suppression")
		}
	}
	sup := run(t, root, review.Options{Only: []review.Group{review.GroupSuppressed}})
	assertLocators(t, section(t, sup, review.GroupSuppressed), "projects.acme-migration")
}

// eligible for --stale again and vanishes from --suppressed.
func TestExpiredSuppressionReachesStaleAgain(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	configure(t, root, "project.stale-after", "30")
	if _, err := w.Suppress(loc(t, "projects.acme-migration"), "2026-01-15", "already over"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}

	sup := run(t, root, review.Options{Only: []review.Group{review.GroupSuppressed}})
	if len(sectionItems(t, sup, review.GroupSuppressed)) != 0 {
		t.Errorf("--suppressed = %d items, want none after until passed", len(sectionItems(t, sup, review.GroupSuppressed)))
	}
	stale := run(t, root, review.Options{Only: []review.Group{review.GroupStale}})
	found := false
	for _, loc := range section(t, stale, review.GroupStale) {
		if loc == "projects.acme-migration" {
			found = true
		}
	}
	if !found {
		t.Errorf("--stale = %v, want projects.acme-migration after suppression expired", section(t, stale, review.GroupStale))
	}
}
