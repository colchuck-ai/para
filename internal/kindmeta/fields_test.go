package kindmeta

import "testing"

// TestFieldMatrix is transcribed directly from the §15 table: every
// (field, kind) cell, checked against Requirement.
func TestFieldMatrix(t *testing.T) {
	na := NotApplicable
	cases := []struct {
		field Field
		want  map[Kind]Requiredness
	}{
		{FieldName, map[Kind]Requiredness{
			KindProject: Required, KindArea: Required, KindResource: Required,
			KindObjective: Required, KindKeyResult: Required, KindSkill: Required,
			KindContainer: Required,
		}},
		{FieldDescription, map[Kind]Requiredness{
			KindProject: Required, KindArea: Required, KindResource: Required,
			KindObjective: Required, KindKeyResult: Optional, KindSkill: Required,
			KindContainer: Required,
		}},
		{FieldStatus, map[Kind]Requiredness{
			KindProject: Optional, KindArea: na, KindResource: na,
			KindObjective: Optional, KindKeyResult: Derived, KindSkill: na,
			KindContainer: na,
		}},
		{FieldPriority, map[Kind]Requiredness{
			KindProject: Optional, KindArea: Optional, KindResource: na,
			KindObjective: Optional, KindKeyResult: na, KindSkill: na,
			KindContainer: na,
		}},
		{FieldDue, map[Kind]Requiredness{
			KindProject: Optional, KindArea: na, KindResource: na,
			KindObjective: Optional, KindKeyResult: Optional, KindSkill: na,
			KindContainer: na,
		}},
		{FieldTags, map[Kind]Requiredness{
			KindProject: Optional, KindArea: Optional, KindResource: Optional,
			KindObjective: Optional, KindKeyResult: Optional, KindSkill: Optional,
			KindContainer: na,
		}},
		{FieldCreated, map[Kind]Requiredness{
			KindProject: Optional, KindArea: Optional, KindResource: Optional,
			KindObjective: Optional, KindKeyResult: Optional, KindSkill: Optional,
			KindContainer: Optional,
		}},
		{FieldType, map[Kind]Requiredness{
			KindProject: na, KindArea: na, KindResource: na,
			KindObjective: na, KindKeyResult: RequiredFixed, KindSkill: na,
			KindContainer: na,
		}},
		{FieldStart, map[Kind]Requiredness{
			KindProject: na, KindArea: na, KindResource: na,
			KindObjective: na, KindKeyResult: Optional, KindSkill: na,
			KindContainer: na,
		}},
		{FieldTarget, map[Kind]Requiredness{
			KindProject: na, KindArea: na, KindResource: na,
			KindObjective: na, KindKeyResult: Required, KindSkill: na,
			KindContainer: na,
		}},
		{FieldScope, map[Kind]Requiredness{
			KindProject: na, KindArea: na, KindResource: na,
			KindObjective: na, KindKeyResult: na, KindSkill: Optional,
			KindContainer: na,
		}},
	}

	for _, c := range cases {
		for kind, want := range c.want {
			got := Requirement(kind, c.field)
			if got != want {
				t.Errorf("Requirement(%v, %q) = %v, want %v", kind, c.field, got, want)
			}
			wantHas := want != NotApplicable
			if gotHas := Has(kind, c.field); gotHas != wantHas {
				t.Errorf("Has(%v, %q) = %v, want %v", kind, c.field, gotHas, wantHas)
			}
		}
	}
}

func TestAllFieldsOrderIsFixed(t *testing.T) {
	want := []Field{
		FieldName, FieldDescription, FieldStatus, FieldPriority, FieldDue,
		FieldTags, FieldCreated, FieldType, FieldStart, FieldTarget, FieldScope,
	}
	for i := 0; i < 5; i++ {
		got := AllFields()
		if len(got) != len(want) {
			t.Fatalf("AllFields() = %v, want %v", got, want)
		}
		for j := range got {
			if got[j] != want[j] {
				t.Fatalf("AllFields() = %v, want %v", got, want)
			}
		}
	}
}

