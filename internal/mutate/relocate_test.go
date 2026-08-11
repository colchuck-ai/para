package mutate_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/doctor"
	"github.com/colchuck-ai/para/internal/mutate"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/view"
)

// addArea creates an area at the given locator, creating nothing else.
func addArea(t *testing.T, e *mutate.Env, dotted string) {
	t.Helper()
	id := dotted[strings.LastIndex(dotted, ".")+1:]
	if _, err := e.Add(loc(t, dotted), fields("name", id, "description", "The "+id+".")); err != nil {
		t.Fatalf("add %s: %v", dotted, err)
	}
}

func addSkill(t *testing.T, e *mutate.Env, id string, scope ...string) {
	t.Helper()
	f := fields("name", id, "description", "when "+id)
	if len(scope) > 0 {
		f.SetList("scope", scope)
	}
	if _, err := e.Add(loc(t, "skills."+id), f); err != nil {
		t.Fatalf("add skills.%s: %v", id, err)
	}
}

func exists(t *testing.T, root, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func journalOf(t *testing.T, root, dirRel string) string {
	t.Helper()
	logs := filepath.Join(root, filepath.FromSlash(dirRel), ".para", "logs")
	entries, err := os.ReadDir(logs)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(logs, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b.Write(data)
	}
	return b.String()
}

func wantKind(t *testing.T, err error, kind paraerr.Kind) {
	t.Helper()
	if err == nil {
		t.Fatalf("want a %s error, got nil", kind)
	}
	var pe *paraerr.Error
	if !errors.As(err, &pe) {
		t.Fatalf("want a paraerr.Error, got %T: %v", err, err)
	}
	if pe.Kind != kind {
		t.Fatalf("error kind: got %s, want %s (%v)", pe.Kind, kind, err)
	}
}

func wantMessage(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("want an error mentioning %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("error %q does not mention %q", err, substr)
	}
}

// --- move -------------------------------------------------------------------

func TestMoveRelocatesTheSubtreeAndRewritesEveryLocator(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	addArea(t, e, "areas.health.training.tempo")
	addArea(t, e, "areas.fitness")

	e = env(t, root)
	plan, err := e.PlanMove(loc(t, "areas.health.training"), loc(t, "areas.fitness.training"))
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if plan.Descendants != 1 {
		t.Errorf("descendants: got %d, want 1", plan.Descendants)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, "areas/health/training") {
		t.Error("the source directory survived the move")
	}
	if !exists(t, root, "areas/fitness/training/tempo/.para/state.toml") {
		t.Error("the subtree did not arrive at the destination")
	}

	// The moved entity's README, and its descendant's, both carry the new
	// locator: README frontmatter is the only projection that names one (§18.3).
	if got := read(t, root, "areas/fitness/training/README.md"); !strings.Contains(got, `locator: "area.fitness.training"`) {
		t.Errorf("moved README frontmatter not rewritten:\n%s", got)
	}
	if got := read(t, root, "areas/fitness/training/tempo/README.md"); !strings.Contains(got, `locator: "area.fitness.training.tempo"`) {
		t.Errorf("descendant README frontmatter not rewritten:\n%s", got)
	}

	// The entity's own history keeps the move visible — the one place a derived
	// value enters a journal (§18.3).
	if got := journalOf(t, root, "areas/fitness/training"); !strings.Contains(got,
		`"kind":"change","field":"locator","from":"areas.health.training","to":"areas.fitness.training"`) {
		t.Errorf("no locator change event on the moved entity:\n%s", got)
	}

	// Both parents log, because both have a different set of children (§3.3).
	for _, dir := range []string{"areas/health", "areas/fitness"} {
		if got := journalOf(t, root, dir); !strings.Contains(got, `"op":"moved","child":"training"`) {
			t.Errorf("%s did not log the move:\n%s", dir, got)
		}
	}

	// A descendant's own fields did not change, so it gets no event (§3.2).
	if got := journalOf(t, root, "areas/fitness/training/tempo"); got != "" {
		t.Errorf("the descendant's journal gained events:\n%s", got)
	}
}

func TestMoveRewritesAContainersLocatorToo(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	plan, err := env(t, root).PlanMove(loc(t, "projects.acme"), loc(t, "projects.acme-migration"))
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// A container's README carries a locator like everything else's (§2.2), and
	// the eager objectives/ container came along with the project (§18.1).
	got := read(t, root, "projects/acme-migration/objectives/README.md")
	if !strings.Contains(got, `locator: "container.acme-migration.objectives"`) {
		t.Errorf("the container's README still names the old locator:\n%s", got)
	}
}

func TestMoveWithinOneParentLogsOneEventNamingWhatLeft(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")

	e = env(t, root)
	plan, err := e.PlanMove(loc(t, "areas.health.training"), loc(t, "areas.health.plan"))
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := journalOf(t, root, "areas/health")
	if n := strings.Count(got, `"op":"moved"`); n != 1 {
		t.Errorf("a rename inside one parent logged %d move events, want 1:\n%s", n, got)
	}
	if !strings.Contains(got, `"from":"areas.health.training","to":"areas.health.plan","op":"moved","child":"training"`) {
		t.Errorf("the event does not name what left and where it went:\n%s", got)
	}
}

func TestMoveRewritesEveryScopeEntryBeneathTheMovedLocator(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	addArea(t, e, "areas.fitness")
	addSkill(t, e, "training-plan", "area.health.training", "project")
	addSkill(t, e, "elsewhere", "project")

	e = env(t, root)
	plan, err := e.PlanMove(loc(t, "areas.health"), loc(t, "areas.wellbeing"))
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if len(plan.ScopeRewrites) != 1 {
		t.Fatalf("scope rewrites: got %d, want 1 (%+v)", len(plan.ScopeRewrites), plan.ScopeRewrites)
	}
	if got := plan.ScopeRewrites[0].Skill.String(); got != "skills.training-plan" {
		t.Errorf("rewrote scope in %s, want skills.training-plan", got)
	}
	if plan.ScopeRewrites[0].Entries != 1 {
		t.Errorf("entries rewritten: got %d, want 1", plan.ScopeRewrites[0].Entries)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// An entry covers its locator and everything beneath it (§5.2), so an entry
	// naming a descendant of the moved locator is rewritten too.
	state := read(t, root, ".agents/skills/para-training-plan/.para/state.toml")
	if !strings.Contains(state, `"area.wellbeing.training"`) {
		t.Errorf("scope entry not rewritten:\n%s", state)
	}
	if got := read(t, root, ".agents/rules/para-training-plan.md"); !strings.Contains(got, "`areas/wellbeing/training/`") {
		t.Errorf("the derived rule still names the old path:\n%s", got)
	}
	// The skill's own field changed, so its history says so (§3.1).
	if got := journalOf(t, root, ".agents/skills/para-training-plan"); !strings.Contains(got, `"field":"scope"`) {
		t.Errorf("the scope rewrite left no event:\n%s", got)
	}
	// A skill the move did not touch is untouched.
	if got := journalOf(t, root, ".agents/skills/para-elsewhere"); got != "" {
		t.Errorf("an unaffected skill gained events:\n%s", got)
	}
}

func TestMoveRenamesASkillAndItsDerivedRule(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addSkill(t, e, "old-name")

	e = env(t, root)
	plan, err := e.PlanMove(loc(t, "skills.old-name"), loc(t, "skills.new-name"))
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, ".agents/skills/para-old-name") {
		t.Error("the old skill directory survived")
	}
	if !exists(t, root, ".agents/skills/para-new-name/SKILL.md") {
		t.Error("the skill did not arrive under its new id")
	}
	// A rule is a projection of a skill and nothing else (§5.3): the old id's
	// rule would be doctor's orphan-rule.
	if exists(t, root, ".agents/rules/para-old-name.md") {
		t.Error("the old derived rule survived the rename")
	}
	if got := read(t, root, ".agents/rules/para-new-name.md"); !strings.Contains(got, `generated_from: "para-new-name"`) {
		t.Errorf("the new rule does not name its skill:\n%s", got)
	}
}

func TestMoveRefusals(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")

	cases := []struct {
		name     string
		src, dst string
		kind     paraerr.Kind
		message  string
	}{
		{
			name: "kind would change", src: "projects.acme", dst: "areas.acme",
			kind: paraerr.KindValidation, message: "kind would change (project → area)",
		},
		{
			name: "into the archive", src: "projects.acme", dst: "archive.projects.acme",
			kind: paraerr.KindValidation, message: "that is `para archive`",
		},
		{
			name: "out of the archive", src: "archive.projects.gone", dst: "projects.gone",
			kind: paraerr.KindNotFound, message: "does not exist",
		},
		{
			name: "destination exists", src: "areas.health.training", dst: "projects.acme",
			kind: paraerr.KindValidation, message: "kind would change",
		},
		{
			name: "destination parent missing", src: "areas.health.training", dst: "areas.missing.training",
			kind: paraerr.KindNotFound, message: "areas.missing does not exist",
		},
		{
			name: "a container is not a subject", src: "projects.acme.objectives", dst: "areas.objectives-of-acme",
			kind: paraerr.KindValidation, message: "is a container",
		},
		{
			name: "into itself", src: "areas.health", dst: "areas.health.inner",
			kind: paraerr.KindValidation, message: "is inside",
		},
		{
			name: "the source does not exist", src: "areas.nothing", dst: "areas.something",
			kind: paraerr.KindNotFound, message: "areas.nothing does not exist",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env(t, root).PlanMove(loc(t, tc.src), loc(t, tc.dst))
			wantKind(t, err, tc.kind)
			wantMessage(t, err, tc.message)
		})
	}
}

