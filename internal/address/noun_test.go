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
		"", "projects", "areas", "resources", "objectives", "key-results",
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
// output list the nouns in — and that it holds exactly the seven words, no
// more and no fewer.
func TestAllNounsOrder(t *testing.T) {
	want := []Noun{Project, Area, Resource, Objective, KeyResult, Skill, Container}
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

// TestAllNounsIsDerivedFromKindmeta pins the derivation the task requires at
// the value level, not just the type level: AllNouns must be exactly
// kindmeta.AllKinds() plus Container, built from that call rather than
// listed again by hand, so a kind kindmeta.AllKinds() grows to include can
// never be silently missing here.
func TestAllNounsIsDerivedFromKindmeta(t *testing.T) {
	kinds := kindmeta.AllKinds()
	nouns := AllNouns()
	if len(nouns) != len(kinds)+1 {
		t.Fatalf("AllNouns() has %d nouns, want kindmeta.AllKinds()'s %d plus Container", len(nouns), len(kinds))
	}
	for i, k := range kinds {
		if nouns[i] != k {
			t.Fatalf("AllNouns()[%d] = %v, want kindmeta.AllKinds()[%d] = %v", i, nouns[i], i, k)
		}
	}
	if nouns[len(nouns)-1] != Container {
		t.Fatalf("AllNouns()'s last entry = %v, want Container", nouns[len(nouns)-1])
	}
}

// TestParseNounErrorNamesTheVocabulary asserts the refusal message actually
// lists the seven legal nouns — the one thing about ParseNoun's error that
// errors.As's Kind check in TestParseNounRefusals cannot see.
func TestParseNounErrorNamesTheVocabulary(t *testing.T) {
	_, err := ParseNoun("bogus")
	if err == nil {
		t.Fatal("ParseNoun(\"bogus\") succeeded, want a refusal")
	}
	for _, word := range []string{"project", "area", "resource", "objective", "key-result", "skill", "container"} {
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
