package address

import (
	"math/rand/v2"
	"testing"

	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// TestToLocator is R3's worked path-derivation table, read as a Locator
// rather than a filesystem path (locator.Locator.Path takes it the rest of
// the way, unaffected by this package).
func TestToLocator(t *testing.T) {
	cases := []struct {
		name string
		addr Address
		want []string
	}{
		{"project", Address{Noun: Project, Chain: []string{"acme"}}, []string{"projects", "acme"}},
		{"project bucket", Address{Noun: Project}, []string{"projects"}},
		{
			"objective",
			Address{Noun: Objective, Chain: []string{"acme", "q1-growth"}},
			[]string{"projects", "acme", "objectives", "q1-growth"},
		},
		{
			"key-result",
			Address{Noun: KeyResult, Chain: []string{"acme", "q1-growth", "signups"}},
			[]string{"projects", "acme", "objectives", "q1-growth", "key-results", "signups"},
		},
		{"area bucket", Address{Noun: Area}, []string{"areas"}},
		{"area", Address{Noun: Area, Chain: []string{"health"}}, []string{"areas", "health"}},
		{
			"area nested",
			Address{Noun: Area, Chain: []string{"health", "training"}},
			[]string{"areas", "health", "training"},
		},
		{"resource bucket", Address{Noun: Resource}, []string{"resources"}},
		{
			"resource",
			Address{Noun: Resource, Chain: []string{"papers", "kafka"}},
			[]string{"resources", "papers", "kafka"},
		},
		{"skill bucket", Address{Noun: Skill}, []string{"skills"}},
		{"skill", Address{Noun: Skill, Chain: []string{"signups-report"}}, []string{"skills", "signups-report"}},
		{
			"container under a project",
			Address{Noun: Container, Chain: []string{"acme", "objectives"}},
			[]string{"projects", "acme", "objectives"},
		},
		{
			"container under an objective",
			Address{Noun: Container, Chain: []string{"acme", "q1-growth", "key-results"}},
			[]string{"projects", "acme", "objectives", "q1-growth", "key-results"},
		},
		{
			"archived project",
			Address{Noun: Project, Chain: []string{"acme"}, Archived: true},
			[]string{"archive", "projects", "acme"},
		},
		{
			"archived area bucket",
			Address{Noun: Area, Archived: true},
			[]string{"archive", "areas"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.addr.ToLocator()
			if err != nil {
				t.Fatalf("ToLocator(%+v): %v", c.addr, err)
			}
			assertLocEqual(t, got, locator.Locator(c.want))
		})
	}
}

// TestToLocatorRefusesInvalidAddress pins that ToLocator validates rather
// than trusting its argument — a struct literal skipping Parse must be
// refused the same way Parse would refuse it.
func TestToLocatorRefusesInvalidAddress(t *testing.T) {
	_, err := Address{Noun: Objective, Chain: []string{"acme"}}.ToLocator()
	assertKind(t, err, paraerr.KindValidation)
}

// TestFromLocator covers FromLocator's own structural refusals: shapes no
// Address can name, distinct from an id that fails checkID.
func TestFromLocator(t *testing.T) {
	cases := []struct {
		name    string
		loc     locator.Locator
		want    Address
		wantErr bool
	}{
		{"project bucket", locator.Locator{"projects"}, Address{Noun: Project}, false},
		{"project", locator.Locator{"projects", "acme"}, Address{Noun: Project, Chain: []string{"acme"}}, false},
		{
			"container arity 2",
			locator.Locator{"projects", "acme", "objectives"},
			Address{Noun: Container, Chain: []string{"acme", "objectives"}},
			false,
		},
		{
			"objective",
			locator.Locator{"projects", "acme", "objectives", "q1"},
			Address{Noun: Objective, Chain: []string{"acme", "q1"}},
			false,
		},
		{
			"container arity 3",
			locator.Locator{"projects", "acme", "objectives", "q1", "key-results"},
			Address{Noun: Container, Chain: []string{"acme", "q1", "key-results"}},
			false,
		},
		{
			"key-result",
			locator.Locator{"projects", "acme", "objectives", "q1", "key-results", "signups"},
			Address{Noun: KeyResult, Chain: []string{"acme", "q1", "signups"}},
			false,
		},
		{"skill bucket", locator.Locator{"skills"}, Address{Noun: Skill}, false},
		{"skill", locator.Locator{"skills", "report"}, Address{Noun: Skill, Chain: []string{"report"}}, false},
		{
			"archived project",
			locator.Locator{"archive", "projects", "acme"},
			Address{Noun: Project, Chain: []string{"acme"}, Archived: true},
			false,
		},
		{
			"archived area bucket",
			locator.Locator{"archive", "areas"},
			Address{Noun: Area, Archived: true},
			false,
		},

		{"empty locator", locator.Locator{}, Address{}, true},
		{"unrecognized bucket", locator.Locator{"junk"}, Address{}, true},
		{"archive alone", locator.Locator{"archive"}, Address{}, true},
		{"archived skill refused", locator.Locator{"archive", "skills", "report"}, Address{}, true},
		{"project cannot nest", locator.Locator{"projects", "acme", "junk"}, Address{}, true},
		{
			"objective cannot nest",
			locator.Locator{"projects", "acme", "objectives", "q1", "junk"},
			Address{},
			true,
		},
		{
			"key-result is a leaf",
			locator.Locator{"projects", "acme", "objectives", "q1", "key-results", "signups", "extra"},
			Address{},
			true,
		},
		{"skills nested illegal", locator.Locator{"skills", "a", "b"}, Address{}, true},
		{"reserved project id", locator.Locator{"projects", "skills"}, Address{}, true},
		{"reserved area segment", locator.Locator{"areas", "health", "logs"}, Address{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := FromLocator(c.loc)
			if c.wantErr {
				if err == nil {
					t.Fatalf("FromLocator(%v) = %+v, want a refusal", c.loc, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("FromLocator(%v): %v", c.loc, err)
			}
			assertAddrEqual(t, got, c.want)
		})
	}
}

// assertLocEqual compares two Locators by segment, the way locator_test.go
// itself does (Locator has no Equal method).
func assertLocEqual(t *testing.T, got, want locator.Locator) {
	t.Helper()
	if got.String() != want.String() {
		t.Fatalf("Locator = %v, want %v", got, want)
	}
}

// TestToLocatorFromLocatorRoundTrip is the property task 4 exists for: the
// conversion is total and round-trips in both directions for every noun and
// arity, including the skill exception and the archive qualifier — the same
// obligation locator_test.go's TestPathFromPathRoundTrip carries for
// locator <-> path.
//
// It checks both directions in the same pass: a random legal Address
// converts to a Locator and back to the same Address (Address -> Locator ->
// Address), and that same Locator converts back to itself once round-tripped
// through the external form (Locator -> Address -> Locator) — the guarantee
// a caller storing a Locator and later re-deriving it from a re-parsed
// Address depends on.
func TestToLocatorFromLocatorRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	ids := []string{"acme", "health", "training", "q1-growth", "signups", "papers", "kafka", "x", "y-z", "plans"}
	randID := func() string { return ids[rng.IntN(len(ids))] }

	for i := 0; i < 500; i++ {
		addr := randomAddress(rng, randID)

		loc, err := addr.ToLocator()
		if err != nil {
			t.Fatalf("ToLocator(%+v): %v", addr, err)
		}
		back, err := FromLocator(loc)
		if err != nil {
			t.Fatalf("FromLocator(%q) (from %+v): %v", loc.String(), addr, err)
		}
		if back.String() != addr.String() {
			t.Fatalf("round trip mismatch: %+v -> %q -> %+v", addr, loc.String(), back)
		}

		loc2, err := back.ToLocator()
		if err != nil {
			t.Fatalf("ToLocator(FromLocator(%q)): %v", loc.String(), err)
		}
		if loc2.String() != loc.String() {
			t.Fatalf("locator round trip mismatch: %q -> %+v -> %q", loc.String(), back, loc2.String())
		}
	}
}

// randomAddress generates a random legal Address, drawn across every noun,
// every arity that noun allows (including the bucket where R3 gives it
// one), and the archive qualifier where R7 allows it.
func randomAddress(rng *rand.Rand, randID func() string) Address {
	noun := allNouns[rng.IntN(len(allNouns))]
	archived := noun != Skill && rng.IntN(4) == 0

	switch noun {
	case Project, Skill:
		if rng.IntN(3) == 0 {
			return Address{Noun: noun, Archived: archived}
		}
		return Address{Noun: noun, Chain: []string{randID()}, Archived: archived}
	case Area, Resource:
		if rng.IntN(4) == 0 {
			return Address{Noun: noun, Archived: archived}
		}
		depth := 1 + rng.IntN(3)
		chain := make([]string, depth)
		for i := range chain {
			chain[i] = randID()
		}
		return Address{Noun: noun, Chain: chain, Archived: archived}
	case Objective:
		return Address{Noun: noun, Chain: []string{randID(), randID()}, Archived: archived}
	case KeyResult:
		return Address{Noun: noun, Chain: []string{randID(), randID(), randID()}, Archived: archived}
	case Link:
		parent, ids := randomLinkParent(rng, randID)
		return Address{Noun: noun, Chain: append(append([]string{parent}, ids...), randID()), Archived: archived}
	case Container:
		switch rng.IntN(3) {
		case 0:
			return Address{Noun: noun, Chain: []string{randID(), "objectives"}, Archived: archived}
		case 1:
			return Address{Noun: noun, Chain: []string{randID(), randID(), "key-results"}, Archived: archived}
		default:
			parent, ids := randomLinkParent(rng, randID)
			return Address{Noun: noun, Chain: append(append([]string{parent}, ids...), "links"), Archived: archived}
		}
	default:
		panic("randomAddress: unreachable noun")
	}
}

// randomLinkParent generates a random legal parent portion for a link (or a
// links container): the selector word, and the id-chain that noun's own
// arity allows — exactly one id for "project", one or more for "area"/
// "resource".
func randomLinkParent(rng *rand.Rand, randID func() string) (string, []string) {
	switch rng.IntN(3) {
	case 0:
		return Project.String(), []string{randID()}
	case 1:
		depth := 1 + rng.IntN(3)
		ids := make([]string, depth)
		for i := range ids {
			ids[i] = randID()
		}
		return Area.String(), ids
	default:
		depth := 1 + rng.IntN(3)
		ids := make([]string, depth)
		for i := range ids {
			ids[i] = randID()
		}
		return Resource.String(), ids
	}
}
