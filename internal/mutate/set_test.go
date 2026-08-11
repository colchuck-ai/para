package mutate_test

import (
	"strings"
	"testing"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/mutate"
)

// TestSetWritesOneEventPerChangedFieldInOnePass is §18.2: "any number of fields
// at once, one event per field changed, one write-through pass at the end".
func TestSetWritesOneEventPerChangedFieldInOnePass(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	res, err := e.Set(loc(t, "projects.acme"), fields("status", "in-progress", "priority", "high"), "")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	if len(res.Changes) != 2 {
		t.Fatalf("changes = %v, want two", res.Changes)
	}
	// Reported in §15's row order, not the command line's.
	if res.Changes[0].Field != kindmeta.FieldStatus || res.Changes[1].Field != kindmeta.FieldPriority {
		t.Errorf("changes = %v, want status then priority (§15's row order)", res.Changes)
	}
	if res.Changes[0].From != "planned" || res.Changes[0].To != "in-progress" {
		t.Errorf("status change = %+v, want planned → in-progress", res.Changes[0])
	}

	got := read(t, root, "projects/acme/.para/logs/20260305T170000Z.jsonl")
	want := `{"at":"2026-03-05T17:00:00Z","kind":"change","field":"status","from":"planned","to":"in-progress"}
{"at":"2026-03-05T17:00:00Z","kind":"change","field":"priority","to":"high"}
`
	if got != want {
		t.Errorf("journal =\n%s\nwant\n%s", got, want)
	}

	// One write-through pass: the two changes produce one rewrite of each file,
	// not one per field.
	assertEqual(t, "wrote", res.Wrote, []string{
		"projects/acme/.para/logs/20260305T170000Z.jsonl",
		"projects/acme/.para/logs/20260305T170000Z.jsonl",
		"projects/acme/.para/state.toml",
		"projects/acme/README.md",
		"projects/acme/ACTIVITY.md",
	})
}

// TestSetToTheSameValueWritesNothing is §15's rule, and the reason it exists:
// without it a shell loop buys permanent silence from every check in §20.
func TestSetToTheSameValueWritesNothing(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	before := snapshot(t, root)

	res, err := e.Set(loc(t, "projects.acme"), fields("status", "planned"), "")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(res.Changes) != 0 || len(res.Wrote) != 0 {
		t.Errorf("a no-op set reported %v / wrote %v, want neither", res.Changes, res.Wrote)
	}
	if len(res.NoOps) != 1 || res.NoOps[0].Value != "planned" {
		t.Errorf("no-ops = %v, want status already planned", res.NoOps)
	}
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("a no-op set changed %v, want nothing", changed)
	}
}

// TestSetRecordsANoteWhenNothingElseChanged is §26's third `set` line: "no
// change (status already blocked); note recorded".
//
// §15's no-op rule and that line disagree, and the line wins for the half it
// covers: the field writes nothing, but the reason the user took the trouble to
// type is an act of attention, and dropping it would lose the only new
// information in the command.
func TestSetRecordsANoteWhenNothingElseChanged(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Set(loc(t, "projects.acme"), fields("status", "blocked"), "waiting on the ingest team"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	res, err := e.Set(loc(t, "projects.acme"), fields("status", "blocked"), "still waiting")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if len(res.Changes) != 0 {
		t.Errorf("changes = %v, want none", res.Changes)
	}
	if !res.NoteRecorded {
		t.Error("the note was not recorded")
	}
	if got := read(t, root, "projects/acme/.para/logs/20260305T170000Z.jsonl"); !strings.Contains(got, `"kind":"note","note":"still waiting"`) {
		t.Errorf("journal =\n%s\nwant a note event", got)
	}
	// state.toml is untouched: the field really did not change.
	for _, path := range res.Wrote {
		if strings.HasSuffix(path, "state.toml") {
			t.Errorf("a no-op set rewrote %s", path)
		}
	}
}

func TestSetRefusals(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Q1.")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1.key-results.signups"),
		fields("name", "Signups", "type", "ratio", "target", "2000/12000")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		loc    string
		fields mutate.Fields
		note   string
		want   string
	}{
		{
			// §26: `para set projects.acme-migration --status blocked` alone.
			name: "blocked with no reason", loc: "projects.acme",
			fields: fields("status", "blocked"),
			want:   "--note is required when setting status to blocked",
		},
		{
			name: "a type change", loc: "projects.acme.objectives.q1.key-results.signups",
			fields: fields("type", "number"),
			want:   "type is fixed at creation",
		},
		{
			name: "a field the kind does not have", loc: "projects.acme",
			fields: fields("target", "10"),
			want:   "a project has no target field",
		},
		{
			name: "a locator that names nothing", loc: "projects.missing",
			fields: fields("status", "done"),
			want:   "projects.missing does not exist",
		},
		{
			name: "an unknown priority", loc: "projects.acme",
			fields: fields("priority", "urgent"),
			want:   "not a priority",
		},
		{
			name: "a created date in the future", loc: "projects.acme",
			fields: fields("created", "2027-01-01"),
			want:   "in the future",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Set(loc(t, tt.loc), tt.fields, tt.note)
			errorContains(t, err, tt.want)
		})
	}
}

