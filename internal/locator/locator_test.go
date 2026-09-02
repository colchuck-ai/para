package locator

import (
	"errors"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    Locator
		wantErr bool
	}{
		{"single segment", "projects", Locator{"projects"}, false},
		{"multi segment", "projects.acme-migration", Locator{"projects", "acme-migration"}, false},
		{
			"six segments", "projects.acme-migration.objectives.q1-growth.key-results.signups",
			Locator{"projects", "acme-migration", "objectives", "q1-growth", "key-results", "signups"},
			false,
		},
		{"skills entity", "skills.signups-report", Locator{"skills", "signups-report"}, false},
		{"empty string", "", nil, true},
		{"empty segment middle", "projects..acme", nil, true},
		{"leading dot", ".projects", nil, true},
		{"trailing dot", "projects.", nil, true},
		{"uppercase illegal", "Projects.Acme", nil, true},
		{"underscore illegal", "projects.acme_migration", nil, true},
		{"space illegal", "projects.acme migration", nil, true},
		{"digits and hyphen ok", "areas.health2-training", Locator{"areas", "health2-training"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %v, want error", c.in, got)
				}
				var perr *paraerr.Error
				if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
					t.Fatalf("Parse(%q) error = %v, want KindValidation", c.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", c.in, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("Parse(%q) = %v, want %v", c.in, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("Parse(%q) = %v, want %v", c.in, got, c.want)
				}
			}
		})
	}
}

func TestLocatorString(t *testing.T) {
	loc := Locator{"projects", "acme-migration", "objectives", "q1-growth"}
	want := "projects.acme-migration.objectives.q1-growth"
	if got := loc.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestParseStringRoundTrip(t *testing.T) {
	inputs := []string{
		"projects.acme-migration",
		"projects.acme-migration.objectives.q1-growth.key-results.signups",
		"areas.health.training",
		"archive.areas.health.training",
		"skills.signups-report",
	}
	for _, in := range inputs {
		loc, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		if got := loc.String(); got != in {
			t.Fatalf("Parse(%q).String() = %q, want %q", in, got, in)
		}
	}
}

func TestBucket(t *testing.T) {
	cases := []struct {
		loc  string
		want string
	}{
		{"projects.acme-migration", "projects"},
		{"archive.areas.health", "archive"},
		{"skills.signups-report", "skills"},
	}
	for _, tc := range cases {
		loc, err := Parse(tc.loc)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.loc, err)
		}
		if got := loc.Bucket(); got != tc.want {
			t.Errorf("Parse(%q).Bucket() = %q, want %q", tc.loc, got, tc.want)
		}
	}
	if got := (Locator{}).Bucket(); got != "" {
		t.Errorf("empty Locator.Bucket() = %q, want \"\"", got)
	}
}

func TestIsArchived(t *testing.T) {
	cases := []struct {
		loc  string
		want bool
	}{
		{"projects.acme-migration", false},
		{"archive.areas.health", true},
		{"archive.projects.old-migration", true},
		{"skills.signups-report", false},
	}
	for _, tc := range cases {
		loc, err := Parse(tc.loc)
		if err != nil {
			t.Fatalf("Parse(%q): %v", tc.loc, err)
		}
		if got := loc.IsArchived(); got != tc.want {
			t.Errorf("Parse(%q).IsArchived() = %v, want %v", tc.loc, got, tc.want)
		}
	}
}

func TestIsReserved(t *testing.T) {
	for _, w := range ReservedWords {
		if !IsReserved(w) {
			t.Errorf("IsReserved(%q) = false, want true", w)
		}
	}
	for _, w := range []string{"acme-migration", "health", "signups-report", "q1-growth"} {
		if IsReserved(w) {
			t.Errorf("IsReserved(%q) = true, want false", w)
		}
	}
}

// TestReservedWordsGrewToSeventeen pins R6: the seven singular nouns join
// the ten structural words, so an entity legitimately named e.g. "project"
// is refused as an id rather than silently accepted and later unparseable
// by a grammar whose first token is a noun. para-6g7 grows the list again,
// to nineteen: "links" (structural) and "link" (its singular noun), the
// same pair "key-results"/"key-result" already is.
func TestReservedWordsGrewToSeventeen(t *testing.T) {
	if len(ReservedWords) != 19 {
		t.Fatalf("len(ReservedWords) = %d, want 19", len(ReservedWords))
	}
	for _, noun := range []string{"project", "area", "resource", "objective", "key-result", "link", "skill", "container"} {
		if !IsReserved(noun) {
			t.Errorf("IsReserved(%q) = false, want true (R6)", noun)
		}
	}
}

