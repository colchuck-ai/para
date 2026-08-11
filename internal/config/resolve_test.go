package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
)

// §7's own worked chain: asking for stale-after on a key-result consults, in
// order, the key-result, its key-results/ container, the objective,
// objectives/, the project, projects/, and the root.
func TestChainWalksEveryAncestorNearestFirst(t *testing.T) {
	tests := []struct {
		loc   string
		want  []string // labels, nearest first
		files []string // the config.toml each level reads, root-relative
	}{
		{
			loc:   "",
			want:  []string{"<root>"},
			files: []string{".para/config.toml"},
		},
		{
			loc:   "projects",
			want:  []string{"project", "<root>"},
			files: []string{"projects/.para/config.toml", ".para/config.toml"},
		},
		{
			loc: "projects.acme.objectives.q1-growth.key-results.signups",
			want: []string{
				"key-result.acme.q1-growth.signups",
				"container.acme.q1-growth.key-results",
				"objective.acme.q1-growth",
				"container.acme.objectives",
				"project.acme",
				"project",
				"<root>",
			},
			files: []string{
				"projects/acme/objectives/q1-growth/key-results/signups/.para/config.toml",
				"projects/acme/objectives/q1-growth/key-results/.para/config.toml",
				"projects/acme/objectives/q1-growth/.para/config.toml",
				"projects/acme/objectives/.para/config.toml",
				"projects/acme/.para/config.toml",
				"projects/.para/config.toml",
				".para/config.toml",
			},
		},
		{
			loc:  "areas.health.training",
			want: []string{"area.health.training", "area.health", "area", "<root>"},
			files: []string{
				"areas/health/training/.para/config.toml",
				"areas/health/.para/config.toml",
				"areas/.para/config.toml",
				".para/config.toml",
			},
		},
		{
			loc:  "archive.projects.old-thing",
			want: []string{"archive.project.old-thing", "archive.project", "archive", "<root>"},
			files: []string{
				"archive/projects/old-thing/.para/config.toml",
				"archive/projects/.para/config.toml",
				"archive/.para/config.toml",
				".para/config.toml",
			},
		},
		{
			// §7: "A skill's chain is its own config.toml, then the root —
			// .agents/ is not in the PARA tree, so there is nothing in
			// between." In particular there is no `skills` level.
			loc:  "skills.signups-report",
			want: []string{"skill.signups-report", "<root>"},
			files: []string{
				".agents/skills/para-signups-report/.para/config.toml",
				".para/config.toml",
			},
		},
	}

	for _, tt := range tests {
		name := tt.loc
		if name == "" {
			name = "<root>"
		}
		t.Run(name, func(t *testing.T) {
			loc := parseLoc(t, tt.loc)
			levels, err := Chain(loc)
			if err != nil {
				t.Fatalf("Chain(%q): %v", tt.loc, err)
			}

			var labels, files []string
			for _, l := range levels {
				labels = append(labels, l.Label())
				files = append(files, l.File)
			}
			if !slices.Equal(labels, tt.want) {
				t.Errorf("Chain(%q) labels =\n%v\nwant\n%v", tt.loc, labels, tt.want)
			}
			if !slices.Equal(files, tt.files) {
				t.Errorf("Chain(%q) files =\n%v\nwant\n%v", tt.loc, files, tt.files)
			}
		})
	}
}

func TestResolveNearestWins(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml":                            "project.stale-after = 14\n",
		"projects/.para/config.toml":                   "project.stale-after = 30\n",
		"projects/acme/.para/config.toml":              "project.stale-after = 7\n",
		"areas/.para/config.toml":                      "area.stale-after = 60\n",
		".agents/skills/para-report/.para/config.toml": "review.cadence = 45\n",
	})
	r := NewResolver(root)

	tests := []struct {
		loc    string
		key    string
		want   string
		source string
	}{
		{loc: "projects.acme", key: "project.stale-after", want: "7", source: "project.acme"},
		{loc: "projects.other", key: "project.stale-after", want: "30", source: "project"},
		{loc: "", key: "project.stale-after", want: "14", source: "<root>"},
		// A level that sets a different key does not shadow the one above it.
		{loc: "areas.health", key: "area.stale-after", want: "60", source: "area"},
		// A skill reads its own config, then the root — nothing in between.
		{loc: "skills.report", key: "review.cadence", want: "45", source: "skill.report"},
	}

	for _, tt := range tests {
		t.Run(tt.loc+"/"+tt.key, func(t *testing.T) {
			res, err := r.Resolve(parseLoc(t, tt.loc), tt.key)
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			if !res.Found {
				t.Fatalf("Resolve(%q, %q) found nothing, want %q", tt.loc, tt.key, tt.want)
			}
			if got := Format(res.Value); got != tt.want {
				t.Errorf("value = %q, want %q", got, tt.want)
			}
			src, ok := res.Source()
			if !ok {
				t.Fatalf("Source() reported no level, want %q", tt.source)
			}
			if src.Label() != tt.source {
				t.Errorf("source = %q, want %q", src.Label(), tt.source)
			}
		})
	}
}

