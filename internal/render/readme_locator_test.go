package render_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/truth"
)

// TestReadmeLocatorIsTheDottedAddress is P18.2's acceptance criterion:
// README.md's `locator:` key carries the external dotted-noun form (R1,
// R24), not the internal plural-bucket Locator string.
func TestReadmeLocatorIsTheDottedAddress(t *testing.T) {
	cases := []struct {
		name string
		in   render.In
		want string
	}{
		{
			name: "an entity",
			in: render.In{
				Locator: loc(t, "projects.acme-migration"),
				Kind:    kindmeta.KindProject,
				State:   truth.State{Name: "Acme migration"},
			},
			want: `locator: "project.acme-migration"`,
		},
		{
			name: "a key-result, whose chain drops both structural words",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives.q1.key-results.signups"),
				Kind:    kindmeta.KindKeyResult,
				State:   truth.State{Name: "Signups", Type: "ratio", Target: "2000/12000"},
			},
			want: `locator: "key-result.acme.q1.signups"`,
		},
		{
			name: "a container nested under a project",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Objectives"},
			},
			want: `locator: "container.acme.objectives"`,
		},
		{
			name: "a container nested under an objective, ending in key-results",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives.q1.key-results"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Key results"},
			},
			want: `locator: "container.acme.q1.key-results"`,
		},
		{
			name: "a resource, which nests like an area",
			in: render.In{
				Locator: loc(t, "resources.papers.kafka"),
				Kind:    kindmeta.KindResource,
				State:   truth.State{Name: "Kafka"},
			},
			want: `locator: "resource.papers.kafka"`,
		},
		{
			name: "a bucket, whose address is the bare noun",
			in: render.In{
				Locator: loc(t, "projects"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Projects"},
			},
			want: `locator: "project"`,
		},
		{
			name: "an archived nested area",
			in: render.In{
				Locator: loc(t, "archive.areas.health.training"),
				Kind:    kindmeta.KindArea,
				State:   truth.State{Name: "Training"},
			},
			want: `locator: "archive.area.health.training"`,
		},
		{
			// §1.6/R7: "the whole archive" has no noun at all — it is not one
			// of R2's seven addressable things — yet the archive root is a
			// real container with its own README (§1.1's tree diagram), so it
			// keeps its bucket word rather than an address that does not
			// exist.
			name: "the archive root itself, which has no noun",
			in: render.In{
				Locator: loc(t, "archive"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Archive"},
			},
			want: `locator: "archive"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := render.Readme.Render(c.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(string(got), c.want) {
				t.Errorf("Readme =\n%s\nwant it to contain %q", got, c.want)
			}
		})
	}
}

// TestReadmeNounKeyOnlyOnDisagreement is para-cov: a bucket's `kind` is
// always `container` (§1.1's furniture, not a thing in its own right), but
// its own address is the bare noun with no chain (R3) — "project", not
// "container" — so the two disagree, and R26 says that is exactly when a
// `noun` key is added beside them. Every other position, including a
// container nested under a project and the archive root itself (which has
// no address at all, R7), agrees or has nothing to disagree with, and gets
// no key.
func TestReadmeNounKeyOnlyOnDisagreement(t *testing.T) {
	cases := []struct {
		name    string
		in      render.In
		wantKey bool
		want    string
	}{
		{
			name: "a bucket disagrees and gets a noun key",
			in: render.In{
				Locator: loc(t, "projects"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Projects"},
			},
			wantKey: true,
			want:    `noun: "project"`,
		},
		{
			name: "an archived bucket mirror disagrees too",
			in: render.In{
				Locator: loc(t, "archive.areas"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Archived areas"},
			},
			wantKey: true,
			want:    `noun: "area"`,
		},
		{
			name: "a container nested under a project already agrees",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Objectives"},
			},
			wantKey: false,
		},
		{
			name: "the archive root has no address at all to disagree with",
			in: render.In{
				Locator: loc(t, "archive"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Archive"},
			},
			wantKey: false,
		},
		{
			name: "an ordinary entity never reaches the mismatch",
			in: render.In{
				Locator: loc(t, "projects.acme-migration"),
				Kind:    kindmeta.KindProject,
				State:   truth.State{Name: "Acme migration"},
			},
			wantKey: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := render.Readme.Render(c.in)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			has := strings.Contains(string(got), "noun:")
			if has != c.wantKey {
				t.Errorf("Readme =\n%s\nwant a noun key: %v", got, c.wantKey)
			}
			if c.wantKey && !strings.Contains(string(got), c.want) {
				t.Errorf("Readme =\n%s\nwant it to contain %q", got, c.want)
			}
		})
	}
}
