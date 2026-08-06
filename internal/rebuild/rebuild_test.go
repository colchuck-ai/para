package rebuild_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/rebuild"
)

func now(t *testing.T) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, "2026-03-05T17:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	return at
}

func loc(t *testing.T, s string) locator.Locator {
	t.Helper()
	l, err := locator.Parse(s)
	if err != nil {
		t.Fatalf("locator.Parse(%q): %v", s, err)
	}
	return l
}

// plantTree writes the smallest tree a rebuild can run against: the root
// marker and the buckets. It is deliberately *not* a rendered tree — the
// README.md files and the buckets' ACTIVITY.md files are missing — because
// "someone hand-edited a generated file" and "a merge resolved truth and left
// the projections wrong" are two of the four things §2.4 says rebuild is the
// answer to.
func plantTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".para/tree.toml": "schema = 1\nname = \"brain\"\ndescription = \"Everything.\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
	}
	for _, rel := range []string{
		"projects", "areas", "resources", "archive",
		"archive/projects", "archive/areas", "archive/resources",
	} {
		files[rel+"/.para/state.toml"] = "name = \"" + rel + "\"\ndescription = \"A bucket.\"\ncreated = \"2026-01-01T00:00:00Z\"\n"
	}
	files[".agents/skills/.keep"] = ""
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

func env(t *testing.T, root string) *rebuild.Env {
	t.Helper()
	return rebuild.NewEnv(root)
}

func mut(t *testing.T, root string) *mutate.Env {
	t.Helper()
	return mutate.NewEnv(root, clock.Fixed{At: now(t)})
}

func fields(pairs ...string) mutate.Fields {
	var f mutate.Fields
	for i := 0; i+1 < len(pairs); i += 2 {
		field := kindmeta.Field(pairs[i])
		if mutate.IsListField(field) {
			f.SetList(field, []string{pairs[i+1]})
			continue
		}
		f.Set(field, pairs[i+1])
	}
	return f
}

