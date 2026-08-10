package address

import (
	"errors"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// TestParseLegalShapes is R4's arity table: every noun's legal chain
// lengths, both through Parse's two-token CLI form and ParseDotted's
// one-token serialized form, agreeing with each other and with String.
func TestParseLegalShapes(t *testing.T) {
	cases := []struct {
		name     string
		noun     Noun
		chain    string // "" means no chain (the bucket, R17)
		dotted   string // the expected serialized form
		wantAddr Address
	}{
		{"project bucket", Project, "", "project", Address{Noun: Project}},
		{"project entity", Project, "acme", "project.acme", Address{Noun: Project, Chain: []string{"acme"}}},
		{"skill bucket", Skill, "", "skill", Address{Noun: Skill}},
		{"skill entity", Skill, "signups-report", "skill.signups-report", Address{Noun: Skill, Chain: []string{"signups-report"}}},
		{"area bucket", Area, "", "area", Address{Noun: Area}},
		{"area one segment", Area, "health", "area.health", Address{Noun: Area, Chain: []string{"health"}}},
		{
			"area nested", Area, "health.training", "area.health.training",
			Address{Noun: Area, Chain: []string{"health", "training"}},
		},
		{
			"area deeply nested", Area, "health.training.plans", "area.health.training.plans",
			Address{Noun: Area, Chain: []string{"health", "training", "plans"}},
		},
		{"resource bucket", Resource, "", "resource", Address{Noun: Resource}},
		{
			"resource nested", Resource, "papers.kafka", "resource.papers.kafka",
			Address{Noun: Resource, Chain: []string{"papers", "kafka"}},
		},
		{
			"objective", Objective, "acme.q1-growth", "objective.acme.q1-growth",
			Address{Noun: Objective, Chain: []string{"acme", "q1-growth"}},
		},
		{
			"key-result", KeyResult, "acme.q1-growth.signups", "key-result.acme.q1-growth.signups",
			Address{Noun: KeyResult, Chain: []string{"acme", "q1-growth", "signups"}},
		},
		{
			"container under a project (objectives)", Container, "acme.objectives", "container.acme.objectives",
			Address{Noun: Container, Chain: []string{"acme", "objectives"}},
		},
		{
			"container under an objective (key-results)", Container, "acme.q1-growth.key-results",
			"container.acme.q1-growth.key-results",
			Address{Noun: Container, Chain: []string{"acme", "q1-growth", "key-results"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Parse(c.noun.String(), c.chain)
			if err != nil {
				t.Fatalf("Parse(%q, %q): %v", c.noun.String(), c.chain, err)
			}
			assertAddrEqual(t, got, c.wantAddr)
			if got.String() != c.dotted {
				t.Errorf("Parse(...).String() = %q, want %q", got.String(), c.dotted)
			}

			dotted, err := ParseDotted(c.dotted)
			if err != nil {
				t.Fatalf("ParseDotted(%q): %v", c.dotted, err)
			}
			assertAddrEqual(t, dotted, c.wantAddr)
		})
	}
}

// TestArityRefusals is R4's negative space: a chain of the wrong length or
// shape for its noun is refused as KindValidation, naming the noun and the
// arity it wants.
func TestArityRefusals(t *testing.T) {
	cases := []struct {
		name  string
		noun  Noun
		chain string
	}{
		{"project two segments", Project, "acme.extra"},
		{"skill two segments", Skill, "a.b"},
		{"objective no chain", Objective, ""},
		{"objective one segment", Objective, "acme"},
		{"objective three segments", Objective, "acme.q1.extra"},
		{"key-result no chain", KeyResult, ""},
		{"key-result two segments", KeyResult, "acme.q1"},
		{"key-result four segments", KeyResult, "acme.q1.signups.extra"},
		{"container no chain", Container, ""},
		{"container one segment", Container, "acme"},
		{"container two segments wrong tail", Container, "acme.key-results"},
		{"container three segments wrong tail", Container, "acme.q1.objectives"},
		{"container four segments", Container, "acme.q1.extra.key-results"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.noun.String(), c.chain)
			assertKind(t, err, paraerr.KindValidation)
			if err != nil && !strings.Contains(err.Error(), c.noun.String()) {
				t.Errorf("error %q does not name the noun %q", err.Error(), c.noun.String())
			}
		})
	}
}