// §22's worked example, as a chain: the entity sets nothing, projects/ wins,
// and the root's value is still consulted and still reported.
func TestResolveReportsEveryLevelConsultedWithTheWinnerMarked(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml":          "project.stale-after = 14\n",
		"projects/.para/config.toml": "project.stale-after = 30\n",
	})
	r := NewResolver(root)

	res, err := r.Resolve(parseLoc(t, "projects.acme-migration"), "project.stale-after")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(res.Levels) != 3 {
		t.Fatalf("Levels = %d, want 3 (the entity, projects, the root)", len(res.Levels))
	}
	want := []struct {
		label string
		set   bool
		value string
	}{
		{"project.acme-migration", false, ""},
		{"project", true, "30"},
		{"<root>", true, "14"},
	}
	for i, w := range want {
		l := res.Levels[i]
		if l.Label() != w.label {
			t.Errorf("level %d label = %q, want %q", i, l.Label(), w.label)
		}
		if l.Set != w.set {
			t.Errorf("level %d set = %v, want %v", i, l.Set, w.set)
		}
		if w.set && Format(l.Value) != w.value {
			t.Errorf("level %d value = %q, want %q", i, Format(l.Value), w.value)
		}
	}
	if res.Winner != 1 {
		t.Errorf("Winner = %d, want 1 (projects)", res.Winner)
	}
	if res.FromDefault {
		t.Error("FromDefault = true, but a level supplied the value")
	}
}

func TestResolveFallsBackToTheBuiltInDefault(t *testing.T) {
	root := plant(t, map[string]string{".para/config.toml": ""})
	r := NewResolver(root)

	res, err := r.Resolve(nil, "log.rotate-bytes")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !res.Found || !res.FromDefault {
		t.Fatalf("Found = %v, FromDefault = %v; want both true", res.Found, res.FromDefault)
	}
	if got := Format(res.Value); got != "4194304" {
		t.Errorf("value = %q, want %q", got, "4194304")
	}
	if _, ok := res.Source(); ok {
		t.Error("Source() named a level, but the value came from the built-in default")
	}
}

// §7: "Unset everywhere means the check never fires." An unset threshold
// resolves to nothing at all rather than to a number nobody chose.
func TestResolveOfAnUnsetThresholdFindsNothing(t *testing.T) {
	root := plant(t, map[string]string{".para/config.toml": ""})
	r := NewResolver(root)

	res, err := r.Resolve(parseLoc(t, "projects.acme"), "project.stale-after")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Found {
		t.Errorf("Found = true (value %q), want false — the check should never fire", Format(res.Value))
	}
	if len(res.Levels) != 3 {
		t.Errorf("Levels = %d, want 3 — an unresolved key still prints its chain", len(res.Levels))
	}
}

func TestResolveRejectsAnUnknownKey(t *testing.T) {
	root := plant(t, map[string]string{".para/config.toml": ""})
	r := NewResolver(root)

	_, err := r.Resolve(nil, "project.stale-aftr")
	if err == nil {
		t.Fatal("Resolve of an unknown key = nil error, want a refusal")
	}
	var perr *paraerr.Error
	if !errors.As(err, &perr) || perr.Kind != paraerr.KindValidation {
		t.Errorf("error = %v, want a %v error", err, paraerr.KindValidation)
	}
}

// A hand-edited value of the wrong type is refused by the file that holds
// it, so the message points at the thing to fix.
func TestResolveNamesTheFileHoldingAWrongTypedValue(t *testing.T) {
	root := plant(t, map[string]string{
		"projects/.para/config.toml": "project.stale-after = \"soon\"\n",
	})
	r := NewResolver(root)

	_, err := r.Resolve(parseLoc(t, "projects.acme"), "project.stale-after")
	if err == nil {
		t.Fatal("Resolve = nil error, want a refusal")
	}
	if !strings.Contains(err.Error(), filepath.Join("projects", ".para", "config.toml")) {
		t.Errorf("error %q does not name the file holding the bad value", err)
	}
}