func run(t *testing.T, root string, opts rebuild.Options) rebuild.Result {
	t.Helper()
	res, err := rebuild.Run(env(t, root), opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	return string(data)
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// snapshot records every file in the tree by content.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestRebuildRegeneratesAHandPlantedTree covers §21.1's first claim: every
// projection under the locator, regenerated from state.toml, tree.toml, and
// the journals. The planted tree has truth and almost no projections, so a
// rebuild has to write the whole set.
func TestRebuildRegeneratesAHandPlantedTree(t *testing.T) {
	root := plantTree(t)

	res := run(t, root, rebuild.Options{})

	for _, want := range []string{
		"README.md", "AGENTS.md", "ACTIVITY.md", ".gitattributes",
		"projects/README.md", "projects/AGENTS.md", "projects/ACTIVITY.md",
		"archive/projects/README.md", "archive/projects/AGENTS.md",
	} {
		if !slices.Contains(res.Changed, want) {
			t.Errorf("rebuild did not write %s; wrote %v", want, res.Changed)
		}
	}
	if got := read(t, root, "projects/README.md"); got == "" {
		t.Error("projects/README.md is empty after a rebuild")
	}
}

// TestRebuildIsIdempotent is §21.1's second claim, and the plan's "proven
// idempotent by running it twice and diffing".
func TestRebuildIsIdempotent(t *testing.T) {
	root := plantTree(t)
	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Note(loc(t, "projects.acme"), "the ingest team is blocked", ""); err != nil {
		t.Fatalf("Note: %v", err)
	}

	run(t, root, rebuild.Options{})
	before := snapshot(t, root)

	res := run(t, root, rebuild.Options{})
	if len(res.Changed) != 0 {
		t.Fatalf("a second rebuild rewrote %v, want nothing", res.Changed)
	}
	after := snapshot(t, root)
	for path, want := range before {
		if got, ok := after[path]; !ok || got != want {
			t.Errorf("%s changed under a second rebuild", path)
		}
	}
	if len(after) != len(before) {
		t.Errorf("the second rebuild changed the file set: %d files, was %d", len(after), len(before))
	}
}

// TestRebuildIsANoOpAfterAMutation is the property the whole design rests on:
// write-through already wrote every projection, so rebuild has nothing to do
// (§2.3, §2.4). If this fails, doctor would report stale-projection on a tree
// nobody touched.
func TestRebuildIsANoOpAfterAMutation(t *testing.T) {
	root := plantTree(t)
	run(t, root, rebuild.Options{})

	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Grow.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1.key-results.signups"),
		fields("name", "Signups", "type", "ratio", "start", "480/9000", "target", "2000/12000")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Measure(loc(t, "projects.acme.objectives.q1.key-results.signups"), "700/10000", "", ""); err != nil {
		t.Fatalf("Measure: %v", err)
	}
	if _, err := e.Add(loc(t, "skills.report"), fields("name", "Report", "description", "when asked for signups")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	res := run(t, root, rebuild.Options{})
	if len(res.Changed) != 0 {
		t.Fatalf("rebuild after a mutation rewrote %v, want nothing", res.Changed)
	}
}

// TestRebuildRestoresAHandEditedFile is §26's own case, and the one
// stale-projection exists to catch.
func TestRebuildRestoresAHandEditedFile(t *testing.T) {
	root := plantTree(t)
	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	run(t, root, rebuild.Options{})
	want := read(t, root, "projects/acme/ACTIVITY.md")

	write(t, root, "projects/acme/ACTIVITY.md", want+"hand-edited\n")

	res := run(t, root, rebuild.Options{Scope: loc(t, "projects.acme")})
	if len(res.Changed) != 1 || res.Changed[0] != "projects/acme/ACTIVITY.md" {
		t.Fatalf("rebuild wrote %v, want just projects/acme/ACTIVITY.md", res.Changed)
	}
	if got := read(t, root, "projects/acme/ACTIVITY.md"); got != want {
		t.Errorf("ACTIVITY.md after rebuild =\n%s\nwant\n%s", got, want)
	}
}

// TestRebuildRederivesActivityFromEveryJournalFile is §21.1's one thing
// write-through never does (§3.5): a rotated journal's days are in the file
// too, not just the newest file's.
func TestRebuildRederivesActivityFromEveryJournalFile(t *testing.T) {
	root := plantTree(t)
	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// A second journal file, holding an older day than the one `add` wrote.
	write(t, root, "projects/acme/.para/logs/20260101T090000Z.jsonl",
		`{"at":"2026-01-01T09:00:00Z","kind":"note","note":"from a rotated file"}`+"\n")
	write(t, root, "projects/acme/ACTIVITY.md", "# Activity\n")

	run(t, root, rebuild.Options{})

	got := read(t, root, "projects/acme/ACTIVITY.md")
	for _, want := range []string{"## 2026-01-01", "from a rotated file", "## 2026-03-05"} {
		if !strings.Contains(got, want) {
			t.Errorf("ACTIVITY.md =\n%s\nmissing %q", got, want)
		}
	}
}

// TestRebuildDryRunWritesNothing covers §21.1's --dry-run: it lists what would
// change, and changes nothing.
func TestRebuildDryRunWritesNothing(t *testing.T) {
	root := plantTree(t)
	before := snapshot(t, root)

	res := run(t, root, rebuild.Options{DryRun: true})
	if len(res.Changed) == 0 {
		t.Fatal("--dry-run on an unrendered tree reported no changes")
	}
	after := snapshot(t, root)
	if len(after) != len(before) {
		t.Fatalf("--dry-run wrote files: %d, was %d", len(after), len(before))
	}
}

// TestRebuildScopeIsInclusive: §21.1 regenerates "every projection under the
// locator", and "under" includes the locator's own (Phase 11's reading).
func TestRebuildScopeIsInclusive(t *testing.T) {
	root := plantTree(t)
	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Staying alive.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	write(t, root, "projects/acme/README.md", "wrong\n")
	write(t, root, "areas/health/README.md", "also wrong\n")

	res := run(t, root, rebuild.Options{Scope: loc(t, "projects.acme")})

	if !slices.Contains(res.Changed, "projects/acme/README.md") {
		t.Errorf("a scoped rebuild skipped the locator's own projection: %v", res.Changed)
	}
	if slices.Contains(res.Changed, "areas/health/README.md") {
		t.Errorf("a scoped rebuild reached outside its scope: %v", res.Changed)
	}
	if got := read(t, root, "areas/health/README.md"); got != "also wrong\n" {
		t.Errorf("a scoped rebuild rewrote a file outside its scope")
	}
}

// TestRebuildRefusesALocatorThatNamesNothing: an empty result is a wrong
// answer to an explicit question (Phase 10's rule for scoped reads).
func TestRebuildRefusesALocatorThatNamesNothing(t *testing.T) {
	root := plantTree(t)
	if _, err := rebuild.Run(env(t, root), rebuild.Options{Scope: loc(t, "projects.nothing")}); err == nil {
		t.Fatal("rebuild of a locator that names nothing: want an error, got nil")
	}
}

// TestRebuildConvergesInOnePassWithTheClaudeSurface is the defect the Phase 12
// review found: CLAUDE.md's import list used to be read off .agents/rules/,
// which is a directory of *projections*. On a tree whose rules are missing —
// §2.4's "a merge resolved truth and left the projections wrong" — rebuild
// wrote all eight CLAUDE.md files before it wrote the rules, so the imports
// were derived from what had not been repaired yet and a second pass was
// needed. §21.1 forbids it in those words: it "never reads a projection to
// produce one".
func TestRebuildConvergesInOnePassWithTheClaudeSurface(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "[emit]\nclaude = true\n")
	// A skill planted as truth alone: no SKILL.md, and no derived rule.
	write(t, root, ".agents/skills/para-alpha/.para/state.toml",
		"name = \"Alpha\"\ndescription = \"when alpha\"\ncreated = \"2026-01-01T00:00:00Z\"\n")

	first := run(t, root, rebuild.Options{})
	if !slices.Contains(first.Changed, ".agents/rules/para-alpha.md") {
		t.Fatalf("the first pass did not write the rule: %v", first.Changed)
	}
	if got := read(t, root, "CLAUDE.md"); !strings.Contains(got, "@.agents/rules/para-alpha.md") {
		t.Errorf("CLAUDE.md after one pass =\n%s\nwant it to import the skill's rule", got)
	}

	second := run(t, root, rebuild.Options{})
	if len(second.Changed) != 0 {
		t.Fatalf("a second pass rewrote %v, want nothing", second.Changed)
	}
}

// TestRebuildDoesNotImportAnOrphanRule is the same rule from the other side: a
// rule file whose skill is gone is `orphan-rule` (§10), and para does not
// import residue it is simultaneously reporting.
func TestRebuildDoesNotImportAnOrphanRule(t *testing.T) {
	root := plantTree(t)
	write(t, root, ".para/config.toml", "[emit]\nclaude = true\n")
	write(t, root, ".agents/rules/para-gone.md",
		"---\ngenerated_from: \"para-gone\"\n---\nUse the **Gone** skill.\n")

	run(t, root, rebuild.Options{})

	if got := read(t, root, "CLAUDE.md"); strings.Contains(got, "para-gone") {
		t.Errorf("CLAUDE.md =\n%s\nwant no import for a rule whose skill is gone", got)
	}
}

// TestRebuildAtAnArchiveStub: a stub is a bare ancestry placeholder with no
// .para/ (§1.6), so scoping a rebuild at one has nothing of its own to write
// and must not be read as an entity.
func TestRebuildAtAnArchiveStub(t *testing.T) {
	root := plantTree(t)
	e := mut(t, root)
	if _, err := e.Add(loc(t, "projects.acme"), fields("name", "Acme", "description", "Rebuild the consumer.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Grow.")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	plan, err := e.PlanArchive(loc(t, "projects.acme.objectives.q1"))
	if err != nil {
		t.Fatalf("PlanArchive: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	run(t, root, rebuild.Options{})

	// archive/projects/acme is a stub: the project itself stayed live.
	res, err := rebuild.Run(env(t, root), rebuild.Options{Scope: loc(t, "archive.projects.acme")})
	if err != nil {
		t.Fatalf("rebuild at a stub: %v", err)
	}
	if len(res.Changed) != 0 {
		t.Errorf("rebuild at a stub rewrote %v, want nothing", res.Changed)
	}
}
