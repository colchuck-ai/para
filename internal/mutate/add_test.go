package mutate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mutate"
)

// TestAddWritesTheWholeFileSet is §18.1's file list, including the eager child
// container and the parent's two files — and nothing else, which is §2.3's
// invariant for the one verb that legitimately reaches beyond a single entity.
func TestAddWritesTheWholeFileSet(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	before := snapshot(t, root)

	res := addProject(t, e, "acme-migration")

	assertEqual(t, "wrote", res.Wrote, []string{
		"projects/acme-migration/.para/state.toml",
		"projects/acme-migration/.para/config.toml",
		"projects/acme-migration/objectives/.para/state.toml",
		"projects/acme-migration/objectives/.para/config.toml",
		"projects/acme-migration/README.md",
		"projects/acme-migration/ACTIVITY.md",
		"projects/acme-migration/objectives/README.md",
		"projects/acme-migration/objectives/ACTIVITY.md",
		"projects/.para/logs/20260305T170000Z.jsonl",
		"projects/ACTIVITY.md",
	})
	assertEqual(t, "files on disk", changedPaths(t, before, snapshot(t, root)), []string{
		"projects/.para/logs/20260305T170000Z.jsonl",
		"projects/ACTIVITY.md",
		"projects/acme-migration/.para/config.toml",
		"projects/acme-migration/.para/state.toml",
		"projects/acme-migration/ACTIVITY.md",
		"projects/acme-migration/README.md",
		"projects/acme-migration/objectives/.para/config.toml",
		"projects/acme-migration/objectives/.para/state.toml",
		"projects/acme-migration/objectives/ACTIVITY.md",
		"projects/acme-migration/objectives/README.md",
	})
}

// TestAddStoresTruthInFieldOrder pins what lands in state.toml: the fields
// given, `created` defaulted to now in UTC (§15.1), and §1.7's default status.
func TestAddStoresTruthInFieldOrder(t *testing.T) {
	root := plantTree(t)
	addProject(t, env(t, root), "acme-migration")

	want := `name = "Acme migration"
description = "Rebuild the consumer."
status = "planned"
created = "2026-03-05T17:00:00Z"
`
	if got := read(t, root, "projects/acme-migration/.para/state.toml"); got != want {
		t.Errorf("state.toml =\n%s\nwant\n%s", got, want)
	}
}

// TestAddLeavesTheNewJournalEmpty is §18.1: `created` is a field, not an event
// (§3.1), so `attention` falls back to it — and the parent is the thing that
// logs.
func TestAddLeavesTheNewJournalEmpty(t *testing.T) {
	root := plantTree(t)
	addProject(t, env(t, root), "acme-migration")

	entries := snapshot(t, root)
	for path := range entries {
		if strings.HasPrefix(path, "projects/acme-migration/.para/logs/") {
			t.Errorf("the new entity's journal has %s, but it should start empty", path)
		}
	}

	parent := read(t, root, "projects/.para/logs/20260305T170000Z.jsonl")
	want := `{"at":"2026-03-05T17:00:00Z","kind":"child","op":"added","child":"acme-migration"}` + "\n"
	if parent != want {
		t.Errorf("the parent's journal =\n%s\nwant\n%s", parent, want)
	}
}

// TestAddCreatesTheEmptyLogsDirectory covers the one directory `add` makes that
// nothing writes into (§5.1's shape, §18.1's empty journal).
func TestAddCreatesTheEmptyLogsDirectory(t *testing.T) {
	root := plantTree(t)
	addProject(t, env(t, root), "acme-migration")

	for _, dir := range []string{
		"projects/acme-migration/.para/logs",
		"projects/acme-migration/objectives/.para/logs",
	} {
		if info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir))); err != nil || !info.IsDir() {
			t.Errorf("expected %s to exist as a directory: %v", dir, err)
		}
	}
}

// TestAddRendersActivityWithTheCreatedLine is the reason render.CreatedDay is
// exported: a file written for the first time must carry the line the full
// re-derivation would produce, or doctor reports drift on a tree nothing has
// touched.
func TestAddRendersActivityWithTheCreatedLine(t *testing.T) {
	root := plantTree(t)
	addProject(t, env(t, root), "acme-migration")

	want := "# Activity\n\n## 2026-03-05\n- Created.\n"
	if got := read(t, root, "projects/acme-migration/ACTIVITY.md"); got != want {
		t.Errorf("ACTIVITY.md =\n%q\nwant\n%q", got, want)
	}
	// The parent's file keeps its prior day and gains today's.
	want = "# Activity\n\n## 2026-03-05\n- Added project **acme-migration**.\n\n## 2026-01-01\n- Created.\n"
	if got := read(t, root, "projects/ACTIVITY.md"); got != want {
		t.Errorf("the parent's ACTIVITY.md =\n%q\nwant\n%q", got, want)
	}
}

