package render_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/testutil"
	"github.com/colchuck-ai/para/internal/truth"
)

func TestHasAgentsIsExactlyTheEightPlaces(t *testing.T) {
	// §6: the root, the four buckets, and archive/{projects,areas,resources}.
	// Nowhere else — not on entities, not on objectives/, not on key-results/.
	for _, in := range []string{
		"", "projects", "areas", "resources",
		"archive", "archive.projects", "archive.areas", "archive.resources",
	} {
		var l locator.Locator
		if in != "" {
			l = loc(t, in)
		}
		if !render.HasAgents(l) {
			t.Errorf("HasAgents(%q) = false, want true", in)
		}
	}
	for _, in := range []string{
		"projects.acme", "projects.acme.objectives", "areas.health",
		"archive.projects.old", "skills.commit-style", "resources.notes",
	} {
		if render.HasAgents(loc(t, in)) {
			t.Errorf("HasAgents(%q) = true, want false", in)
		}
	}
}

func TestArtifactsPerShape(t *testing.T) {
	measured := []journal.Event{journal.NewMeasurement(ts(t, "2026-03-05T09:00:00"), "880/11000", "")}

	cases := []struct {
		name string
		in   render.In
		want []string
	}{
		{
			name: "root",
			in: render.In{
				Kind:   kindmeta.KindContainer,
				Tree:   truth.Tree{Schema: 1, Name: "brain", Created: "2026-01-01"},
				Config: render.DefaultConfig(),
			},
			want: []string{"README.md", "AGENTS.md", "ACTIVITY.md", ".gitattributes"},
		},
		{
			name: "root with the Claude surface on",
			in: render.In{
				Kind:   kindmeta.KindContainer,
				Tree:   truth.Tree{Schema: 1, Name: "brain", Created: "2026-01-01"},
				Config: render.Config{EmitClaude: true, EmitGitattributes: true},
			},
			want: []string{"README.md", "AGENTS.md", "ACTIVITY.md", "CLAUDE.md", ".gitattributes"},
		},
		{
			name: "root with .gitattributes off",
			in: render.In{
				Kind: kindmeta.KindContainer,
				Tree: truth.Tree{Schema: 1, Name: "brain", Created: "2026-01-01"},
			},
			want: []string{"README.md", "AGENTS.md", "ACTIVITY.md"},
		},
		{
			name: "a bucket gets AGENTS.md",
			in: render.In{
				Locator: loc(t, "projects"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Projects", Created: "2026-01-01"},
			},
			want: []string{"projects/README.md", "projects/AGENTS.md", "projects/ACTIVITY.md"},
		},
		{
			name: "an inner container does not",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives"),
				Kind:    kindmeta.KindContainer,
				State:   truth.State{Name: "Objectives", Created: "2026-01-01"},
			},
			want: []string{"projects/acme/objectives/README.md", "projects/acme/objectives/ACTIVITY.md"},
		},
		{
			name: "an entity",
			in: render.In{
				Locator: loc(t, "areas.health"),
				Kind:    kindmeta.KindArea,
				State:   truth.State{Name: "Health", Description: "Staying in one piece.", Created: "2026-01-01"},
			},
			want: []string{"areas/health/README.md", "areas/health/ACTIVITY.md"},
		},
		{
			name: "a key-result with no readings has no MEASUREMENTS.csv",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives.q1.key-results.signups"),
				Kind:    kindmeta.KindKeyResult,
				State:   truth.State{Name: "Signups", Type: "ratio", Target: "2000/12000", Created: "2026-01-01"},
			},
			want: []string{
				"projects/acme/objectives/q1/key-results/signups/README.md",
				"projects/acme/objectives/q1/key-results/signups/ACTIVITY.md",
			},
		},
		{
			name: "a key-result with readings does",
			in: render.In{
				Locator: loc(t, "projects.acme.objectives.q1.key-results.signups"),
				Kind:    kindmeta.KindKeyResult,
				State:   truth.State{Name: "Signups", Type: "ratio", Target: "2000/12000", Created: "2026-01-01"},
				Events:  measured,
			},
			want: []string{
				"projects/acme/objectives/q1/key-results/signups/README.md",
				"projects/acme/objectives/q1/key-results/signups/ACTIVITY.md",
				"projects/acme/objectives/q1/key-results/signups/MEASUREMENTS.csv",
			},
		},
		{
			name: "a skill owns SKILL.md and its rule, and nothing else",
			in: render.In{
				Locator: loc(t, "skills.signups-report"),
				Kind:    kindmeta.KindSkill,
				State:   truth.State{Name: "Signups report", Description: "when asked for the number", Created: "2026-01-01"},
			},
			want: []string{
				".agents/skills/para-signups-report/SKILL.md",
				".agents/rules/para-signups-report.md",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := render.Artifacts(tc.in)
			if err != nil {
				t.Fatalf("Artifacts: %v", err)
			}
			var paths []string
			for _, a := range got {
				paths = append(paths, a.Path)
			}
			if strings.Join(paths, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Artifacts paths =\n  %v\nwant\n  %v", paths, tc.want)
			}
		})
	}
}

