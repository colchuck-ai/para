package cli

import (
	"testing"

	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/locator"
)

func mustLocForRepairTest(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// TestDoctorOutputLocatorIsTheDottedAddress is R24's rule for `doctor
// --json`: a finding naming an entity carries the dotted address in its
// `locator` key, not the internal plural-bucket Locator string. A finding
// with no locator at all (an orphan, whose whole point is a directory the
// walk cannot derive one for) keeps the key absent.
func TestDoctorOutputLocatorIsTheDottedAddress(t *testing.T) {
	cases := []struct {
		name string
		f    doctor.Finding
		want string
	}{
		{
			name: "a finding naming an entity",
			f:    doctor.Finding{Kind: doctor.KindInvalid, Path: "projects/acme/.para/state.toml", Locator: mustLocForRepairTest(t, "projects.acme"), Detail: "x"},
			want: "project.acme",
		},
		{
			name: "a finding naming a key-result, both structural words dropped",
			f: doctor.Finding{
				Kind: doctor.KindJournal, Path: "x", Locator: mustLocForRepairTest(t,
					"projects.acme.objectives.q1.key-results.signups"),
				Detail: "x",
			},
			want: "key-result.acme.q1.signups",
		},
		{
			name: "an orphan has no locator to derive",
			f:    doctor.Finding{Kind: doctor.KindOrphan, Path: "projects/acme/notes/x", Detail: "x"},
			want: "",
		},
		{
			// A collision's whole point is that the locator derives no valid
			// kind — a reserved word sits in an id position — so it derives
			// no address either. The raw internal Locator is shown, not a
			// guessed noun: this is the fallback branch actually firing, not
			// dead defensive code.
			name: "a collision's locator derives no address, so the raw form is shown",
			f:    doctor.Finding{Kind: doctor.KindCollision, Path: "projects/skills", Locator: mustLocForRepairTest(t, "projects.skills"), Detail: "x"},
			want: "projects.skills",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := doctorOutputOf(doctor.Report{Findings: []doctor.Finding{c.f}})
			if got := out.Findings[0].Locator; got != c.want {
				t.Errorf("Locator = %q, want %q", got, c.want)
			}
		})
	}
}
