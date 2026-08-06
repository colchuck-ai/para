package query_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/tagexpr"
	"github.com/colchuck-ai/para/internal/view"
)

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
	if _, err := e.Set(loc(t, l), fields(kv...), ""); err != nil {
		t.Fatalf("Set(%s): %v", l, err)
	}
}

// fixture is the tree every test below lists: two projects with a full
// objective/key-result chain under one of them, two areas, a resource, a skill,
// and an archived project.
func fixture(t *testing.T) string {
	root := plantTree(t)
	w := writer(t, root)

	add(t, w, "projects.acme-migration", "name", "Acme migration",
		"description", "Rebuild the consumer.", "status", "in-progress",
		"priority", "high", "due", "2026-09-30", "tags", "consumer,kafka")
	add(t, w, "projects.acme-migration.objectives.q1-growth",
		"name", "Grow signups", "description", "Move the top of the funnel.",
		"status", "in-progress", "priority", "medium")
	add(t, w, "projects.acme-migration.objectives.q1-growth.key-results.signups",
		"name", "Weekly signups", "description", "Sign-ups per week.",
		"type", "ratio", "start", "480/9000", "target", "2000/12000", "due", "2026-09-30")
	add(t, w, "projects.website", "name", "Website", "description", "A refresh.",
		"status", "planned", "priority", "low", "tags", "kafka")

	add(t, w, "areas.health", "name", "Health", "description", "Staying in one piece.")
	add(t, w, "areas.health.training", "name", "Training", "description", "The weekly plan.",
		"tags", "fitness")
	add(t, w, "resources.rust", "name", "Rust", "description", "Notes.", "tags", "rust,reference")
	add(t, w, "skills.signups-report", "name", "Signups report",
		"description", "when asked for the weekly signups number", "scope", "projects")

	add(t, w, "projects.old", "name", "Old", "description", "Finished long ago.")
	plan, err := w.PlanArchive(loc(t, "projects.old"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatal(err)
	}
	return root
}

func list(t *testing.T, root string, opts query.Options) query.Result {
	t.Helper()
	res, err := query.List(reader(t, root), opts)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return res
}

func locators(res query.Result) []string {
	out := make([]string, 0, len(res.Entities))
	for _, e := range res.Entities {
		out = append(out, e.Locator.String())
	}
	return out
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

// TestListNeverEmitsAContainerRowButFindsWhatIsBeneathOne is §16.2's
// transparency, and the phase's stated done-when condition.
func TestListNeverEmitsAContainerRowButFindsWhatIsBeneathOne(t *testing.T) {
	root := fixture(t)
	got := list(t, root, query.Options{})

	for _, e := range got.Entities {
		if e.Container {
			t.Errorf("%s is a container and must never be a row (§16.2)", e.Locator)
		}
	}

	// The objective and the key-result both sit under containers, and both are
	// found — which is the other half of transparency.
	assertLocators(t, locators(got),
		"areas.health",
		"areas.health.training",
		"projects.acme-migration",
		"projects.acme-migration.objectives.q1-growth",
		"projects.acme-migration.objectives.q1-growth.key-results.signups",
		"projects.website",
		"resources.rust",
		"skills.signups-report",
	)
}

// TestArchiveIsNotTraversedUnlessNamed is §16.2 and §1.6: archived things are
// not hidden, they are somewhere else.
func TestArchiveIsNotTraversedUnlessNamed(t *testing.T) {
	root := fixture(t)

	for _, name := range locators(list(t, root, query.Options{})) {
		if strings.HasPrefix(name, "archive.") {
			t.Errorf("an unscoped list must not traverse archive/: found %s", name)
		}
	}

	got := list(t, root, query.Options{Scope: loc(t, "archive.projects")})
	assertLocators(t, locators(got), "archive.projects.old")
}

func TestScopeListsBeneathAndNotTheThingNamed(t *testing.T) {
	root := fixture(t)
	got := list(t, root, query.Options{Scope: loc(t, "projects.acme-migration")})
	assertLocators(t, locators(got),
		"projects.acme-migration.objectives.q1-growth",
		"projects.acme-migration.objectives.q1-growth.key-results.signups",
	)
}

// TestDirectAppliesTransparencyFirst is §17's worked case: `list projects.acme
// --direct` shows that project's objectives, one container down.
func TestDirectAppliesTransparencyFirst(t *testing.T) {
	root := fixture(t)

	got := list(t, root, query.Options{
		Scope:  loc(t, "projects.acme-migration"),
		Filter: query.Filter{Direct: true},
	})
	assertLocators(t, locators(got), "projects.acme-migration.objectives.q1-growth")

	// From the root, --direct is the top level of each bucket.
	got = list(t, root, query.Options{Filter: query.Filter{Direct: true}})
	assertLocators(t, locators(got),
		"areas.health", "projects.acme-migration", "projects.website",
		"resources.rust", "skills.signups-report",
	)
}

func TestListRefusesAScopeThatNamesNothing(t *testing.T) {
	root := fixture(t)
	_, err := query.List(reader(t, root), query.Options{Scope: loc(t, "projects.nope")})
	if err == nil {
		t.Fatal("a mistyped locator must be an error, not an empty list")
	}
}

// TestTerminalHidingIsItsOwnNumber is the plan's third settled decision: the
// count line keeps meaning what the filters matched, and hiding is reported
// separately with the flag that undoes it.
func TestTerminalHidingIsItsOwnNumber(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	set(t, w, "projects.website", "status", "done")

	got := list(t, root, query.Options{})
	if got.Hidden != 1 {
		t.Errorf("hidden: got %d, want 1", got.Hidden)
	}
	if len(got.HiddenStatuses) != 1 || got.HiddenStatuses[0] != "done" {
		t.Errorf("hidden statuses: got %v, want [done]", got.HiddenStatuses)
	}
	for _, name := range locators(got) {
		if name == "projects.website" {
			t.Error("a done project is hidden without --all")
		}
	}

	all := list(t, root, query.Options{Filter: query.Filter{All: true}})
	if all.Hidden != 0 {
		t.Errorf("--all hides nothing: got %d", all.Hidden)
	}
	if all.Total != got.Total+1 {
		t.Errorf("--all totals: got %d, want %d", all.Total, got.Total+1)
	}
}

// TestTerminalCascadeHidesDescendants is §1.7's cascade reaching `list`.
func TestTerminalCascadeHidesDescendants(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	set(t, w, "projects.acme-migration", "status", "done")

	got := list(t, root, query.Options{})
	for _, name := range locators(got) {
		if strings.HasPrefix(name, "projects.acme-migration") {
			t.Errorf("%s is quieted by its ancestor and must be hidden (§1.7)", name)
		}
	}
	// The project, its objective, and its key-result: three rows.
	if got.Hidden != 3 {
		t.Errorf("hidden: got %d, want 3", got.Hidden)
	}
}

func TestFilters(t *testing.T) {
	root := fixture(t)
	cases := []struct {
		name string
		opts query.Options
		want []string
	}{
		{
			name: "status filters on effective status",
			opts: query.Options{Filter: query.Filter{Status: "planned"}},
			want: []string{"projects.website"},
		},
		{
			name: "priority, on the kinds that have one",
			opts: query.Options{Filter: query.Filter{Priority: "high"}},
			want: []string{"projects.acme-migration"},
		},
		{
			name: "a bare tag",
			opts: query.Options{Filter: query.Filter{Tags: mustExpr(t, "kafka")}},
			want: []string{"projects.acme-migration", "projects.website"},
		},
		{
			name: "a comma list is or",
			opts: query.Options{Filter: query.Filter{Tags: mustExpr(t, "rust,fitness")}},
			want: []string{"areas.health.training", "resources.rust"},
		},
		{
			name: "and not",
			opts: query.Options{Filter: query.Filter{Tags: mustExpr(t, "kafka and not consumer")}},
			want: []string{"projects.website"},
		},
		{
			name: "match reads name and description",
			opts: query.Options{Filter: query.Filter{Match: "funnel"}},
			want: []string{"projects.acme-migration.objectives.q1-growth"},
		},
		{
			name: "match is case-insensitive",
			opts: query.Options{Filter: query.Filter{Match: "ACME"}},
			want: []string{"projects.acme-migration"},
		},
		{
			name: "overdue is open and past due",
			opts: query.Options{Filter: query.Filter{Overdue: true}},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertLocators(t, locators(list(t, root, tc.opts)), tc.want...)
		})
	}
}

// TestMatchReadsJournalNoteBodies is §17's "the one read path that must open
// journals, because note bodies live only there".
func TestMatchReadsJournalNoteBodies(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	if _, err := w.Note(loc(t, "areas.health"), "the physio said to stop running", ""); err != nil {
		t.Fatal(err)
	}
	// A note attached to a change is as findable as a bare one: `note` is a
	// field on every event kind (§3.1).
	if _, err := w.Set(loc(t, "projects.website"), fields("status", "blocked"), "waiting on procurement"); err != nil {
		t.Fatal(err)
	}

	assertLocators(t, locators(list(t, root, query.Options{Filter: query.Filter{Match: "physio"}})),
		"areas.health")
	assertLocators(t, locators(list(t, root, query.Options{Filter: query.Filter{Match: "procurement"}})),
		"projects.website")
}

func TestSortAndLimit(t *testing.T) {
	root := fixture(t)

	// The default is locator order — §17 makes --sort explicit and lists
	// `locator` among its keys, so the default is that key, not an unstated one.
	def := list(t, root, query.Options{})
	byLocator := list(t, root, query.Options{Sort: query.SortLocator})
	assertLocators(t, locators(def), locators(byLocator)...)

	byName := list(t, root, query.Options{Sort: query.SortName})
	assertLocators(t, locators(byName),
		"projects.acme-migration",
		"projects.acme-migration.objectives.q1-growth",
		"areas.health",
		"resources.rust",
		"skills.signups-report",
		"areas.health.training",
		"projects.website",
		"projects.acme-migration.objectives.q1-growth.key-results.signups",
	)

	rev := list(t, root, query.Options{Sort: query.SortName, Reverse: true})
	want := locators(byName)
	slicesReverse(want)
	assertLocators(t, locators(rev), want...)

	limited := list(t, root, query.Options{Sort: query.SortName, Limit: 3})
	if limited.Total != 8 || len(limited.Entities) != 3 {
		t.Errorf("limit: total %d shown %d, want 8 and 3", limited.Total, len(limited.Entities))
	}
	assertLocators(t, locators(limited), locators(byName)[:3]...)
}

// TestSortByAKeyOnlySomeKindsHave is §17's amendment to ADR 0002: groups that
// have the field sort by it, and a group that does not falls back to locator
// order and comes last.
func TestSortByAKeyOnlySomeKindsHave(t *testing.T) {
	root := fixture(t)
	got := list(t, root, query.Options{Sort: query.SortPriority})

	// §15 gives `priority` to projects, areas, and objectives, so those are the
	// three groups, in kind order, each ranked by §1.7 within itself. An area
	// with no priority set sorts last inside its own group rather than being
	// demoted to the fallback: absent-on-the-kind and absent-as-a-value are the
	// same case (§17), and the group is chosen by the kind.
	assertLocators(t, locators(got),
		"projects.acme-migration", // high
		"projects.website",        // low
		"areas.health",            // unset, and so last among the areas
		"areas.health.training",
		"projects.acme-migration.objectives.q1-growth", // medium
		// The fallback: resources, key-results, and skills have no priority at
		// all, so they come last, in locator order (§17).
		"projects.acme-migration.objectives.q1-growth.key-results.signups",
		"resources.rust",
		"skills.signups-report",
	)
}

// TestSortByAKeyNoMatchedKindHasIsAnError is the other half of §17's rule.
func TestSortByAKeyNoMatchedKindHasIsAnError(t *testing.T) {
	root := fixture(t)

	// Areas have no progress, and nothing else matched.
	_, err := query.List(reader(t, root), query.Options{
		Scope: loc(t, "areas"), Sort: query.SortProgress,
	})
	if err == nil {
		t.Fatal("a key no matched kind has must be an error")
	}
	if got := err.Error(); !strings.Contains(got, "progress") || !strings.Contains(got, "key-result") {
		t.Errorf("the error must name the key and the kinds it applies to: %q", got)
	}

	// The same key across the whole tree is fine, because key-results matched.
	if _, err := query.List(reader(t, root), query.Options{Sort: query.SortProgress}); err != nil {
		t.Errorf("cross-kind --sort progress must not fail: %v", err)
	}
}

func TestSortKeyValidation(t *testing.T) {
	if _, err := query.ParseSortKey("nonsense"); err == nil {
		t.Error("an unknown sort key must be refused")
	}
	for _, k := range query.SortKeys() {
		if _, err := query.ParseSortKey(string(k)); err != nil {
			t.Errorf("%s should parse: %v", k, err)
		}
	}
}

func mustExpr(t *testing.T, s string) tagexpr.Expr {
	t.Helper()
	e, err := tagexpr.Parse(s)
	if err != nil {
		t.Fatalf("tagexpr.Parse(%q): %v", s, err)
	}
	return e
}

func slicesReverse(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// TestAnExplicitTerminalStatusIsNotHidden: `--status done` selects exactly the
// entities the default hiding removes, so composing the two would make the flag
// return nothing, ever. Asking for done things is the request `--all` signals,
// said more precisely.
func TestAnExplicitTerminalStatusIsNotHidden(t *testing.T) {
	root := fixture(t)
	w := writer(t, root)
	set(t, w, "projects.website", "status", "done")

	got := list(t, root, query.Options{Filter: query.Filter{Status: "done"}})
	assertLocators(t, locators(got), "projects.website")
	if got.Hidden != 0 {
		t.Errorf("hidden: got %d, want 0 — nothing was withheld", got.Hidden)
	}

	// The exemption is exactly as wide as what was asked for: it is the status
	// filter that grants it, so an unfiltered list still hides the same
	// entity. (Nothing else can appear in Hidden here — the filter runs first,
	// so a `--status done` list has only done things to withhold in the first
	// place, which is the whole reason composing the two produced nothing.)
	set(t, w, "projects.acme-migration", "status", "dropped")
	unfiltered := list(t, root, query.Options{})
	for _, name := range locators(unfiltered) {
		if name == "projects.website" {
			t.Error("without the filter, a done project is still hidden")
		}
	}
	if len(unfiltered.HiddenStatuses) != 2 {
		t.Errorf("hidden statuses: got %v, want both done and dropped", unfiltered.HiddenStatuses)
	}
}

// TestIncludeSelfPutsTheNamedEntityInTheResult is the seam `review` needs and
// `list` must never take: `list` is a listing of contents (§16.1: "`show` is how
// you see the thing you named"), while `review` is a question about a region of
// the tree, and the root of the region is in the region.
func TestIncludeSelfPutsTheNamedEntityInTheResult(t *testing.T) {
	root := fixture(t)

	without := list(t, root, query.Options{Scope: loc(t, "projects.acme-migration")})
	assertLocators(t, locators(without),
		"projects.acme-migration.objectives.q1-growth",
		"projects.acme-migration.objectives.q1-growth.key-results.signups")

	with := list(t, root, query.Options{
		Scope:       loc(t, "projects.acme-migration"),
		IncludeSelf: true,
	})
	assertLocators(t, locators(with),
		"projects.acme-migration",
		"projects.acme-migration.objectives.q1-growth",
		"projects.acme-migration.objectives.q1-growth.key-results.signups")
}

// TestIncludeSelfOnAContainerStillEmitsNoContainerRow: transparency is decided
// after the scope is included, not before, so naming a container adds nothing.
func TestIncludeSelfOnAContainerStillEmitsNoContainerRow(t *testing.T) {
	root := fixture(t)
	got := list(t, root, query.Options{Scope: loc(t, "projects"), IncludeSelf: true})
	for _, e := range got.Entities {
		if e.Container {
			t.Errorf("%s is a container and must never be a row (§16.2)", e.Locator)
		}
	}
}

// TestIncludeArchivedReachesArchiveWithoutNamingIt is §20's `--all`, which
// covers archived things as well as terminal ones — unlike `list`, where
// archive/ is somewhere else rather than hidden (§1.6, §16.2).
func TestIncludeArchivedReachesArchiveWithoutNamingIt(t *testing.T) {
	root := fixture(t)

	for _, name := range locators(list(t, root, query.Options{})) {
		if strings.HasPrefix(name, "archive.") {
			t.Fatalf("%s: archive/ is not traversed unless named (§16.2)", name)
		}
	}

	got := locators(list(t, root, query.Options{IncludeArchived: true}))
	if !slices.Contains(got, "archive.projects.old") {
		t.Errorf("got %v, want it to include archive.projects.old", got)
	}
}