// TestReservedWordRefusals is §1.4's other refusal: a reserved word used as
// an id is KindConflict, distinct from an arity mismatch.
func TestReservedWordRefusals(t *testing.T) {
	cases := []struct {
		name  string
		noun  Noun
		chain string
	}{
		{"project id is a reserved word", Project, "projects"},
		{"area first segment reserved", Area, "archive"},
		{"area nested segment reserved", Area, "health.skills"},
		{"objective project id reserved", Objective, "skills.q1"},
		{"objective own id reserved", Objective, "acme.objectives"},
		{"key-result own id reserved", KeyResult, "acme.q1.key-results"},
		{"container project id reserved", Container, "logs.objectives"},
		// A reserved id combined with a wrong tail must still report the
		// reserved word, not the arity: checking the tail before the id
		// positions would silently swallow the conflict behind a generic
		// arity refusal.
		{"container reserved id and wrong tail (arity 2)", Container, "archive.wrongtail"},
		{"container reserved id and wrong tail (arity 3)", Container, "acme.skills.wrongtail"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.noun.String(), c.chain)
			assertKind(t, err, paraerr.KindConflict)
		})
	}
}

// TestIllegalCharsetRefusals is §1.4's charset rule applied to id
// positions: an id must match [a-z0-9-]+.
func TestIllegalCharsetRefusals(t *testing.T) {
	cases := []struct {
		name  string
		noun  Noun
		chain string
	}{
		{"uppercase", Project, "Acme"},
		{"underscore", Area, "health_training"},
		{"empty segment", Area, "health."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.noun.String(), c.chain)
			assertKind(t, err, paraerr.KindValidation)
		})
	}
}

// TestSkillCannotBeArchived is §1.6's rule that skills are not archivable,
// asserted at the address layer so a hand-built Address is refused exactly
// like a parsed one.
func TestSkillCannotBeArchived(t *testing.T) {
	a := Address{Noun: Skill, Chain: []string{"signups-report"}, Archived: true}
	assertKind(t, a.Validate(), paraerr.KindValidation)

	_, err := ParseDotted("archive.skill.signups-report")
	assertKind(t, err, paraerr.KindValidation)
}

// TestArchivedRoundTrip is R10's stored form: the archive qualifier
// prepends one segment and nothing else changes.
func TestArchivedRoundTrip(t *testing.T) {
	cases := []struct {
		dotted string
		want   Address
	}{
		{"archive.project.acme", Address{Noun: Project, Chain: []string{"acme"}, Archived: true}},
		{
			"archive.area.health.training",
			Address{Noun: Area, Chain: []string{"health", "training"}, Archived: true},
		},
		{
			"archive.key-result.acme.q1-growth.signups",
			Address{Noun: KeyResult, Chain: []string{"acme", "q1-growth", "signups"}, Archived: true},
		},
		{"archive.project", Address{Noun: Project, Archived: true}},
	}
	for _, c := range cases {
		t.Run(c.dotted, func(t *testing.T) {
			got, err := ParseDotted(c.dotted)
			if err != nil {
				t.Fatalf("ParseDotted(%q): %v", c.dotted, err)
			}
			assertAddrEqual(t, got, c.want)
			if got.String() != c.dotted {
				t.Errorf("String() = %q, want %q", got.String(), c.dotted)
			}
		})
	}
}

// TestParseDottedMalformed covers the one-token form's own structural
// refusals: empty input and a bare "archive" with nothing after it.
func TestParseDottedMalformed(t *testing.T) {
	for _, s := range []string{"", "archive", "."} {
		t.Run(s, func(t *testing.T) {
			if _, err := ParseDotted(s); err == nil {
				t.Fatalf("ParseDotted(%q) succeeded, want a refusal", s)
			}
		})
	}
}

// TestUnknownNounRefused pins Validate's default case: a Noun value outside
// the seven R2 defines (here, the zero value) is refused rather than
// silently treated as some noun's arity rule.
func TestUnknownNounRefused(t *testing.T) {
	var zero Noun
	a := Address{Noun: zero, Chain: []string{"x"}}
	assertKind(t, a.Validate(), paraerr.KindValidation)
}

func assertAddrEqual(t *testing.T, got, want Address) {
	t.Helper()
	if got.Noun != want.Noun || got.Archived != want.Archived || len(got.Chain) != len(want.Chain) {
		t.Fatalf("Address = %+v, want %+v", got, want)
	}
	for i := range got.Chain {
		if got.Chain[i] != want.Chain[i] {
			t.Fatalf("Address = %+v, want %+v", got, want)
		}
	}
}

func assertKind(t *testing.T, err error, want paraerr.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("got no error, want %v", want)
	}
	var perr *paraerr.Error
	if !errors.As(err, &perr) {
		t.Fatalf("error %v is not a *paraerr.Error", err)
	}
	if perr.Kind != want {
		t.Fatalf("error kind = %v, want %v (%v)", perr.Kind, want, err)
	}
}
