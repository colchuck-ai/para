package address

import (
	"errors"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// TestNounRoundTrip is R2's seven words: every noun's String output parses
// back to the same noun, in both directions.
func TestNounRoundTrip(t *testing.T) {
	cases := []struct {
		noun Noun
		word string
	}{
		{Project, "project"},
		{Area, "area"},
		{Resource, "resource"},
		{Objective, "objective"},
		{KeyResult, "key-result"},
		{Link, "link"},
		{Skill, "skill"},
		{Container, "container"},
	}
	for _, c := range cases {
		t.Run(c.word, func(t *testing.T) {
			if got := c.noun.String(); got != c.word {
				t.Fatalf("%v.String() = %q, want %q", c.noun, got, c.word)
			}
			got, err := ParseNoun(c.word)
			if err != nil {
				t.Fatalf("ParseNoun(%q): %v", c.word, err)
			}
			if got != c.noun {
				t.Fatalf("ParseNoun(%q) = %v, want %v", c.word, got, c.noun)
			}
		})
	}
}

// TestParseNounRefusals is the vocabulary's edge: anything not one of the
// seven words is refused, including the plural forms Locator uses
// internally and the empty string.
func TestParseNounRefusals(t *testing.T) {
	for _, s := range []string{
		"", "projects", "areas", "resources", "objectives", "key-results", "links",
		"skills", "Project", "PROJECT", "kr", "task", "unknown",
	} {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseNoun(s); err == nil {
				t.Fatalf("ParseNoun(%q) succeeded, want a refusal", s)
			} else {
				var perr *paraerr.Error
				if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
					t.Fatalf("ParseNoun(%q) error = %v, want KindValidation", s, err)
				}
			}
		})
	}
}

// TestAllNounsOrder pins R2's fixed order — the order help and completion
// output list the nouns in — and that it holds exactly the eight words, no
// more and no fewer.
func TestAllNounsOrder(t *testing.T) {
	want := []Noun{Project, Area, Resource, Objective, KeyResult, Link, Skill, Container}
	got := AllNouns()
	if len(got) != len(want) {
		t.Fatalf("AllNouns() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("AllNouns()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

// TestAllNounsIsAClone guards against a caller mutating the shared slice
// (kindmeta.AllKinds carries the same guarantee, and the two lists must not
// drift apart from each other for the same reason).
func TestAllNounsIsAClone(t *testing.T) {
	got := AllNouns()
	got[0] = Container
	if AllNouns()[0] != Project {
		t.Fatal("AllNouns() returned a slice callers can corrupt")
	}
}

// TestAllNounsIsDerivedFromKindmeta pins the derivation at the value level,
// not just the type level: the words a user may type and the words a
// state.toml's `kind` may hold are one vocabulary (§8.3, §30), so AllNouns
// must be kindmeta.AllStorableKinds() itself rather than a list rebuilt here
// — otherwise a kind kindmeta grows can be silently missing a noun.
//
// It still checks the composition rather than only the identity, because
// "storable" and "addressable plus container" agreeing is the property that
// makes one list serve both callers.
func TestAllNounsIsDerivedFromKindmeta(t *testing.T) {
	storable := kindmeta.AllStorableKinds()
	nouns := AllNouns()
	if len(nouns) != len(storable) {
		t.Fatalf("AllNouns() has %d nouns, want kindmeta.AllStorableKinds()'s %d", len(nouns), len(storable))
	}
	for i := range storable {
		if nouns[i] != storable[i] {
			t.Fatalf("AllNouns()[%d] = %v, want %v", i, nouns[i], storable[i])
		}
	}
	addressable := kindmeta.AllKinds()
	if len(nouns) != len(addressable)+1 || nouns[len(nouns)-1] != Container {
		t.Fatalf("AllNouns() = %v, want kindmeta.AllKinds() then Container", nouns)
	}
}

// TestParseNounErrorNamesTheVocabulary asserts the refusal message actually
// lists every legal noun — the one thing about ParseNoun's error that
// errors.As's Kind check in TestParseNounRefusals cannot see.
//
// The list is derived rather than written out. It used to be a hand-written
// seven, which had silently gone stale: `link` joined the vocabulary in
// para-6g7 and was never added here, so the test could not have noticed a
// noun going missing from the message it exists to check.
func TestParseNounErrorNamesTheVocabulary(t *testing.T) {
	_, err := ParseNoun("bogus")
	if err == nil {
		t.Fatal("ParseNoun(\"bogus\") succeeded, want a refusal")
	}
	for _, noun := range AllNouns() {
		word := noun.String()
		if !strings.Contains(err.Error(), word) {
			t.Errorf("ParseNoun error %q does not name noun %q", err.Error(), word)
		}
	}
}

// TestNounIsKindmetaKind pins the derivation the task requires: Noun is not
// a second enum declared beside kindmeta.Kind, it is that type, so an eighth
// kind kindmeta grows can never leave a nounless address behind.
func TestNounIsKindmetaKind(t *testing.T) {
	var n Noun = kindmeta.KindProject
	if n != Project {
		t.Fatalf("Noun is not kindmeta.Kind: assigning a kindmeta.Kind value gave %v, want %v", n, Project)
	}
}

func TestParseNounErrorNamesNounsNotKinds(t *testing.T) {
	// The match is delegated; the wording is not. A user who typed a bad first
	// token has no concept called "kind" in front of them.
	_, err := ParseNoun("banana")
	if err == nil {
		t.Fatal("ParseNoun(\"banana\") = nil error")
	}
	if !strings.Contains(err.Error(), "is not a noun") {
		t.Errorf("ParseNoun error = %q, want it to say \"is not a noun\"", err.Error())
	}
	if strings.Contains(err.Error(), "is not a kind") {
		t.Errorf("ParseNoun error = %q, leaked kindmeta's wording", err.Error())
	}
}