// The plan's condition for this phase: every resolved threshold `show`
// reports (§16.1) must name the level it came from, for every kind that has
// one.
func TestEveryKindsThresholdNamesTheLevelItCameFrom(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml": strings.Join([]string{
			"area.stale-after = 30",
			"key-result.at-risk-pace = 0.8",
			"key-result.stale-after = 7",
			"objective.stale-after = 21",
			"project.stale-after = 14",
			"resource.stale-after = 90",
			"review.cadence = 90",
			"",
		}, "\n"),
		"projects/acme/.para/config.toml": "project.stale-after = 3\n",
	})
	r := NewResolver(root)

	kinds := []struct {
		kind kindmeta.Kind
		loc  string
		want string // the level that should win
	}{
		{kindmeta.KindProject, "projects.acme", "project.acme"},
		{kindmeta.KindObjective, "projects.acme.objectives.q1", "<root>"},
		{kindmeta.KindKeyResult, "projects.acme.objectives.q1.key-results.signups", "<root>"},
		{kindmeta.KindArea, "areas.health", "<root>"},
		{kindmeta.KindResource, "resources.papers", "<root>"},
		{kindmeta.KindSkill, "skills.report", "<root>"},
	}
	for _, k := range kinds {
		t.Run(k.kind.String(), func(t *testing.T) {
			key, ok := StaleKey(k.kind)
			if !ok {
				t.Fatalf("StaleKey(%v) reported no key", k.kind)
			}
			res, err := r.Resolve(parseLoc(t, k.loc), key)
			if err != nil {
				t.Fatalf("Resolve(%q, %q): %v", k.loc, key, err)
			}
			src, ok := res.Source()
			if !ok {
				t.Fatalf("Resolve(%q, %q) named no level", k.loc, key)
			}
			if src.Label() != k.want {
				t.Errorf("source = %q, want %q", src.Label(), k.want)
			}
			if src.File == "" {
				t.Error("the winning level names no config.toml, so §16.1 has nothing to print")
			}
			if _, ok := res.Int(); !ok {
				t.Error("Int() reported nothing for a threshold that resolved")
			}
		})
	}

	// The one threshold that is not a day count.
	res, err := r.Resolve(parseLoc(t, "projects.acme.objectives.q1.key-results.signups"), "key-result.at-risk-pace")
	if err != nil {
		t.Fatalf("Resolve(at-risk-pace): %v", err)
	}
	pace, ok := res.Float()
	if !ok || pace != 0.8 {
		t.Errorf("at-risk-pace = %v, %v; want 0.8, true", pace, ok)
	}
}

func TestRenderConfigResolvesTheEmitKeys(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml": "emit.claude = true\nemit.claude-skills = \"copy\"\n",
	})
	r := NewResolver(root)

	got, err := r.RenderConfig(nil)
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}
	want := render.Config{EmitClaude: true, EmitClaudeSkills: render.MirrorCopy, EmitGitattributes: true}
	if got != want {
		t.Errorf("RenderConfig() = %+v, want %+v", got, want)
	}
}

func TestRenderConfigOnAnEmptyTreeIsTheDefaults(t *testing.T) {
	root := plant(t, map[string]string{".para/config.toml": ""})
	r := NewResolver(root)

	got, err := r.RenderConfig(parseLoc(t, "projects.acme"))
	if err != nil {
		t.Fatalf("RenderConfig: %v", err)
	}
	if got != render.DefaultConfig() {
		t.Errorf("RenderConfig() = %+v, want the defaults %+v", got, render.DefaultConfig())
	}
}

func TestRotateBytesResolvesThroughTheChain(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml":          "",
		"projects/.para/config.toml": "log.rotate-bytes = 1024\n",
	})
	r := NewResolver(root)

	got, err := r.RotateBytes(parseLoc(t, "projects.acme"))
	if err != nil {
		t.Fatalf("RotateBytes: %v", err)
	}
	if got != 1024 {
		t.Errorf("RotateBytes(projects.acme) = %d, want 1024", got)
	}

	got, err = r.RotateBytes(parseLoc(t, "areas.health"))
	if err != nil {
		t.Fatalf("RotateBytes: %v", err)
	}
	if got != 4<<20 {
		t.Errorf("RotateBytes(areas.health) = %d, want the 4 MiB default", got)
	}
}

// A resolver is built once per command and walks the same ancestors for
// every entity under them, so it reads each config.toml once.
func TestResolverReadsEachFileOnce(t *testing.T) {
	root := plant(t, map[string]string{
		".para/config.toml":          "project.stale-after = 14\n",
		"projects/.para/config.toml": "project.stale-after = 30\n",
	})
	r := NewResolver(root)

	for _, loc := range []string{"projects.a", "projects.b", "projects.c"} {
		if _, err := r.Resolve(parseLoc(t, loc), "project.stale-after"); err != nil {
			t.Fatalf("Resolve(%q): %v", loc, err)
		}
	}
	// Deleting the files after they are cached must not change the answer,
	// which is the only observable proof they are not re-read.
	if err := os.RemoveAll(filepath.Join(root, "projects", ".para")); err != nil {
		t.Fatal(err)
	}
	res, err := r.Resolve(parseLoc(t, "projects.d"), "project.stale-after")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got := Format(res.Value); got != "30" {
		t.Errorf("value = %q, want the cached %q", got, "30")
	}
}

func parseLoc(t *testing.T, s string) locator.Locator {
	t.Helper()
	if s == "" {
		return nil
	}
	loc, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return loc
}

// plant writes a tree of files under a temp root, keyed by slash-separated
// relative path, and returns the root.
func plant(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".para"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".para", "tree.toml"), []byte("schema = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A bare `skills` locator names no skill, so it names no level either: §7
// puts nothing between a skill's own config.toml and the root, and
// .agents/skills/ is not a place with policy of its own.
func TestChainRefusesABareSkillsLocator(t *testing.T) {
	if _, err := Chain(parseLoc(t, "skills")); err == nil {
		t.Error("Chain(skills) = nil error, want a refusal — .agents/skills/ is not a level")
	}
	if _, err := Chain(parseLoc(t, "skills.a.b")); err == nil {
		t.Error("Chain(skills.a.b) = nil error, want a refusal — skills is one level only")
	}
}
