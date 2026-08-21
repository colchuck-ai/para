package mutate_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/view"
)

// initAt runs `init` against a directory that does not exist yet, which is what
// `para init brain` does.
func initAt(t *testing.T, parent, name string) (string, mutate.Result) {
	t.Helper()
	root := filepath.Join(parent, name)
	res, err := mutate.NewEnv(root, clock.Fixed{At: now()}).Init(mutate.Identity{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	return root, res
}

// TestInitWritesTheFileSet is §26's `para init` block: the root marker and its
// two siblings, the root's own generated files, the four buckets, the archive's
// three, and .agents/.
func TestInitWritesTheFileSet(t *testing.T) {
	root, res := initAt(t, t.TempDir(), "brain")

	for _, rel := range []string{
		".para/tree.toml", ".para/config.toml",
		"README.md", "AGENTS.md", "ACTIVITY.md", ".gitattributes",
		"projects/README.md", "projects/AGENTS.md", "projects/ACTIVITY.md", "projects/.para/state.toml",
		"areas/README.md", "resources/README.md", "archive/README.md",
		"archive/projects/README.md", "archive/areas/README.md", "archive/resources/README.md",
		// The skill bucket (para-a3p): a container like the other three, so
		// `show`, `log`, `activity`, `review`, `rebuild`, and `doctor` can
		// resolve the bare `skill` noun the same way they resolve `project`,
		// `area`, and `resource` — but it never gets an AGENTS.md/CLAUDE.md
		// (checked below) or a config.toml (checked below): those are
		// separate, deliberately untouched invariants (§6's fixed eight
		// AGENTS.md places, §7's "no `skills` level to consult").
		".agents/skills/README.md", ".agents/skills/ACTIVITY.md", ".agents/skills/.para/state.toml",
	} {
		if !lstatExists(root, rel) {
			t.Errorf("init did not create %s", rel)
		}
		if !slices.Contains(res.Wrote, rel) {
			t.Errorf("init did not report %s; wrote %v", rel, res.Wrote)
		}
	}
	for _, rel := range []string{".agents/rules", ".agents/skills", ".para/logs", "projects/.para/logs"} {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || !info.IsDir() {
			t.Errorf("init did not create the directory %s", rel)
		}
	}
	for _, rel := range []string{".agents/skills/AGENTS.md", ".agents/skills/CLAUDE.md", ".agents/skills/.para/config.toml"} {
		if lstatExists(root, rel) {
			t.Errorf("init created %s, which the skill bucket does not get", rel)
		}
	}
}

// TestInitWritesNoClaudeSurface is §26's closing line: "no CLAUDE.md, no
// .claude/ — enable with `para config set emit.claude true`".
func TestInitWritesNoClaudeSurface(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")

	for _, rel := range []string{"CLAUDE.md", "projects/CLAUDE.md", ".claude"} {
		if lstatExists(root, rel) {
			t.Errorf("init created %s", rel)
		}
	}
}

// TestInitLeavesACleanTree is the property every other verb is held to, applied
// to the one that makes the tree: doctor is clean and rebuild has nothing to do.
func TestInitLeavesACleanTree(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")

	rep, err := doctor.Run(view.NewEnv(root, clock.Fixed{At: now()}), doctor.Options{})
	if err != nil {
		t.Fatalf("doctor.Run: %v", err)
	}
	if !rep.Clean() {
		var lines []string
		for _, f := range rep.Findings {
			lines = append(lines, string(f.Kind)+" "+f.Path+" "+f.Detail)
		}
		t.Errorf("a freshly initialised tree is not clean:\n%s", strings.Join(lines, "\n"))
	}

	res, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{})
	if err != nil {
		t.Fatalf("rebuild.Run: %v", err)
	}
	if !res.Empty() {
		t.Errorf("rebuild after init changed %v, removed %v", res.Changed, res.Removed)
	}
}

// TestInitNamesTheTreeAfterItsDirectory: §13's shape is `para init [path]` with
// no required flags, so the name has to come from somewhere — and the directory
// is the one thing the user did say.
func TestInitNamesTheTreeAfterItsDirectory(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")

	if got := read(t, root, ".para/tree.toml"); !strings.Contains(got, `name = "brain"`) {
		t.Errorf("tree.toml = %q, want it named after the directory", got)
	}
}

// TestInitTakesAnExplicitIdentity: the root has no locator, so `para set` can
// never reach it (§14) — which makes init the only chance to say what the tree
// is called and what it holds.
func TestInitTakesAnExplicitIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "brain")
	if _, err := mutate.NewEnv(root, clock.Fixed{At: now()}).
		Init(mutate.Identity{Name: "max's brain", Description: "Everything I am carrying."}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	got := read(t, root, ".para/tree.toml")
	for _, want := range []string{`name = "max's brain"`, `description = "Everything I am carrying."`} {
		if !strings.Contains(got, want) {
			t.Errorf("tree.toml = %q, want it to contain %s", got, want)
		}
	}
}

