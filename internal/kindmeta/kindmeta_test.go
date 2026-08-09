package kindmeta

import (
	"errors"
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

func mustParse(t *testing.T, s string) locator.Locator {
	t.Helper()
	loc, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return loc
}

// TestKindOf covers every row of §1.3: each legal location and the kind it
// derives, archived or not.
func TestKindOf(t *testing.T) {
	cases := []struct {
		loc      string
		wantKind Kind
		archived bool
	}{
		{"projects.acme-migration", KindProject, false},
		{"projects.acme-migration.objectives.q1-growth", KindObjective, false},
		{"projects.acme-migration.objectives.q1-growth.key-results.signups", KindKeyResult, false},
		{"areas.health", KindArea, false},
		{"areas.health.training", KindArea, false},
		{"areas.health.training.deep.deeper", KindArea, false},
		{"resources.templates", KindResource, false},
		{"resources.templates.emails", KindResource, false},
		{"skills.signups-report", KindSkill, false},

		// archive/{projects,areas,resources}/… — same kinds, dormant.
		{"archive.projects.acme-migration", KindProject, true},
		{"archive.projects.acme-migration.objectives.q1-growth", KindObjective, true},
		{"archive.projects.acme-migration.objectives.q1-growth.key-results.signups", KindKeyResult, true},
		{"archive.areas.health", KindArea, true},
		{"archive.areas.health.training", KindArea, true},
		{"archive.resources.templates", KindResource, true},
	}
	for _, c := range cases {
		t.Run(c.loc, func(t *testing.T) {
			info, err := KindOf(mustParse(t, c.loc))
			if err != nil {
				t.Fatalf("KindOf(%q): %v", c.loc, err)
			}
			if info.Kind != c.wantKind {
				t.Errorf("KindOf(%q).Kind = %v, want %v", c.loc, info.Kind, c.wantKind)
			}
			if info.Archived != c.archived {
				t.Errorf("KindOf(%q).Archived = %v, want %v", c.loc, info.Archived, c.archived)
			}
		})
	}
}

// TestKindOfIllegalPositions covers §10's "misplaced" finding — locators
// shaped so that no position in §1.3 derives a kind.
func TestKindOfIllegalPositions(t *testing.T) {
	cases := []string{
		"projects",                                              // no id
		"projects.acme.extra",                                   // depth 2 under projects/: a project cannot nest
		"projects.acme.objectives.q1.extra",                     // an objective cannot nest
		"projects.acme.key-results.signups",                     // key-result outside a key-results/ container
		"projects.acme.objectives.q1.key-results.signups.extra", // past the leaf
		"areas",              // no id
		"resources",          // no id
		"skills",             // no id
		"skills.a.b",         // skills is one level only
		"archive",            // archive alone
		"archive.skills.foo", // skills cannot be archived
		"logs.foo",           // logs is not a bucket
		"foo.bar",            // unknown top-level segment
	}
	for _, loc := range cases {
		t.Run(loc, func(t *testing.T) {
			_, err := KindOf(mustParse(t, loc))
			if err == nil {
				t.Fatalf("KindOf(%q) = nil error, want misplaced", loc)
			}
			var perr *paraerr.Error
			if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
				t.Fatalf("KindOf(%q) error = %v, want KindValidation", loc, err)
			}
		})
	}
}

// TestKindOfReservedWordAsID covers §10's "collision" finding: a reserved
// word used as an id, at every position an id can appear.
func TestKindOfReservedWordAsID(t *testing.T) {
	cases := []string{
		"projects.objectives",
		"projects.archive",
		"projects.acme.objectives.key-results",
		"projects.acme.objectives.q1.key-results.skills",
		"areas.projects",
		"areas.health.archive",
		"resources.areas",
		"skills.logs",
	}
	for _, loc := range cases {
		t.Run(loc, func(t *testing.T) {
			_, err := KindOf(mustParse(t, loc))
			if err == nil {
				t.Fatalf("KindOf(%q) = nil error, want collision", loc)
			}
			var perr *paraerr.Error
			if !errors.As(err, &perr) || perr.Kind != paraerr.KindConflict {
				t.Fatalf("KindOf(%q) error = %v, want KindConflict", loc, err)
			}
		})
	}
}

func TestKindString(t *testing.T) {
	cases := map[Kind]string{
		KindProject:   "project",
		KindArea:      "area",
		KindResource:  "resource",
		KindObjective: "objective",
		KindKeyResult: "key-result",
		KindSkill:     "skill",
		KindContainer: "container",
		KindUnknown:   "unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", k, got, want)
		}
	}
}
