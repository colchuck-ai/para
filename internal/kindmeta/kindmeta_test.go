package kindmeta

import (
	"errors"
	"slices"
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

		// links/ under any of the three link-capable parents (para-6g7).
		{"projects.acme-migration.links.jira-epic", KindLink, false},
		{"areas.health.links.blog", KindLink, false},
		{"areas.health.training.links.blog", KindLink, false},
		{"areas.health.training.deep.deeper.links.blog", KindLink, false},
		{"resources.templates.links.feed", KindLink, false},
		{"resources.templates.emails.links.feed", KindLink, false},

		// archive/{projects,areas,resources}/… — same kinds, dormant.
		{"archive.projects.acme-migration", KindProject, true},
		{"archive.projects.acme-migration.objectives.q1-growth", KindObjective, true},
		{"archive.projects.acme-migration.objectives.q1-growth.key-results.signups", KindKeyResult, true},
		{"archive.areas.health", KindArea, true},
		{"archive.areas.health.training", KindArea, true},
		{"archive.resources.templates", KindResource, true},
		{"archive.projects.acme-migration.links.jira-epic", KindLink, true},
		{"archive.areas.health.training.links.blog", KindLink, true},
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

		// links/ misplacement (para-6g7): a link is a leaf, and "links" must
		// be the second-to-last segment, never buried or trailing alone.
		"projects.acme.links",                    // links with no id names no entity
		"projects.acme.links.jira-epic.extra",    // a link is a leaf
		"projects.acme.objectives.q1.links.blog", // links never follows an objective
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
		"projects.links",
		"projects.acme.links.links",
		"areas.health.links.links",
		"areas.links.blog",
		"resources.links.feed",
		"areas.health.links",
		"areas.health.links.blog.extra",
		"resources.templates.links",
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
		KindLink:      "link",
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

func TestParseKindAcceptsEveryKindWord(t *testing.T) {
	// What this actually catches is String() not being injective over the
	// vocabulary. ParseKind iterates the same storableKinds this loop does, so
	// "in the list but unparseable" is unrepresentable — but the word table
	// and the vocabulary list are two separate declarations, and if two kinds
	// ever claimed one word, the second would parse back to the first.
	for _, want := range AllStorableKinds() {
		got, err := ParseKind(want.String())
		if err != nil {
			t.Errorf("ParseKind(%q): %v", want.String(), err)
			continue
		}
		if got != want {
			t.Errorf("ParseKind(%q) = %v, want %v", want.String(), got, want)
		}
	}
}

func TestAllStorableKindsCarriesContainer(t *testing.T) {
	// A container has its own state.toml (§8.2) and so its own stored kind,
	// which AllKinds() deliberately excludes because no locator names one.
	if !slices.Contains(AllStorableKinds(), KindContainer) {
		t.Error("AllStorableKinds() omits container, which has a state.toml and so a stored kind")
	}
	if slices.Contains(AllKinds(), KindContainer) {
		t.Error("AllKinds() now carries container; AllStorableKinds is no longer the wider list")
	}
}

func TestAllStorableKindsIsNotAliased(t *testing.T) {
	// ParseKind, KindWords and address.AllNouns all read storableKinds;
	// handing any caller the backing array would let it rewrite the
	// vocabulary process-wide.
	first := AllStorableKinds()
	first[0] = KindUnknown
	if AllStorableKinds()[0] == KindUnknown {
		t.Error("AllStorableKinds() returns an aliased slice; a caller can mutate the vocabulary")
	}
}

func TestParseKindRejectsWhatIsNotAKindWord(t *testing.T) {
	for _, s := range []string{
		"",            // absent, which is a different finding from invalid
		"banana",      // not a word at all
		"unknown",     // KindUnknown.String(), a sentinel and not a kind
		"Area",        // the charset is lowercase (§1.4)
		"areas",       // a bucket word, never a kind
		"key-result ", // trailing space
	} {
		if got, err := ParseKind(s); err == nil {
			t.Errorf("ParseKind(%q) = %v, want an error", s, got)
		}
	}
}

func TestParseKindErrorIsValidation(t *testing.T) {
	// doctor tags an unrecognised stored value `invalid` (§10), which is a
	// validation failure rather than a collision.
	_, err := ParseKind("banana")
	var perr *paraerr.Error
	if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
		t.Errorf("ParseKind error = %v, want KindValidation", err)
	}
}

func TestEveryKindWordIsReserved(t *testing.T) {
	// §1.4 reserves the noun words as ids, and the reason is load-bearing:
	// without it an entity legitimately named `project` makes `para list
	// project project` unparseable, since list tells a kind filter from a
	// scope by lookahead.
	//
	// locator.ReservedWords has to hand-list them — kindmeta imports locator,
	// so the derivation cannot run the other way — which makes it the one
	// genuinely unavoidable second copy of the vocabulary. locator's own test
	// checks its eight noun words by hand, which catches a word being
	// *removed* — but not a kind being *added*, since the hand-written
	// expectation would not grow either. A ninth kind whose word nobody
	// reserved would leave every test in that package passing while an id
	// could shadow its own noun. This is the check that grows on its own, and
	// it lives here because this is the side that can see both lists.
	for _, k := range AllStorableKinds() {
		if !locator.IsReserved(k.String()) {
			t.Errorf("kind word %q is not in locator.ReservedWords; an id could shadow it", k.String())
		}
	}
}
