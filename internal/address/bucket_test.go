package address

import (
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
)

// TestBucketRoundTrip is R3/R17's bucket table, exercised explicitly rather
// than left to the property test's random draws: the four buckets a noun
// with an empty chain names, plus the three archived mirrors R7/R10 give
// three of them (skills excluded — §1.6, skills are not archivable, pinned
// separately by TestSkillCannotBeArchived).
//
// Both entry points are checked for each row: Parse's two-token CLI form
// with an empty chain, and the Address <-> Locator conversion, so a bucket
// typed on the command line and a bucket read back from a stored Locator
// agree.
func TestBucketRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		noun     Noun
		archived bool
		wantLoc  []string
	}{
		{"project bucket", Project, false, []string{"projects"}},
		{"area bucket", Area, false, []string{"areas"}},
		{"resource bucket", Resource, false, []string{"resources"}},
		{"skill bucket", Skill, false, []string{"skills"}},
		{"archived project bucket", Project, true, []string{"archive", "projects"}},
		{"archived area bucket", Area, true, []string{"archive", "areas"}},
		{"archived resource bucket", Resource, true, []string{"archive", "resources"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Parse's two-token form: an empty chain is the bucket (R17).
			// Parse itself never sets Archived (R7: that's a flag applied by
			// the CLI layer), so build the archived case by hand the way that
			// layer will, and check Validate() still accepts it.
			parsed, err := Parse(c.noun.String(), "")
			if err != nil {
				t.Fatalf("Parse(%q, \"\"): %v", c.noun.String(), err)
			}
			if parsed.Noun != c.noun || len(parsed.Chain) != 0 {
				t.Fatalf("Parse(%q, \"\") = %+v, want noun %v with no chain", c.noun.String(), parsed, c.noun)
			}
			parsed.Archived = c.archived
			if err := parsed.Validate(); err != nil {
				t.Fatalf("Validate() on the bucket with Archived=%v: %v", c.archived, err)
			}

			want := Address{Noun: c.noun, Archived: c.archived}

			loc, err := parsed.ToLocator()
			if err != nil {
				t.Fatalf("ToLocator(%+v): %v", parsed, err)
			}
			assertLocEqual(t, loc, locator.Locator(c.wantLoc))

			back, err := FromLocator(loc)
			if err != nil {
				t.Fatalf("FromLocator(%v): %v", loc, err)
			}
			assertAddrEqual(t, back, want)
		})
	}
}

// TestSkillBucketIsNotArchivable is the one bucket §1.6 excludes from the
// three archived mirrors: a skill's chain is empty either way, so this is
// the one case TestSkillCannotBeArchived's entity-chain cases do not cover.
func TestSkillBucketIsNotArchivable(t *testing.T) {
	a := Address{Noun: Skill, Archived: true}
	if err := a.Validate(); err == nil {
		t.Fatal("Validate() on an archived skill bucket succeeded, want a refusal")
	}
	if _, err := FromLocator(locator.Locator{"archive", "skills"}); err == nil {
		t.Fatal("FromLocator(archive.skills) succeeded, want a refusal")
	}
}