func TestPath(t *testing.T) {
	cases := []struct {
		name    string
		loc     Locator
		want    string
		wantErr bool
	}{
		{"project", Locator{"projects", "acme-migration"}, "projects/acme-migration", false},
		{
			"key-result",
			Locator{"projects", "acme-migration", "objectives", "q1-growth", "key-results", "signups"},
			"projects/acme-migration/objectives/q1-growth/key-results/signups", false,
		},
		{
			"archived area",
			Locator{"archive", "areas", "health", "training"},
			"archive/areas/health/training", false,
		},
		{"skill exception", Locator{"skills", "signups-report"}, ".agents/skills/para-signups-report", false},
		{"skill bucket", Locator{"skills"}, ".agents/skills", false},
		{"skills nested illegal", Locator{"skills", "foo", "bar"}, "", true},
		{"empty locator", Locator{}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.loc.Path()
			if c.wantErr {
				if err == nil {
					t.Fatalf("Path() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Path() unexpected error: %v", err)
			}
			if got != c.want {
				t.Fatalf("Path() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFromPath(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		want    Locator
		wantErr bool
	}{
		{"project", "projects/acme-migration", Locator{"projects", "acme-migration"}, false},
		{"archived area", "archive/areas/health/training", Locator{"archive", "areas", "health", "training"}, false},
		{"skill exception", ".agents/skills/para-signups-report", Locator{"skills", "signups-report"}, false},
		{"skill without prefix not a locator", ".agents/skills/signups-report", nil, true},
		{"skill with empty id after prefix illegal", ".agents/skills/para-", nil, true},
		{"skill with illegal charset in id", ".agents/skills/para-Signups", nil, true},
		{"rule not a locator", ".agents/rules/para-signups-report.md", nil, true},
		{"agents yours not a locator", ".agents/notes.md", nil, true},
		{"empty path", "", nil, true},
		{"illegal segment", "projects/Acme", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FromPath(c.path)
			if c.wantErr {
				if err == nil {
					t.Fatalf("FromPath(%q) = %v, want error", c.path, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromPath(%q) unexpected error: %v", c.path, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("FromPath(%q) = %v, want %v", c.path, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Fatalf("FromPath(%q) = %v, want %v", c.path, got, c.want)
				}
			}
		})
	}
}

// TestPathFromPathRoundTrip is the property test §1.5 requires: the
// locator↔path isomorphism must round-trip for arbitrary locators, with the
// skills exception as the one deliberate special case. It also covers an
// archive-stub-shaped path (a segment with no entity behind it, §1.6) —
// that case needs no special code because the isomorphism is purely
// segment-for-segment.
func TestPathFromPathRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	alphabet := []string{"acme", "health", "training", "q1-growth", "signups", "growth2", "x", "y-z"}
	buckets := []string{"projects", "areas", "resources"}

	randSeg := func() string { return alphabet[rng.IntN(len(alphabet))] }

	for i := 0; i < 300; i++ {
		var segs []string
		if rng.IntN(4) == 0 {
			segs = append(segs, "archive")
		}
		bucket := buckets[rng.IntN(len(buckets))]
		segs = append(segs, bucket)
		switch bucket {
		case "projects":
			segs = append(segs, randSeg())
			if rng.IntN(2) == 0 {
				segs = append(segs, "objectives", randSeg())
				if rng.IntN(2) == 0 {
					segs = append(segs, "key-results", randSeg())
				}
			}
		default: // areas, resources: arbitrary nesting depth
			depth := 1 + rng.IntN(4)
			for d := 0; d < depth; d++ {
				segs = append(segs, randSeg())
			}
		}

		loc := Locator(segs)
		p, err := loc.Path()
		if err != nil {
			t.Fatalf("Path() on %v: %v", segs, err)
		}
		back, err := FromPath(p)
		if err != nil {
			t.Fatalf("FromPath(%q) (from %v): %v", p, segs, err)
		}
		if back.String() != loc.String() {
			t.Fatalf("round trip mismatch: %v -> %q -> %v", segs, p, back)
		}
	}
}

func TestPathFromPathRoundTripSkills(t *testing.T) {
	ids := []string{"signups-report", "commit-style", "a", "long-id-name"}
	for _, id := range ids {
		loc := Locator{"skills", id}
		p, err := loc.Path()
		if err != nil {
			t.Fatalf("Path(): %v", err)
		}
		if !strings.HasPrefix(p, ".agents/skills/para-") {
			t.Fatalf("Path() = %q, want .agents/skills/para- prefix", p)
		}
		back, err := FromPath(p)
		if err != nil {
			t.Fatalf("FromPath(%q): %v", p, err)
		}
		if back.String() != loc.String() {
			t.Fatalf("round trip mismatch: %v -> %q -> %v", loc, p, back)
		}
	}
}