func TestArtifactPathsAreSlashSeparatedAndRootRelative(t *testing.T) {
	// Every Artifact.Path is joined onto an OS path by the caller, so it must
	// carry no OS separators of its own.
	got, err := render.Artifacts(render.In{
		Locator: loc(t, "areas.health.training"),
		Kind:    kindmeta.KindArea,
		State:   truth.State{Name: "Training", Created: "2026-01-01"},
	})
	if err != nil {
		t.Fatalf("Artifacts: %v", err)
	}
	for _, a := range got {
		if filepath.IsAbs(a.Path) || strings.Contains(a.Path, `\`) {
			t.Errorf("Artifact path %q is not a slash-separated relative path", a.Path)
		}
	}
}

// TestEveryRendererIsByteStableAcrossCalls pins §0.2's first consequence: no
// map iteration in output, so two runs of every writer produce identical bytes.
// It is a cheap test and the one that would catch a renderer growing a
// range-over-map.
func TestEveryRendererIsByteStableAcrossCalls(t *testing.T) {
	for _, in := range everyShape(t) {
		t.Run(shapeName(in), func(t *testing.T) {
			first, err := render.Artifacts(in)
			if err != nil {
				t.Fatalf("Artifacts: %v", err)
			}
			second, err := render.Artifacts(in)
			if err != nil {
				t.Fatalf("Artifacts: %v", err)
			}
			if len(first) != len(second) {
				t.Fatalf("artifact count differs: %d then %d", len(first), len(second))
			}
			for i := range first {
				if first[i].Path != second[i].Path {
					t.Errorf("artifact %d path differs: %q then %q", i, first[i].Path, second[i].Path)
				}
				if string(first[i].Bytes) != string(second[i].Bytes) {
					t.Errorf("%s differs between two renders:\n%s\nvs\n%s", first[i].Path, first[i].Bytes, second[i].Bytes)
				}
			}
		})
	}
}

// TestGoldenArtifacts reviews every generated file as a diff (implementation
// plan §0.1's renderer-golden altitude). Run `go test -update` to rewrite them.
func TestGoldenArtifacts(t *testing.T) {
	for _, in := range everyShape(t) {
		t.Run(shapeName(in), func(t *testing.T) {
			artifacts, err := render.Artifacts(in)
			if err != nil {
				t.Fatalf("Artifacts: %v", err)
			}
			for _, a := range artifacts {
				name := strings.ReplaceAll(strings.TrimPrefix(a.Path, "."), "/", "_")
				golden := filepath.Join("testdata", shapeName(in), name)
				testutil.Golden(t, golden, a.Bytes)
			}
		})
	}
}

// everyShape is one In per row of §2.2's table of generated files, reused by
// the byte-stability and golden tests so both cover the same ground.
func everyShape(t *testing.T) []render.In {
	t.Helper()
	return []render.In{
		{
			Kind: kindmeta.KindContainer,
			Tree: truth.Tree{
				Schema: 1, ParaVersion: "0.4.0",
				Name: "max's brain", Description: "Everything I am carrying.",
				Created: "2026-01-01T08:00:00-08:00",
			},
			Events: []journal.Event{
				journal.NewChild(ts(t, "2026-01-01T08:00:01"), journal.ChildOpAdded, "projects", "", "", ""),
				journal.NewChild(ts(t, "2026-01-01T08:00:02"), journal.ChildOpAdded, "areas", "", "", ""),
			},
			Config: render.Config{EmitClaude: true, EmitClaudeSkills: render.MirrorSymlink, EmitGitattributes: true},
			Rules:  []string{"para-signups-report.md", "para-commit-style.md"},
		},
		{
			Locator: loc(t, "projects"),
			Kind:    kindmeta.KindContainer,
			State: truth.State{
				Name: "Projects", Description: "Work with a finish line.",
				Created: "2026-01-01T08:00:01-08:00",
			},
			Events: []journal.Event{
				journal.NewChild(ts(t, "2026-01-01T08:15:00"), journal.ChildOpAdded, "acme-migration", "", "", ""),
			},
			Config: render.Config{EmitClaude: true, EmitClaudeSkills: render.MirrorSymlink},
			Rules:  []string{"para-signups-report.md"},
		},
		{
			Locator: loc(t, "archive.areas"),
			Kind:    kindmeta.KindContainer,
			State: truth.State{
				Name: "Areas", Description: "Responsibilities you no longer hold.",
				Created: "2026-01-01T08:00:03-08:00",
			},
		},
		{
			Locator: loc(t, "projects.acme-migration"),
			Kind:    kindmeta.KindProject,
			State: truth.State{
				Name:        "Acme migration",
				Description: "Rebuild the consumer so it stops falling over under replay load.",
				Status:      "in-progress",
				Priority:    "high",
				Due:         "2026-09-30",
				Tags:        []string{"kafka", "consumer"},
				Created:     "2026-01-01T08:15:00-08:00",
			},
			Events: []journal.Event{
				journal.NewChange(ts(t, "2026-01-02T09:00:00"), "status", "planned", "in-progress", "kickoff done"),
				journal.NewNote(ts(t, "2026-01-04T17:40:00"), "waiting on the ingest team"),
			},
		},
		{
			Locator: loc(t, "projects.acme-migration.objectives.q1-growth.key-results.signups"),
			Kind:    kindmeta.KindKeyResult,
			State: truth.State{
				Name:    "Weekly signups",
				Due:     "2026-09-30",
				Created: "2026-01-01T08:15:00-08:00",
				Type:    "ratio",
				Start:   "480/9000",
				Target:  "2000/12000",
			},
			Events: []journal.Event{
				journal.NewMeasurement(ts(t, "2026-01-03T09:02:11"), "880/11000", ""),
				journal.NewMeasurement(ts(t, "2026-01-17T09:10:04"), "1320/12400", "denominator grew after the launch"),
			},
		},
		{
			Locator: loc(t, "skills.signups-report"),
			Kind:    kindmeta.KindSkill,
			State: truth.State{
				Name:        "Signups report",
				Description: "when asked for the weekly signups number",
				Tags:        []string{"growth", "reporting"},
				Created:     "2026-01-01T08:15:00-08:00",
				Scope: []string{
					"projects.acme-migration.objectives.q1-growth",
					"areas.growth",
				},
			},
		},
		{
			Locator: loc(t, "skills.commit-style"),
			Kind:    kindmeta.KindSkill,
			State: truth.State{
				Name:        "Commit style",
				Description: "when writing a commit message",
				Created:     "2026-01-01T08:20:00-08:00",
			},
		},
	}
}

func shapeName(in render.In) string {
	if len(in.Locator) == 0 {
		return "root"
	}
	return in.Locator.String()
}
