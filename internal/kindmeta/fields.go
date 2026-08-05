package kindmeta

// Field is the name of a stored field (§15). One spelling, shared by add,
// set, unset, and --sort.
type Field string

const (
	FieldName        Field = "name"
	FieldDescription Field = "description"
	FieldStatus      Field = "status"
	FieldPriority    Field = "priority"
	FieldDue         Field = "due"
	FieldTags        Field = "tags"
	FieldCreated     Field = "created"
	FieldType        Field = "type"
	FieldStart       Field = "start"
	FieldTarget      Field = "target"
	FieldScope       Field = "scope"
)

// allFields is every field in the §15 table, in the table's own row order.
// This is the fixed order every field-set derived from the matrix is
// returned in — no map iteration (§0.2).
var allFields = []Field{
	FieldName, FieldDescription, FieldStatus, FieldPriority, FieldDue,
	FieldTags, FieldCreated, FieldType, FieldStart, FieldTarget, FieldScope,
}

// Requiredness is what the §15 table cell for a (kind, field) pair says.
type Requiredness int

const (
	// NotApplicable means the kind has no such field: "—" in §15.
	NotApplicable Requiredness = iota
	// Optional means the field may be set but need not be.
	Optional
	// Required means the field must be given at creation.
	Required
	// RequiredFixed means the field must be given at creation and can
	// never be changed afterward (only key-result's type, §15).
	RequiredFixed
	// Derived means the field is computed, not stored, except for the one
	// settable value carved out of it (key-result status: dropped).
	Derived
)

// matrix is transcribed directly from the §15 table.
var matrix = map[Kind]map[Field]Requiredness{
	KindProject: {
		FieldName: Required, FieldDescription: Required, FieldStatus: Optional,
		FieldPriority: Optional, FieldDue: Optional, FieldTags: Optional, FieldCreated: Optional,
	},
	KindArea: {
		FieldName: Required, FieldDescription: Required,
		FieldPriority: Optional, FieldTags: Optional, FieldCreated: Optional,
	},
	KindResource: {
		FieldName: Required, FieldDescription: Required,
		FieldTags: Optional, FieldCreated: Optional,
	},
	KindObjective: {
		FieldName: Required, FieldDescription: Required, FieldStatus: Optional,
		FieldPriority: Optional, FieldDue: Optional, FieldTags: Optional, FieldCreated: Optional,
	},
	KindKeyResult: {
		FieldName: Required, FieldDescription: Optional, FieldStatus: Derived,
		FieldDue: Optional, FieldTags: Optional, FieldCreated: Optional,
		FieldType: RequiredFixed, FieldStart: Optional, FieldTarget: Required,
	},
	KindSkill: {
		FieldName: Required, FieldDescription: Required, FieldTags: Optional,
		FieldCreated: Optional, FieldScope: Optional,
	},
	KindContainer: {
		FieldName: Required, FieldDescription: Required, FieldCreated: Optional,
	},
}

// Requirement reports what §15 says about field on kind.
func Requirement(kind Kind, field Field) Requiredness {
	fields, ok := matrix[kind]
	if !ok {
		return NotApplicable
	}
	return fields[field]
}

// Has reports whether kind has field at all — the lookup Phase 0.3 calls
// out as replacing v2's noun×field matrix (§15's field vocabulary, from
// which add's legal flags, set/unset validation, and --sort validation all
// fall out as one lookup).
func Has(kind Kind, field Field) bool {
	return Requirement(kind, field) != NotApplicable
}

// AllFields returns every field name in the §15 table, in the table's fixed
// row order.
func AllFields() []Field {
	out := make([]Field, len(allFields))
	copy(out, allFields)
	return out
}

// SortableFields returns the fields kind has, in AllFields order — the
// sort-key set §17's per-kind --sort validation is built from.
func SortableFields(kind Kind) []Field {
	var out []Field
	for _, f := range allFields {
		if Has(kind, f) {
			out = append(out, f)
		}
	}
	return out
}