func TestMoveOutOfTheArchiveNamesUnarchive(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	_, err := env(t, root).PlanMove(loc(t, "archive.areas.health"), loc(t, "areas.health"))
	wantMessage(t, err, "that is `para unarchive`")
}

// --- archive ----------------------------------------------------------------

func mustPlanArchive(t *testing.T, e *mutate.Env, dotted string) *mutate.Relocation {
	t.Helper()
	plan, err := e.PlanArchive(loc(t, dotted))
	if err != nil {
		t.Fatalf("PlanArchive(%s): %v", dotted, err)
	}
	return plan
}

func TestArchiveCreatesAStubForALiveAncestor(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")

	plan := mustPlanArchive(t, env(t, root), "areas.health.training")
	if len(plan.StubsCreated) != 1 || plan.StubsCreated[0].String() != "archive.areas.health" {
		t.Fatalf("stubs created: got %v, want [archive.areas.health]", plan.StubsCreated)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// A stub is a bare directory: no README.md, no .para/ (§1.6).
	if !exists(t, root, "archive/areas/health") {
		t.Fatal("no stub was created")
	}
	if exists(t, root, "archive/areas/health/.para") || exists(t, root, "archive/areas/health/README.md") {
		t.Error("the stub is not bare")
	}
	if !exists(t, root, "archive/areas/health/training/.para/state.toml") {
		t.Error("the entity did not arrive under the stub")
	}
	if !exists(t, root, "areas/health/.para/state.toml") {
		t.Error("the live parent did not stay behind")
	}

	// The old parent logs; the new parent is a stub, which has no journal.
	if got := journalOf(t, root, "areas/health"); !strings.Contains(got, `"op":"archived","child":"training"`) {
		t.Errorf("the live parent did not log the archival:\n%s", got)
	}
	if got := read(t, root, "archive/areas/health/training/README.md"); !strings.Contains(got, `locator: "archive.area.health.training"`) {
		t.Errorf("the archived README still names the old locator:\n%s", got)
	}
}

func TestArchiveTurnsAnExistingStubIntoTheEntity(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	addArea(t, e, "areas.health.nutrition")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive training: %v", err)
	}

	plan := mustPlanArchive(t, env(t, root), "areas.health")
	if plan.StubAdopted.String() != "archive.areas.health" {
		t.Fatalf("stub adopted: got %q, want archive.areas.health", plan.StubAdopted)
	}
	if plan.Descendants != 1 {
		t.Errorf("descendants: got %d, want 1 (nutrition)", plan.Descendants)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, "areas/health") {
		t.Error("the live directory survived")
	}
	// The stub gained an entity, and kept what was already beneath it.
	if !exists(t, root, "archive/areas/health/.para/state.toml") {
		t.Error("the stub did not become the entity")
	}
	for _, rel := range []string{"archive/areas/health/training", "archive/areas/health/nutrition"} {
		if !exists(t, root, rel+"/.para/state.toml") {
			t.Errorf("%s is not there", rel)
		}
	}
}

