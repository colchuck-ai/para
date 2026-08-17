package mutate_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/mdfile"
	"github.com/colchuck-ai/para/internal/mirror"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/rebuild"
	"github.com/colchuck-ai/para/internal/view"
)

// claudeLocations is the eight places CLAUDE.md is emitted (§6), in the order
// the surface refresh visits them.
var claudeLocations = []string{
	"CLAUDE.md",
	"projects/CLAUDE.md",
	"areas/CLAUDE.md",
	"resources/CLAUDE.md",
	"archive/CLAUDE.md",
	"archive/projects/CLAUDE.md",
	"archive/areas/CLAUDE.md",
	"archive/resources/CLAUDE.md",
}

// treeWithSkill is a rebuilt tree holding one skill and nothing else, so that
// doctor is clean before a test does anything — which is what lets every test
// below end by asserting it is clean again.
//
// It plants more than plantTree does, because these tests measure themselves
// against `doctor`: a tree.toml without a name is `invalid`, and a tree that
// was never rebuilt is stale everywhere.
func treeWithSkill(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		".para/tree.toml":      "schema = 1\nname = \"brain\"\ndescription = \"Everything.\"\ncreated = \"2026-01-01T00:00:00Z\"\n",
		".agents/skills/.keep": "",
	}
	for _, rel := range []string{
		"projects", "areas", "resources", "archive",
		"archive/projects", "archive/areas", "archive/resources",
	} {
		files[rel+"/.para/state.toml"] = "name = \"" + rel + "\"\ndescription = \"A bucket.\"\ncreated = \"2026-01-01T00:00:00Z\"\n"
	}
	writeFiles(t, root, files)

	if _, err := env(t, root).Add(loc(t, "skills.report"),
		fields("name", "Report", "description", "when asked for the signups number")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	return root
}

// setClaude writes a config key at the root through `config set`'s own path, so
// the test exercises what the CLI does rather than editing the file behind it.
func setClaude(t *testing.T, e *mutate.Env, root, key string, value ptoml.Value) mutate.Result {
	t.Helper()
	path := filepath.Join(root, ".para", "config.toml")
	f, err := config.Read(path)
	if err != nil {
		t.Fatalf("config.Read: %v", err)
	}
	before, _ := f.Get(key)
	if _, err := f.Set(key, value); err != nil {
		t.Fatalf("Set(%s): %v", key, err)
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	after, _ := f.Get(key)
	res, err := e.ConfigChange(nil, key, config.Format(before), config.Format(after), data)
	if err != nil {
		t.Fatalf("ConfigChange(%s): %v", key, err)
	}
	return res
}

func assertClean(t *testing.T, root string) {
	t.Helper()
	rep, err := doctor.Run(view.NewEnv(root, clock.Fixed{At: now()}), doctor.Options{})
	if err != nil {
		t.Fatalf("doctor.Run: %v", err)
	}
	if !rep.Clean() {
		var lines []string
		for _, f := range rep.Findings {
			lines = append(lines, string(f.Kind)+" "+f.Path+" "+f.Detail)
		}
		t.Errorf("the tree is not clean after a mutation:\n%s", strings.Join(lines, "\n"))
	}
}

func lstatExists(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// TestConfigSetEmitClaudeWritesTheWholeSurface is §26's own transcript: turning
// the flag on writes .para/config.toml, the eight CLAUDE.md files, and a link
// per skill — in one command, without a rebuild.
//
// It is what Phase 12 left owing. The eight locations are a fixed set rather
// than a walk, so honouring §6.1's "no separate bookkeeping" here costs a
// constant number of writes and does not break §2.3.
func TestConfigSetEmitClaudeWritesTheWholeSurface(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)

	res := setClaude(t, e, root, config.KeyEmitClaude, ptoml.Bool(true))

	if !slices.Contains(res.Wrote, ".para/config.toml") {
		t.Errorf("Wrote = %v, want the config file", res.Wrote)
	}
	for _, rel := range claudeLocations {
		if !slices.Contains(res.Wrote, rel) {
			t.Errorf("Wrote = %v, want it to carry %s", res.Wrote, rel)
		}
		if !lstatExists(root, rel) {
			t.Errorf("%s was not written", rel)
		}
	}
	want := mirror.Change{
		Verb:   mirror.VerbLinked,
		Path:   ".claude/skills/para-report",
		Target: "../../.agents/skills/para-report",
	}
	if !slices.Equal(res.Mirror, []mirror.Change{want}) {
		t.Errorf("Mirror = %v, want %v", res.Mirror, []mirror.Change{want})
	}
	assertClean(t, root)
}

// TestConfigSetEmitClaudeOffSweepsTheSurface is the same command in reverse:
// the residue Phase 12 could only report is now removed by the write that
// caused it.
func TestConfigSetEmitClaudeOffSweepsTheSurface(t *testing.T) {
	root := treeWithSkill(t)
	e := env(t, root)
	setClaude(t, e, root, config.KeyEmitClaude, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(false))

	for _, rel := range claudeLocations {
		if !slices.Contains(res.Removed, rel) {
			t.Errorf("Removed = %v, want it to carry %s", res.Removed, rel)
		}
		if lstatExists(root, rel) {
			t.Errorf("%s survived", rel)
		}
	}
	if want := (mirror.Change{Verb: mirror.VerbRemoved, Path: ".claude/skills/para-report"}); !slices.Contains(res.Mirror, want) {
		t.Errorf("Mirror = %v, want %v", res.Mirror, want)
	}
	if lstatExists(root, ".claude") {
		t.Error(".claude/ survived")
	}
	assertClean(t, root)
}

// TestConfigSetMirrorModeSwitchesInPlace: §6.1 says switching modes is "a config
// change plus a rebuild", and the rebuild is not needed once the config change
// itself owns the fixed set — but it must still be a no-op afterwards.
func TestConfigSetMirrorModeSwitchesInPlace(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, config.KeyEmitClaudeSkills, ptoml.String("copy"))

	if want := (mirror.Change{Verb: mirror.VerbCopied, Path: ".claude/skills/para-report"}); !slices.Equal(res.Mirror, []mirror.Change{want}) {
		t.Fatalf("Mirror = %v, want %v", res.Mirror, want)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "para-report", "SKILL.md")); err != nil {
		t.Errorf("the copy has no SKILL.md: %v", err)
	}
	assertClean(t, root)
}