// TestInitLogsAChildEventPerBucket is §8.1: "the root's journal is thin but
// real — init, config changes, and child events for the four buckets". The
// archive's own three are its own event, because it is the parent that changed
// (§3.3).
func TestInitLogsAChildEventPerBucket(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")

	rootLog := journalText(t, root, ".para/logs")
	for _, bucket := range []string{"projects", "areas", "resources", "archive"} {
		want := `"kind":"child","op":"added","child":"` + bucket + `"`
		if !strings.Contains(rootLog, want) {
			t.Errorf("the root journal has no %s event:\n%s", bucket, rootLog)
		}
	}
	if strings.Count(rootLog, `"kind":"child"`) != 4 {
		t.Errorf("the root journal has %d child events, want 4:\n%s", strings.Count(rootLog, `"kind":"child"`), rootLog)
	}

	archiveLog := journalText(t, root, "archive/.para/logs")
	if strings.Count(archiveLog, `"kind":"child"`) != 3 {
		t.Errorf("the archive journal has %d child events, want 3:\n%s", strings.Count(archiveLog, `"kind":"child"`), archiveLog)
	}
}

// TestInitRefusesInsideAnExistingTree is §1.1's rule, and it holds for a
// directory that does not exist yet — which is the whole point, since that is
// what `para init` is usually given.
func TestInitRefusesInsideAnExistingTree(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")

	for _, inside := range []string{".", "projects", filepath.Join("projects", "nested")} {
		_, err := mutate.NewEnv(filepath.Join(root, inside), clock.Fixed{At: now()}).Init(mutate.Identity{})
		if err == nil {
			t.Errorf("init inside %s succeeded", inside)
			continue
		}
		if !strings.Contains(err.Error(), root) {
			t.Errorf("init inside %s said %q, want it to name the tree it found at %s", inside, err, root)
		}
	}
}

// TestInitTheTreeMarkerLandsLast: the marker is what makes the directory a tree,
// so a crash before it leaves something `init` can be run on again, and a crash
// after it leaves truth complete and projections possibly stale — the one
// degraded state §2.4 defines a repair for.
func TestInitTheTreeMarkerLandsLast(t *testing.T) {
	_, res := initAt(t, t.TempDir(), "brain")

	marker, lastTruth := -1, -1
	for i, path := range res.Wrote {
		switch {
		case path == ".para/tree.toml":
			marker = i
		case strings.HasSuffix(path, "/state.toml"), strings.HasSuffix(path, "/config.toml"),
			strings.HasSuffix(path, ".jsonl"):
			lastTruth = i
		}
	}
	if marker < 0 {
		t.Fatalf("no tree.toml among %v", res.Wrote)
	}
	if marker < lastTruth {
		t.Errorf("tree.toml landed at %d, before the last truth file at %d:\n%v", marker, lastTruth, res.Wrote)
	}
}

func journalText(t *testing.T, root, rel string) string {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	var b strings.Builder
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
	}
	return b.String()
}

// TestAddASkillIntoAFreshClone is the repair for a state that had none: git
// does not carry an empty directory, so a clone of a tree with no skills in
// it has no `.agents/rules/` — and `add skills.x` used to refuse a missing
// `.agents/skills/` too, with no command that would fix it. `.agents/skills/`
// itself no longer has that problem (para-a3p gave it a real, always-committed
// state.toml, README.md, and ACTIVITY.md, so a clone always carries it, the
// same way it always carries `projects/`), but `.agents/rules/` stays empty
// until a skill's own rule file lands in it, so it is still exactly what
// `git clone` gives you here.
func TestAddASkillIntoAFreshClone(t *testing.T) {
	root, _ := initAt(t, t.TempDir(), "brain")
	// Precisely what `git clone` gives you.
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(".agents/rules"))); err != nil {
		t.Fatalf("removing .agents/rules: %v", err)
	}

	if _, err := mutate.NewEnv(root, clock.Fixed{At: now()}).
		Add(loc(t, "skills.report"), fields("name", "Report", "description", "when asked")); err != nil {
		t.Fatalf("Add into a tree with no .agents/skills/: %v", err)
	}
	if !lstatExists(root, ".agents/skills/para-report/SKILL.md") {
		t.Error("the skill was not created")
	}
	if !lstatExists(root, ".agents/rules/para-report.md") {
		t.Error("the skill's derived rule was not created")
	}

	rep, err := doctor.Run(view.NewEnv(root, clock.Fixed{At: now()}), doctor.Options{})
	if err != nil {
		t.Fatalf("doctor.Run: %v", err)
	}
	if !rep.Clean() {
		t.Errorf("the tree is not clean: %v", rep.Findings)
	}
}