func TestArchiveRefusesAChildIdTheStubAlreadyHolds(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive training: %v", err)
	}
	// A live `training` again, beside the archived one.
	addArea(t, env(t, root), "areas.health.training")

	_, err := env(t, root).PlanArchive(loc(t, "areas.health"))
	wantKind(t, err, paraerr.KindConflict)
	wantMessage(t, err, "archive/areas/health/training already exists")
}

func TestArchiveRefusals(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	addSkill(t, e, "commit-style")
	if _, err := mustPlanArchive(t, env(t, root), "projects.acme").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	cases := []struct {
		name    string
		loc     string
		kind    paraerr.Kind
		message string
	}{
		{"already archived", "archive.projects.acme", paraerr.KindValidation, "is already archived"},
		{"a skill has no archive", "skills.commit-style", paraerr.KindValidation, "a skill cannot be archived"},
		{"does not exist", "projects.acme", paraerr.KindNotFound, "does not exist"},
		{"a container", "areas", paraerr.KindValidation, "is a container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env(t, root).PlanArchive(loc(t, tc.loc))
			wantKind(t, err, tc.kind)
			wantMessage(t, err, tc.message)
		})
	}
}

func TestArchiveRefusesWhenTheArchivedCounterpartExists(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := mustPlanArchive(t, env(t, root), "projects.acme").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}
	addProject(t, env(t, root), "acme")

	_, err := env(t, root).PlanArchive(loc(t, "projects.acme"))
	wantKind(t, err, paraerr.KindConflict)
	wantMessage(t, err, "archive.projects.acme already exists")
}