// TestConfigSetOfAnotherKeyLeavesTheSurfaceAlone: the refresh is triggered by
// the two keys that decide the surface, not by every config write.
func TestConfigSetOfAnotherKeyLeavesTheSurfaceAlone(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))

	res := setClaude(t, env(t, root), root, "project.stale-after", ptoml.Int64(30))

	if len(res.Mirror) != 0 || len(res.Removed) != 0 {
		t.Errorf("an unrelated key touched the surface: %v / %v", res.Mirror, res.Removed)
	}
	for _, rel := range claudeLocations {
		if slices.Contains(res.Wrote, rel) {
			t.Errorf("an unrelated key rewrote %s", rel)
		}
	}
}

// TestAddingASkillKeepsEveryClaudeMdCorrect is the §6.1-versus-§2.3 debt Phase
// 12 recorded: adding a skill changes the set of derived rules and therefore
// every CLAUDE.md import list, and before this the tree stayed stale until a
// rebuild.
func TestAddingASkillKeepsEveryClaudeMdCorrect(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))

	res, err := env(t, root).Add(loc(t, "skills.commit-style"),
		fields("name", "Commit style", "description", "when writing a commit message"))
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	for _, rel := range claudeLocations {
		if !slices.Contains(res.Wrote, rel) {
			t.Errorf("Wrote = %v, want it to carry %s", res.Wrote, rel)
		}
	}
	got := read(t, root, "CLAUDE.md")
	for _, want := range []string{"@.agents/rules/para-commit-style.md", "@.agents/rules/para-report.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("CLAUDE.md =\n%s\nwant it to import %s", got, want)
		}
	}
	if !slices.Contains(res.Mirror, mirror.Change{
		Verb: mirror.VerbLinked, Path: ".claude/skills/para-commit-style",
		Target: "../../.agents/skills/para-commit-style",
	}) {
		t.Errorf("Mirror = %v, want a link for the new skill", res.Mirror)
	}
	assertClean(t, root)
}