func TestSortableFields(t *testing.T) {
	cases := []struct {
		kind Kind
		want []Field
	}{
		{KindProject, []Field{FieldName, FieldDescription, FieldStatus, FieldPriority, FieldDue, FieldTags, FieldCreated}},
		{KindArea, []Field{FieldName, FieldDescription, FieldPriority, FieldTags, FieldCreated}},
		{KindResource, []Field{FieldName, FieldDescription, FieldTags, FieldCreated}},
		{KindObjective, []Field{FieldName, FieldDescription, FieldStatus, FieldPriority, FieldDue, FieldTags, FieldCreated}},
		{KindKeyResult, []Field{FieldName, FieldDescription, FieldStatus, FieldDue, FieldTags, FieldCreated, FieldType, FieldStart, FieldTarget}},
		{KindSkill, []Field{FieldName, FieldDescription, FieldTags, FieldCreated, FieldScope}},
		{KindContainer, []Field{FieldName, FieldDescription, FieldCreated}},
	}
	for _, c := range cases {
		got := SortableFields(c.kind)
		if len(got) != len(c.want) {
			t.Fatalf("SortableFields(%v) = %v, want %v", c.kind, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("SortableFields(%v) = %v, want %v", c.kind, got, c.want)
			}
		}
	}
}

func TestUnknownKindHasNoFields(t *testing.T) {
	if got := SortableFields(KindUnknown); got != nil {
		t.Errorf("SortableFields(KindUnknown) = %v, want nil", got)
	}
	for _, f := range AllFields() {
		if Has(KindUnknown, f) {
			t.Errorf("Has(KindUnknown, %q) = true, want false", f)
		}
	}
}

// TestUnsettable walks §15's table for the three refusals `unset` makes: a
// field the kind requires, `type` (fixed at creation), and `created` (which
// everything has). Everything else the kind has, it can lose.
//
// This is the rule the shell completion shares with `mutate.Unset`, which is
// why it is a table here rather than an assertion at either call site.
func TestUnsettable(t *testing.T) {
	cases := []struct {
		kind  Kind
		field Field
		want  bool
	}{
		{KindProject, FieldName, false},        // required
		{KindProject, FieldDescription, false}, // required
		{KindProject, FieldCreated, false},     // everything has a creation time
		{KindProject, FieldStatus, true},
		{KindProject, FieldPriority, true},
		{KindProject, FieldDue, true},
		{KindProject, FieldTags, true},
		{KindProject, FieldScope, false}, // a project has no scope at all
		{KindKeyResult, FieldType, false},
		{KindKeyResult, FieldTarget, false}, // required
		{KindKeyResult, FieldStart, true},
		{KindKeyResult, FieldDescription, true}, // optional here, unlike a project's
		{KindKeyResult, FieldStatus, true},      // derived, and derived is not required
		{KindSkill, FieldScope, true},           // absence is what widens a skill (§5.2)
		{KindSkill, FieldName, false},
		{KindArea, FieldStatus, false}, // an area has no status
		{KindUnknown, FieldName, false},
	}
	for _, c := range cases {
		if got := Unsettable(c.kind, c.field); got != c.want {
			t.Errorf("Unsettable(%v, %q) = %v, want %v", c.kind, c.field, got, c.want)
		}
	}
}

// TestUnsettableFields is the same rule as a set, in AllFields order — no map
// iteration reaches it, so two calls list the same fields the same way (§0.2).
func TestUnsettableFields(t *testing.T) {
	cases := []struct {
		kind Kind
		want []Field
	}{
		{KindProject, []Field{FieldStatus, FieldPriority, FieldDue, FieldTags}},
		{KindArea, []Field{FieldPriority, FieldTags}},
		{KindResource, []Field{FieldTags}},
		{KindObjective, []Field{FieldStatus, FieldPriority, FieldDue, FieldTags}},
		{KindKeyResult, []Field{FieldDescription, FieldStatus, FieldDue, FieldTags, FieldStart}},
		{KindSkill, []Field{FieldTags, FieldScope}},
		{KindContainer, nil},
		{KindUnknown, nil},
	}
	for _, c := range cases {
		got := UnsettableFields(c.kind)
		if len(got) != len(c.want) {
			t.Fatalf("UnsettableFields(%v) = %v, want %v", c.kind, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("UnsettableFields(%v) = %v, want %v", c.kind, got, c.want)
			}
		}
	}
	// Every field it offers, Unsettable agrees to — the two must not be able to
	// disagree, since one is the list and the other is the gate.
	for _, kind := range []Kind{KindProject, KindArea, KindResource, KindObjective, KindKeyResult, KindSkill, KindContainer} {
		for _, f := range UnsettableFields(kind) {
			if !Unsettable(kind, f) {
				t.Errorf("UnsettableFields(%v) offers %q, which Unsettable refuses", kind, f)
			}
		}
	}
}