func TestAddRefusals(t *testing.T) {
	tests := []struct {
		name   string
		loc    string
		fields mutate.Fields
		want   string
	}{
		{
			// §26: `para add projects.acme-migration` again.
			name: "an existing locator", loc: "projects.acme-migration",
			fields: fields("name", "Again", "description", "Again."),
			want:   "projects.acme-migration already exists",
		},
		{
			// §26: `para add projects.a.b` — a project cannot nest.
			name: "a project nested in a project", loc: "projects.acme-migration.nested",
			fields: fields("name", "Nested", "description", "Nested."),
			want:   "a project cannot nest",
		},
		{
			// §26: `para add projects.objectives` — a reserved word.
			name: "a reserved word as an id", loc: "projects.objectives",
			fields: fields("name", "Objectives", "description", "No."),
			want:   "reserved word",
		},
		{
			name: "a missing parent", loc: "projects.missing.objectives.q1",
			fields: fields("name", "Q1", "description", "Q1."),
			want:   "projects.missing does not exist",
		},
		{
			name: "a missing required field", loc: "projects.other",
			fields: fields("name", "Other"),
			want:   "project needs --description",
		},
		{
			name: "a field the kind does not have", loc: "areas.health",
			fields: fields("name", "Health", "description", "Health.", "due", "2026-09-30"),
			want:   "an area has no due field",
		},
		{
			name: "a status the kind does not have", loc: "projects.other",
			fields: fields("name", "Other", "description", "Other.", "status", "on-track"),
			want:   "not a project status",
		},
		{
			name: "creating inside the archive", loc: "archive.projects.old",
			fields: fields("name", "Old", "description", "Old."),
			want:   "is in the archive",
		},
	}

	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme-migration")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Add(loc(t, tt.loc), tt.fields)
			errorContains(t, err, tt.want)
		})
	}
}

// TestAddKeyResultRequiresTypeAndTarget is §15's "req, fixed" row, and §26's
// fourth refusal.
func TestAddKeyResultRequiresTypeAndTarget(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Q1.")); err != nil {
		t.Fatalf("Add objective: %v", err)
	}

	kr := loc(t, "projects.acme.objectives.q1.key-results.signups")
	_, err := e.Add(kr, fields("name", "Signups", "type", "ratio"))
	errorContains(t, err, "--target")

	_, err = e.Add(kr, fields("name", "Signups", "target", "2000/12000"))
	errorContains(t, err, "--type")

	// The full form lands, storing the ratio exactly as written (§4.1).
	if _, err := e.Add(kr, fields(
		"name", "Weekly signups", "type", "ratio",
		"start", "480/9000", "target", "2000/12000", "due", "2026-09-30",
	)); err != nil {
		t.Fatalf("Add key-result: %v", err)
	}
	// due is stored exactly as typed, the way §8.3's own truth files write it —
	// a deadline is a date somebody chose, not an instant something happened at.
	want := `name = "Weekly signups"
due = "2026-09-30"
created = "2026-03-05T17:00:00Z"
type = "ratio"
start = "480/9000"
target = "2000/12000"
`
	if got := read(t, root, "projects/acme/objectives/q1/key-results/signups/.para/state.toml"); got != want {
		t.Errorf("state.toml =\n%s\nwant\n%s", got, want)
	}
}

// TestAddKeyResultHasNoEagerContainer: only a project and an objective are born
// with one (§18.1), because only they hold children.
func TestAddKeyResultHasNoEagerContainer(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Q1.")); err != nil {
		t.Fatalf("Add objective: %v", err)
	}
	res, err := e.Add(loc(t, "projects.acme.objectives.q1.key-results.signups"),
		fields("name", "Signups", "type", "number", "target", "2000"))
	if err != nil {
		t.Fatalf("Add key-result: %v", err)
	}
	for _, path := range res.Wrote {
		if strings.Contains(path, "signups/") && !strings.HasPrefix(path, "projects/acme/objectives/q1/key-results/signups/") {
			t.Errorf("a key-result created %s, but it holds no children", path)
		}
	}
}

// TestAddSkillWritesItsRuleAndNoParentEvent: a skill's third artifact lives
// outside its directory (§5.3), and .agents/skills/ is not a parent that has a
// journal (§1.4, §3.3).
func TestAddSkillWritesItsRuleAndNoParentEvent(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)

	var f mutate.Fields
	f.Set(kindmeta.FieldName, "Signups report")
	f.Set(kindmeta.FieldDescription, "when asked for the weekly signups number")
	f.SetList(kindmeta.FieldScope, []string{"projects.acme-migration", "areas.growth"})

	res, err := e.Add(loc(t, "skills.signups-report"), f)
	if err != nil {
		t.Fatalf("Add skill: %v", err)
	}
	assertEqual(t, "wrote", res.Wrote, []string{
		".agents/skills/para-signups-report/.para/state.toml",
		".agents/skills/para-signups-report/.para/config.toml",
		".agents/skills/para-signups-report/SKILL.md",
		".agents/skills/para-signups-report/ACTIVITY.md",
		".agents/rules/para-signups-report.md",
	})

	// A scope entry that names nothing is legal: doctor reports it
	// (`scope-unresolved`), add does not refuse it (§5.4).
	if got := read(t, root, ".agents/skills/para-signups-report/.para/state.toml"); !strings.Contains(got, "areas.growth") {
		t.Errorf("state.toml lost the scope entry:\n%s", got)
	}
}