// TestAddDryRunOnASkillWithClaudeSurfaceOnPreviewsTheWholeReach is
// TestAddingASkillKeepsEveryClaudeMdCorrect's rehearsal: a dry run must report
// the same eight CLAUDE.md rewrites and the same mirror link the immediately
// following real add produces, even though the skill it is naming does not
// exist on disk yet. Before the fix, AddDryRun's report of a skill add under
// emit.claude was silently missing this whole reach — tree.SkillIDs never saw
// the not-yet-written skill, so the CLAUDE.md diff came back empty and the
// mirror never saw it as missing.
func TestAddDryRunOnASkillWithClaudeSurfaceOnPreviewsTheWholeReach(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	before := snapshot(t, root)

	f := fields("name", "Commit style", "description", "when writing a commit message")
	dry, err := env(t, root).AddDryRun(loc(t, "skills.commit-style"), f)
	if err != nil {
		t.Fatalf("AddDryRun: %v", err)
	}
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("AddDryRun wrote %v, want nothing", changed)
	}

	real, err := env(t, root).Add(loc(t, "skills.commit-style"), f)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	assertEqual(t, "AddDryRun's Wrote", dry.Wrote, real.Wrote)
	if !slices.Equal(dry.Mirror, real.Mirror) {
		t.Errorf("AddDryRun's Mirror = %v, want %v", dry.Mirror, real.Mirror)
	}
	for _, rel := range claudeLocations {
		if !slices.Contains(dry.Wrote, rel) {
			t.Errorf("AddDryRun's Wrote = %v, want it to carry %s", dry.Wrote, rel)
		}
	}
	if !slices.Contains(dry.Mirror, mirror.Change{
		Verb: mirror.VerbLinked, Path: ".claude/skills/para-commit-style",
		Target: "../../.agents/skills/para-commit-style",
	}) {
		t.Errorf("AddDryRun's Mirror = %v, want a link for the new skill", dry.Mirror)
	}
}

// TestRemovingASkillPrunesItsMirror is Phase 13's task 4 on the write path:
// `remove skills.x` takes the rule *and* the mirror with it (§18.2, §6.1).
func TestRemovingASkillPrunesItsMirror(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))

	e := env(t, root)
	plan, err := e.PlanRemove(loc(t, "skills.report"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	res, err := plan.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if lstatExists(root, ".claude/skills/para-report") {
		t.Error("the mirror of a removed skill survived")
	}
	if !slices.Contains(res.Mirror, (mirror.Change{Verb: mirror.VerbRemoved, Path: ".claude/skills/para-report"})) {
		t.Errorf("Mirror = %v, want the mirror removed", res.Mirror)
	}
	if got := read(t, root, "CLAUDE.md"); strings.Contains(got, "para-report") {
		t.Errorf("CLAUDE.md =\n%s\nwant no import for the removed skill", got)
	}
	assertClean(t, root)
}

// TestEditingASkillRefreshesACopiedMirror: in copy mode the mirror holds the
// skill's own files, so any mutation of the skill leaves it drifted. Symlink
// mode cannot drift, which is why it is the default (§6.1).
func TestEditingASkillRefreshesACopiedMirror(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	setClaude(t, env(t, root), root, config.KeyEmitClaudeSkills, ptoml.String("copy"))

	if _, err := env(t, root).Set(loc(t, "skills.report"),
		fields("description", "when asked for the weekly number"), ""); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "para-report", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "weekly number") {
		t.Errorf("the copied SKILL.md =\n%s\nwant the edit carried through", got)
	}
	assertClean(t, root)
}

// TestSkillMutationOnATreeWithTheSurfaceOffWritesNothingExtra: the refresh must
// not manufacture a surface nobody asked for, which is the whole meaning of
// "off by default" (§6.1).
func TestSkillMutationOnATreeWithTheSurfaceOffWritesNothingExtra(t *testing.T) {
	root := treeWithSkill(t)

	res, err := env(t, root).Note(loc(t, "skills.report"), "still useful", "", false)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}

	for _, path := range res.Wrote {
		if strings.HasSuffix(path, "CLAUDE.md") {
			t.Errorf("a skill mutation wrote %s with emit.claude off", path)
		}
	}
	if len(res.Mirror) != 0 || len(res.Removed) != 0 {
		t.Errorf("a skill mutation touched the surface with emit.claude off: %v / %v", res.Mirror, res.Removed)
	}
	if lstatExists(root, ".claude") {
		t.Error(".claude/ was created with emit.claude off")
	}
}

// TestSkillMutationWithTheSurfaceOffPreservesAForeignClaudeFile is para-0o6
// itself: a skill mutation refreshes the surface through WriteClaudeSurface
// (§6.1), which used to treat any CLAUDE.md at a bare-off location as pure
// residue and delete it outright — including content para never wrote. A
// foreign CLAUDE.md must survive a skill mutation exactly as it survives a
// full `rebuild`.
func TestSkillMutationWithTheSurfaceOffPreservesAForeignClaudeFile(t *testing.T) {
	root := treeWithSkill(t)
	foreign := "<!-- BEGIN BEADS INTEGRATION -->\nSee `bd prime` for workflow context.\n<!-- END BEADS INTEGRATION -->\n"
	writeFiles(t, root, map[string]string{"CLAUDE.md": foreign})

	res, err := env(t, root).Note(loc(t, "skills.report"), "still useful", "", false)
	if err != nil {
		t.Fatalf("Note: %v", err)
	}

	if slices.Contains(res.Removed, "CLAUDE.md") {
		t.Errorf("Removed = %v, want CLAUDE.md left alone", res.Removed)
	}
	if got := read(t, root, "CLAUDE.md"); got != foreign {
		t.Errorf("CLAUDE.md =\n%s\nwant it byte-identical to\n%s", got, foreign)
	}
}