// TestSetCreatedAfterDueIsRefused is §15's second bound on created, and the one
// that needs both sides of a comparison the same command may be setting.
func TestSetCreatedAfterDueIsRefused(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")

	if _, err := e.Set(loc(t, "projects.acme"), fields("due", "2026-09-30"), ""); err != nil {
		t.Fatalf("Set due: %v", err)
	}
	// The bound is symmetric: a due date earlier than the creation time fails
	// the same comparison from the other side.
	_, err := e.Set(loc(t, "projects.acme"), fields("due", "2026-01-01"), "")
	errorContains(t, err, "after due")

	// A bare date is the whole day, so a deadline of today is legal for
	// something created today — the common case, and one a midnight reading of
	// the date would refuse.
	if _, err := e.Set(loc(t, "projects.acme"), fields("due", "2026-03-05"), ""); err != nil {
		t.Errorf("a deadline of today for something created today was refused: %v", err)
	}

	// Both sides in one command are judged as a whole, so a backdated created
	// and an earlier due are legal together while neither is alone.
	if _, err := e.Set(loc(t, "projects.acme"), fields("created", "2026-01-15", "due", "2026-02-01"), ""); err != nil {
		t.Errorf("setting both at once: %v", err)
	}
}

// TestUnsetRefusals covers §15's three refusals: created, a fixed field, and a
// required one.
func TestUnsetRefusals(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1"), fields("name", "Q1", "description", "Q1.")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Add(loc(t, "projects.acme.objectives.q1.key-results.signups"),
		fields("name", "Signups", "type", "ratio", "target", "2000/12000")); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		loc   string
		field kindmeta.Field
		want  string
	}{
		{"created", "projects.acme", kindmeta.FieldCreated, "created cannot be unset"},
		{"a required field", "projects.acme", kindmeta.FieldName, "name is required"},
		{"a fixed field", "projects.acme.objectives.q1.key-results.signups", kindmeta.FieldType, "fixed at creation"},
		{"a field the kind lacks", "areas.health", kindmeta.FieldDue, "no due field"},
	}
	if _, err := e.Add(loc(t, "areas.health"), fields("name", "Health", "description", "Health.")); err != nil {
		t.Fatal(err)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e.Unset(loc(t, tt.loc), []kindmeta.Field{tt.field})
			errorContains(t, err, tt.want)
		})
	}
}

// TestUnsetScopeWidensASkillToTheWholeTree is §15 and §5.2: absence already
// says "everywhere", so there is no `scope = ["*"]` to write.
func TestUnsetScopeWidensASkillToTheWholeTree(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)

	var f mutate.Fields
	f.Set(kindmeta.FieldName, "Signups report")
	f.Set(kindmeta.FieldDescription, "when asked for the weekly signups number")
	f.SetList(kindmeta.FieldScope, []string{"project.acme"})
	if _, err := e.Add(loc(t, "skills.signups-report"), f); err != nil {
		t.Fatalf("Add skill: %v", err)
	}

	res, err := e.Unset(loc(t, "skills.signups-report"), []kindmeta.Field{kindmeta.FieldScope})
	if err != nil {
		t.Fatalf("Unset: %v", err)
	}
	if len(res.Changes) != 1 || res.Changes[0].To != "" {
		t.Errorf("changes = %v, want scope cleared", res.Changes)
	}
	if got := read(t, root, ".agents/skills/para-signups-report/.para/state.toml"); strings.Contains(got, "scope") {
		t.Errorf("state.toml still carries scope:\n%s", got)
	}
	// The derived rule loses its location clause, which is the whole visible
	// difference between a tree-wide skill and a scoped one (§5.3).
	rule := read(t, root, ".agents/rules/para-signups-report.md")
	if strings.Contains(rule, "When working under") {
		t.Errorf("the rule still names a scope:\n%s", rule)
	}
}

// TestUnsetAnAbsentFieldWritesNothing: unsetting what is already absent is the
// same no-op §15 defines for setting what is already set.
func TestUnsetAnAbsentFieldWritesNothing(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	before := snapshot(t, root)

	res, err := e.Unset(loc(t, "projects.acme"), []kindmeta.Field{kindmeta.FieldDue})
	if err != nil {
		t.Fatalf("Unset: %v", err)
	}
	if len(res.Wrote) != 0 {
		t.Errorf("wrote %v, want nothing", res.Wrote)
	}
	if changed := changedPaths(t, before, snapshot(t, root)); len(changed) != 0 {
		t.Errorf("unsetting an absent field changed %v", changed)
	}
}

// TestSetTouchesNothingOutsideTheEntity is §2.3's invariant for a field change:
// no containment changed, so the parent is not written at all (§3.3).
func TestSetTouchesNothingOutsideTheEntity(t *testing.T) {
	root := plantTree(t)
	e := env(t, root)
	addProject(t, e, "acme")
	before := snapshot(t, root)

	if _, err := e.Set(loc(t, "projects.acme"), fields("status", "in-progress"), ""); err != nil {
		t.Fatalf("Set: %v", err)
	}
	for _, path := range changedPaths(t, before, snapshot(t, root)) {
		if !strings.HasPrefix(path, "projects/acme/") {
			t.Errorf("a field change on projects.acme touched %s", path)
		}
	}
}