// --- unarchive --------------------------------------------------------------

func TestUnarchiveCascadesUpwardAndDemotesTheAncestorToAStub(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	addArea(t, e, "areas.health.nutrition")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	plan, err := env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	if err != nil {
		t.Fatalf("PlanUnarchive: %v", err)
	}
	if len(plan.Reinstated) != 1 || plan.Reinstated[0].String() != "areas.health" {
		t.Fatalf("reinstated: got %v, want [areas.health]", plan.Reinstated)
	}
	if len(plan.StubsDemoted) != 1 || plan.StubsDemoted[0].String() != "archive.areas.health" {
		t.Fatalf("stubs demoted: got %v, want [archive.areas.health]", plan.StubsDemoted)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The ancestor came back as a live entity, with its truth — the archive side
	// keeps nothing but a bare directory recording the sibling's ancestry.
	if !exists(t, root, "areas/health/.para/state.toml") {
		t.Error("the ancestor was not reinstated")
	}
	if !exists(t, root, "areas/health/training/.para/state.toml") {
		t.Error("the named entity did not come back")
	}
	if !exists(t, root, "archive/areas/health") {
		t.Fatal("the stub recording the archived sibling is gone")
	}
	if exists(t, root, "archive/areas/health/.para") {
		t.Error("the demoted directory kept its .para/ — it should have moved up")
	}
	if !exists(t, root, "archive/areas/health/nutrition/.para/state.toml") {
		t.Error("the archived sibling did not stay archived")
	}

	// Both entities that moved say so in their own journals (§18.3, §18.5).
	for dir, want := range map[string]string{
		"areas/health":          `"field":"locator","from":"archive.areas.health","to":"areas.health"`,
		"areas/health/training": `"field":"locator","from":"archive.areas.health.training","to":"areas.health.training"`,
	} {
		if got := journalOf(t, root, dir); !strings.Contains(got, want) {
			t.Errorf("%s: no locator change event:\n%s", dir, got)
		}
	}
	// The reinstated ancestor is both ends of its child's move, so one event.
	if got := journalOf(t, root, "areas/health"); strings.Count(got, `"op":"unarchived","child":"training"`) != 1 {
		t.Errorf("areas.health should log the child once:\n%s", got)
	}
	// Its own containers log at both ends, because they are different journals.
	for _, dir := range []string{"areas", "archive/areas"} {
		if got := journalOf(t, root, dir); !strings.Contains(got, `"op":"unarchived","child":"health"`) {
			t.Errorf("%s did not log the ancestor coming back:\n%s", dir, got)
		}
	}
}

func TestUnarchiveRemovesAStubLeftRecordingNothing(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	plan, err := env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	if err != nil {
		t.Fatalf("PlanUnarchive: %v", err)
	}
	// The live parent never left, so nothing is reinstated — it is adopted.
	if len(plan.Reinstated) != 0 {
		t.Errorf("reinstated: got %v, want none", plan.Reinstated)
	}
	if len(plan.StubsRemoved) != 1 || plan.StubsRemoved[0].String() != "archive.areas.health" {
		t.Fatalf("stubs removed: got %v, want [archive.areas.health]", plan.StubsRemoved)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, "archive/areas/health") {
		t.Error("a stub recording nothing survived")
	}
	if !exists(t, root, "areas/health/training/.para/state.toml") {
		t.Error("the entity did not come back")
	}
}

func TestUnarchiveAdoptsALiveAncestorThatTookTheId(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// A new, live `health`, unrelated to the archived one.
	addArea(t, env(t, root), "areas.health")

	plan, err := env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	if err != nil {
		t.Fatalf("PlanUnarchive: %v", err)
	}
	if len(plan.Reinstated) != 0 {
		t.Errorf("reinstated: got %v, want none — the live ancestor is adopted", plan.Reinstated)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if !exists(t, root, "areas/health/training/.para/state.toml") {
		t.Error("the entity did not land under the live ancestor")
	}
	// The archived ancestor stays archived: it is a different entity with a
	// different journal, and nothing said to bring it back.
	if !exists(t, root, "archive/areas/health/.para/state.toml") {
		t.Error("the archived ancestor should still be there")
	}
	// Both ends log, and they are two journals here.
	if got := journalOf(t, root, "archive/areas/health"); !strings.Contains(got, `"op":"unarchived","child":"training"`) {
		t.Errorf("the archived parent did not log the child leaving:\n%s", got)
	}
	if got := journalOf(t, root, "areas/health"); !strings.Contains(got, `"op":"unarchived","child":"training"`) {
		t.Errorf("the live parent did not log the child arriving:\n%s", got)
	}
}

func TestUnarchiveLeavesAStillArchivedDescendantsScopeAlone(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	addArea(t, e, "areas.health.nutrition")
	addSkill(t, e, "plan", "area.health.training", "area.health.nutrition")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	plan, err := env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	if err != nil {
		t.Fatalf("PlanUnarchive: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	state := read(t, root, ".agents/skills/para-plan/.para/state.toml")
	if !strings.Contains(state, `"area.health.training"`) {
		t.Errorf("the unarchived entity's entry was not rewritten:\n%s", state)
	}
	// nutrition stayed archived, so its entry must not have moved with the
	// ancestor: the ancestor carried no subtree (§1.6).
	if !strings.Contains(state, `"archive.area.health.nutrition"`) {
		t.Errorf("a still-archived entry was rewritten:\n%s", state)
	}
}

func TestUnarchiveRefusals(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "old-migration")
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "projects.old-migration").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}
	addProject(t, env(t, root), "old-migration")

	cases := []struct {
		name    string
		loc     string
		kind    paraerr.Kind
		message string
	}{
		{
			name: "the id is taken", loc: "archive.projects.old-migration",
			kind: paraerr.KindConflict, message: "projects.old-migration exists; rename it or leave this archived",
		},
		{
			name: "a stub is not an entity", loc: "archive.areas.health",
			kind: paraerr.KindNotFound, message: "nothing to unarchive — archive.areas.health is a stub, not an entity",
		},
		{
			name: "not archived", loc: "areas.health",
			kind: paraerr.KindValidation, message: "is not archived",
		},
		{
			name: "the archive's own containers", loc: "archive.projects",
			kind: paraerr.KindValidation, message: "is part of the archive itself",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env(t, root).PlanUnarchive(loc(t, tc.loc))
			wantKind(t, err, tc.kind)
			wantMessage(t, err, tc.message)
		})
	}
}