// TestConfigSetEmitClaudeOffShortensAForeignClaudeFile is the `config set`
// half of the same bug: turning `emit.claude` off after it was on must take
// only para's block out of a CLAUDE.md that also holds foreign content, not
// delete the file (R5).
func TestConfigSetEmitClaudeOffShortensAForeignClaudeFile(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	foreign := "<!-- BEGIN BEADS INTEGRATION -->\nSee `bd prime` for workflow context.\n<!-- END BEADS INTEGRATION -->\n"
	writeFiles(t, root, map[string]string{"CLAUDE.md": foreign + read(t, root, "CLAUDE.md")})

	res := setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(false))

	if slices.Contains(res.Removed, "CLAUDE.md") {
		t.Errorf("Removed = %v, want CLAUDE.md shortened rather than deleted", res.Removed)
	}
	if got := read(t, root, "CLAUDE.md"); got != foreign {
		t.Errorf("CLAUDE.md =\n%s\nwant only the foreign block\n%s", got, foreign)
	}
	if strings.Contains(read(t, root, "CLAUDE.md"), mdfile.BeginMarker) {
		t.Error("CLAUDE.md still holds para's block after emit.claude turned off")
	}
}

// TestConfigSetOnASkillRefreshesTheMirror is the one skill-subject mutation
// that used to slip past the refresh: `config set --at skills.x <any key>`
// re-renders that skill's ACTIVITY.md, which in copy mode the mirror holds. The
// key is unrelated to the surface, so only the "subject is a skill" rule
// catches it — and that rule has to be the same one every other verb uses.
func TestConfigSetOnASkillRefreshesTheMirror(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	setClaude(t, env(t, root), root, config.KeyEmitClaudeSkills, ptoml.String("copy"))

	e := env(t, root)
	skill := loc(t, "skills.report")
	path := filepath.Join(root, ".agents", "skills", "para-report", ".para", "config.toml")
	f, err := config.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Set(config.KeyReviewCadence, ptoml.Int64(90)); err != nil {
		t.Fatal(err)
	}
	data, err := f.Encode()
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.ConfigChange(skill, config.KeyReviewCadence, "", "90", data)
	if err != nil {
		t.Fatalf("ConfigChange: %v", err)
	}
	if want := (mirror.Change{Verb: mirror.VerbCopied, Path: ".claude/skills/para-report"}); !slices.Contains(res.Mirror, want) {
		t.Errorf("Mirror = %v, want %v", res.Mirror, want)
	}
	assertClean(t, root)
}

// TestEmitClaudeIsReadAtTheRootOnly: §6.1 opens by saying the surface is "off by
// default and turns on together, because it is one concern", and §7's table
// gives both keys `root`. It has to be enforced rather than advised, because the
// surface has two halves in two places — eight CLAUDE.md files, one per
// location, and one .claude/ at the root — so a per-location answer would write
// half of it and call the result clean.
func TestEmitClaudeIsReadAtTheRootOnly(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))

	// Hand-written at a bucket, which is the one way it can get there: the CLI
	// refuses `config set --at` for these keys.
	writeFiles(t, root, map[string]string{"projects/.para/config.toml": "emit.claude = false\n"})

	if _, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{}); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !lstatExists(root, "projects/CLAUDE.md") {
		t.Error("a bucket-level emit.claude removed projects/CLAUDE.md; the root's value decides")
	}
	assertClean(t, root)
}

// TestSurfaceRefreshLeavesRebuildNothingToDo is the property every write-path
// change has to keep: write-through is complete, so rebuild after it is a no-op
// (§2.3, §2.4).
func TestSurfaceRefreshLeavesRebuildNothingToDo(t *testing.T) {
	root := treeWithSkill(t)
	setClaude(t, env(t, root), root, config.KeyEmitClaude, ptoml.Bool(true))
	if _, err := env(t, root).Add(loc(t, "skills.commit-style"),
		fields("name", "Commit style", "description", "when writing a commit message")); err != nil {
		t.Fatalf("Add: %v", err)
	}

	res, err := rebuild.Run(rebuild.NewEnv(root), rebuild.Options{})
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if !res.Empty() {
		t.Errorf("rebuild after the refresh did %v / %v / %v, want nothing", res.Changed, res.Removed, res.Mirror)
	}
}