func TestUnarchiveRefusesWhenTheLiveParentIsGone(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// The live parent is deleted, leaving the stub pointing at nothing.
	removal, err := env(t, root).PlanRemove(loc(t, "areas.health"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := removal.Apply(); err != nil {
		t.Fatalf("remove: %v", err)
	}

	_, err = env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	wantKind(t, err, paraerr.KindNotFound)
	wantMessage(t, err, "areas.health does not exist — create it first")
}

// --- remove -----------------------------------------------------------------

func TestRemoveDeletesTheSubtreeAndLogsAtTheParent(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	plan, err := env(t, root).PlanRemove(loc(t, "projects.acme"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if len(plan.Descendants) != 1 {
		t.Errorf("descendants: got %v, want the objectives/ container", plan.Descendants)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, "projects/acme") {
		t.Error("the subtree survived")
	}
	if got := journalOf(t, root, "projects"); !strings.Contains(got, `"op":"removed","child":"acme"`) {
		t.Errorf("the parent did not log the removal:\n%s", got)
	}
}

func TestRemoveKeepFilesKeepsBodiesAndDeletesParasFootprint(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	body := "\n# Acme migration\n\nMy own notes, which are mine.\n"
	readme := filepath.Join(root, "projects", "acme", "README.md")
	data := read(t, root, "projects/acme/README.md")
	if err := os.WriteFile(readme, []byte(strings.SplitAfter(data, "---\n")[0]+
		strings.Join(strings.SplitAfter(data, "---\n")[1:2], "")+body), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := env(t, root).PlanRemove(loc(t, "projects.acme"), true)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if len(plan.Strips) != 2 {
		t.Errorf("strips: got %v, want the project's README and its container's", plan.Strips)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Your content is still there; para's footprint is not (§18.4).
	if !exists(t, root, "projects/acme/README.md") {
		t.Fatal("the README was deleted")
	}
	got := read(t, root, "projects/acme/README.md")
	if strings.Contains(got, "kind:") || strings.Contains(got, "---") {
		t.Errorf("the frontmatter block survived:\n%s", got)
	}
	if !strings.Contains(got, "My own notes, which are mine.") {
		t.Errorf("the body did not survive:\n%s", got)
	}
	for _, rel := range []string{"projects/acme/.para", "projects/acme/ACTIVITY.md", "projects/acme/objectives/.para"} {
		if exists(t, root, rel) {
			t.Errorf("%s survived --keep-files", rel)
		}
	}
}

func TestRemoveTakesASkillsDerivedRuleWithIt(t *testing.T) {
	root := plantTree(t)
	addSkill(t, env(t, root), "signups-report", "projects")
	if !exists(t, root, ".agents/rules/para-signups-report.md") {
		t.Fatal("the rule was never written")
	}

	plan, err := env(t, root).PlanRemove(loc(t, "skills.signups-report"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, ".agents/skills/para-signups-report") {
		t.Error("the skill survived")
	}
	if exists(t, root, ".agents/rules/para-signups-report.md") {
		t.Error("the derived rule outlived its skill")
	}
}

func TestRemoveDropsAStubLeftRecordingNothing(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addArea(t, e, "areas.health.training")
	if _, err := mustPlanArchive(t, env(t, root), "areas.health.training").Apply(); err != nil {
		t.Fatalf("archive: %v", err)
	}

	plan, err := env(t, root).PlanRemove(loc(t, "archive.areas.health.training"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if exists(t, root, "archive/areas/health") {
		t.Error("the stub survived with nothing beneath it")
	}
}

func TestRemoveLeavesAnUnresolvableScopeEntryForDoctor(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addArea(t, e, "areas.health")
	addSkill(t, e, "plan", "areas.health")

	plan, err := env(t, root).PlanRemove(loc(t, "areas.health"), false)
	if err != nil {
		t.Fatalf("PlanRemove: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// §5.4 makes para own the rename, not the deletion: there is no locator to
	// rewrite to, and an entry naming nothing is doctor's scope-unresolved.
	if got := read(t, root, ".agents/skills/para-plan/.para/state.toml"); !strings.Contains(got, `"areas.health"`) {
		t.Errorf("the scope entry was silently changed:\n%s", got)
	}
}

func TestArchiveAndUnarchiveAnObjectiveThroughATwoLevelStubChain(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1-growth"),
		fields("name", "Grow signups", "description", "Move the funnel.")); err != nil {
		t.Fatalf("add objective: %v", err)
	}

	// Archiving an objective leaves the project and its objectives/ container
	// live, so ancestry needs a stub at each of them (§1.6).
	plan := mustPlanArchive(t, env(t, root), "projects.acme.objectives.q1-growth")
	if len(plan.StubsCreated) != 2 {
		t.Fatalf("stubs created: got %v, want two levels", plan.StubsCreated)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	for _, rel := range []string{"archive/projects/acme", "archive/projects/acme/objectives"} {
		if !exists(t, root, rel) {
			t.Errorf("%s was not created", rel)
		}
		if exists(t, root, rel+"/.para") {
			t.Errorf("%s is not bare", rel)
		}
	}
	if !exists(t, root, "archive/projects/acme/objectives/q1-growth/key-results/.para/state.toml") {
		t.Error("the objective's own container did not come along")
	}

	// Unarchiving it empties the whole chain, and a stub that records nothing
	// records nothing — deepest first, so the outer one is empty by the time it
	// is judged.
	back, err := env(t, root).PlanUnarchive(loc(t, "archive.projects.acme.objectives.q1-growth"))
	if err != nil {
		t.Fatalf("PlanUnarchive: %v", err)
	}
	if len(back.StubsRemoved) != 2 {
		t.Fatalf("stubs removed: got %v, want two levels", back.StubsRemoved)
	}
	if _, err := back.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if exists(t, root, "archive/projects/acme") {
		t.Error("the stub chain survived with nothing beneath it")
	}
	if !exists(t, root, "projects/acme/objectives/q1-growth/.para/state.toml") {
		t.Error("the objective did not come back")
	}
	// The live container never left, so it was adopted rather than reinstated.
	if len(back.Reinstated) != 0 {
		t.Errorf("reinstated: got %v, want none", back.Reinstated)
	}
}

// TestUnarchiveReinstatingAnAncestorLeavesACleanTree is the regression test for
// a defect Phase 14's crash matrix surfaced by adding `unarchive` to it.
//
// A reinstated ancestor is both a *subject* of the relocation — its own locator
// changed — and a *parent* of it, since the child moved into it. Both plans
// rendered its ACTIVITY.md from the bytes on disk before the mutation, so the
// parent's write overwrote the subject's and the `locator` change line vanished:
// a command that succeeded, and `doctor` red immediately afterwards.
func TestUnarchiveReinstatingAnAncestorLeavesACleanTree(t *testing.T) {
	// A real tree rather than a planted one, so the only thing a whole-tree
	// `doctor` can find at the end is what these mutations did.
	root, _ := initAt(t, t.TempDir(), "brain")
	e := env(t, root)

	mustAdd(t, e, "areas.health", "Health", "Staying in one piece.")
	mustAdd(t, e, "areas.health.training", "Training", "The weekly plan.")
	archivePlan, err := env(t, root).PlanArchive(loc(t, "areas.health.training"))
	mustRelocate(t, archivePlan, err)
	archivePlan, err = env(t, root).PlanArchive(loc(t, "areas.health"))
	mustRelocate(t, archivePlan, err)

	unarchivePlan, err := env(t, root).PlanUnarchive(loc(t, "archive.areas.health.training"))
	mustRelocate(t, unarchivePlan, err)

	// The reinstated ancestor's own ACTIVITY.md must carry both of the things
	// that happened to it: its locator changed, and it gained a child back.
	got := read(t, root, "areas/health/ACTIVITY.md")
	for _, want := range []string{
		"Changed **locator** from archive.areas.health to areas.health.",
		"Unarchived area **training**.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("areas/health/ACTIVITY.md =\n%s\nwant it to contain %q", got, want)
		}
	}

	// And the whole tree agrees with its journals, which is the property the
	// missing line broke.
	rep, err := doctor.Run(view.NewEnv(root, clock.Fixed{At: now()}), doctor.Options{})
	if err != nil {
		t.Fatalf("doctor.Run: %v", err)
	}
	if !rep.Clean() {
		var lines []string
		for _, f := range rep.Findings {
			lines = append(lines, string(f.Kind)+" "+f.Path+" — "+f.Detail)
		}
		t.Errorf("the tree is not clean after unarchive:\n%s", strings.Join(lines, "\n"))
	}
}

func mustAdd(t *testing.T, e *mutate.Env, locStr, name, description string) {
	t.Helper()
	if _, err := e.Add(loc(t, locStr), fields("name", name, "description", description)); err != nil {
		t.Fatalf("Add(%s): %v", locStr, err)
	}
}

func mustRelocate(t *testing.T, plan *mutate.Relocation, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("planning a relocation: %v", err)
	}
	if _, err := plan.Apply(); err != nil {
		t.Fatalf("applying a relocation: %v", err)
	}
}
