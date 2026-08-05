// Package truth reads and writes the tree's truth files: an entity's or
// container's .para/state.toml (design §8.2, §8.3) and the root marker
// .para/tree.toml (§8.1). Truth is what generates every projection, so these
// codecs are the input side of the projection engine.
//
// Every field is carried as the string it was written as, never as a parsed
// time or number. Two reasons, both from the design: `created` and `due` take
// progressive precision (§15.1), so "2026-09-30" and
// "2026-09-30T17:00:00-07:00" are both legal spellings of a stored value and
// re-rendering one as the other would churn the file; and a ratio's
// denominator belongs to the reading (§4.1), so "480/9000" must survive as
// written. Parsing happens where a value is used, not where it is stored.
//
// Field order on write is §15's row order — kindmeta.AllFields() — for every
// kind, filtered to the fields actually present. One declared order rather
// than one per kind, because §8.4's argument for uniform filenames applies
// just as well here: a per-kind order would be a second copy of the kind,
// which the path already states.
package truth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptoml"
)

// State is one .para/state.toml: the stored fields of an entity or container
// (§8.2, §8.3). Absence is the empty string for a scalar and an empty slice
// for a list — there is no distinct "set but empty" state, because §15 has no
// field for which the empty string is a legal value.
//
// Absent by construction, and deliberately not fields here: kind, id, parent,
// and locator, all of which come from the path; and updated, current,
// progress, pace, derived status, and attention, all of which are computed
// (§2.5, §8.3).
type State struct {
	Name        string
	Description string
	Status      string
	Priority    string
	Due         string
	Tags        []string
	Created     string
	Type        string
	Start       string
	Target      string
	Scope       []string
}

// Field returns the scalar field named f, or "" if f is a list field or is
// absent. It exists so set/unset and --sort can address a state by §15's one
// spelling per field rather than by struct member.
func (s State) Field(f kindmeta.Field) string {
	switch f {
	case kindmeta.FieldName:
		return s.Name
	case kindmeta.FieldDescription:
		return s.Description
	case kindmeta.FieldStatus:
		return s.Status
	case kindmeta.FieldPriority:
		return s.Priority
	case kindmeta.FieldDue:
		return s.Due
	case kindmeta.FieldCreated:
		return s.Created
	case kindmeta.FieldType:
		return s.Type
	case kindmeta.FieldStart:
		return s.Start
	case kindmeta.FieldTarget:
		return s.Target
	default:
		return ""
	}
}

// SetField sets the scalar field named f. A list field (tags, scope) is
// ignored — those are set directly, since a comma-joined string would be a
// second grammar for the same value.
func (s *State) SetField(f kindmeta.Field, v string) {
	switch f {
	case kindmeta.FieldName:
		s.Name = v
	case kindmeta.FieldDescription:
		s.Description = v
	case kindmeta.FieldStatus:
		s.Status = v
	case kindmeta.FieldPriority:
		s.Priority = v
	case kindmeta.FieldDue:
		s.Due = v
	case kindmeta.FieldCreated:
		s.Created = v
	case kindmeta.FieldType:
		s.Type = v
	case kindmeta.FieldStart:
		s.Start = v
	case kindmeta.FieldTarget:
		s.Target = v
	}
}

// List returns the list field named f, or nil if f is not a list field.
func (s State) List(f kindmeta.Field) []string {
	switch f {
	case kindmeta.FieldTags:
		return s.Tags
	case kindmeta.FieldScope:
		return s.Scope
	default:
		return nil
	}
}

// EncodeState renders s in §15's field order, omitting every absent field.
// Two calls over the same State always produce identical bytes (§0.2).
func EncodeState(s State) ([]byte, error) {
	var fields []ptoml.Field
	for _, f := range kindmeta.AllFields() {
		switch f {
		case kindmeta.FieldTags, kindmeta.FieldScope:
			if list := s.List(f); len(list) > 0 {
				fields = append(fields, ptoml.Field{Key: string(f), Value: ptoml.StringArray(list)})
			}
		default:
			if v := s.Field(f); v != "" {
				fields = append(fields, ptoml.Field{Key: string(f), Value: ptoml.String(v)})
			}
		}
	}
	return ptoml.Encode(fields)
}

// DecodeState parses one .para/state.toml. Keys para does not recognise are
// ignored rather than rejected: an unknown key is doctor's `invalid` finding
// to report (§10), not a reason for every read of the file to fail.
func DecodeState(data []byte) (State, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return State{}, err
	}
	var s State
	for _, f := range kindmeta.AllFields() {
		switch f {
		case kindmeta.FieldTags:
			s.Tags, _ = doc.StringArray(string(f))
		case kindmeta.FieldScope:
			s.Scope, _ = doc.StringArray(string(f))
		default:
			if v, ok := doc.String(string(f)); ok {
				s.SetField(f, v)
			}
		}
	}
	return s, nil
}

// Tree is .para/tree.toml: schema version and tree identity, and nothing else
// (§8.1). The root has no state.toml because the root is not an entity — it is
// the tree — and its README frontmatter generates from here.
type Tree struct {
	Schema      int64
	ParaVersion string
	Name        string
	Description string
	Created     string
}

// treeKeys is tree.toml's declared field order, as §8.1 writes it.
const (
	treeKeySchema      = "schema"
	treeKeyParaVersion = "para-version"
	treeKeyName        = "name"
	treeKeyDescription = "description"
	treeKeyCreated     = "created"
)

// EncodeTree renders t in §8.1's declared key order, omitting absent fields.
func EncodeTree(t Tree) ([]byte, error) {
	var fields []ptoml.Field
	if t.Schema != 0 {
		fields = append(fields, ptoml.Field{Key: treeKeySchema, Value: ptoml.Int64(t.Schema)})
	}
	for _, kv := range []struct {
		key string
		val string
	}{
		{treeKeyParaVersion, t.ParaVersion},
		{treeKeyName, t.Name},
		{treeKeyDescription, t.Description},
		{treeKeyCreated, t.Created},
	} {
		if kv.val != "" {
			fields = append(fields, ptoml.Field{Key: kv.key, Value: ptoml.String(kv.val)})
		}
	}
	return ptoml.Encode(fields)
}

// DecodeTree parses .para/tree.toml, ignoring unrecognised keys for the same
// reason DecodeState does.
func DecodeTree(data []byte) (Tree, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return Tree{}, err
	}
	var t Tree
	t.Schema, _ = doc.Int64(treeKeySchema)
	t.ParaVersion, _ = doc.String(treeKeyParaVersion)
	t.Name, _ = doc.String(treeKeyName)
	t.Description, _ = doc.String(treeKeyDescription)
	t.Created, _ = doc.String(treeKeyCreated)
	return t, nil
}

// StatePath is dir's state.toml, the uniform filename every .para/ holds
// (§8.4).
func StatePath(dir string) string { return filepath.Join(dir, ".para", "state.toml") }

// ConfigPath is dir's config.toml, the other uniform filename (§8.4).
func ConfigPath(dir string) string { return filepath.Join(dir, ".para", "config.toml") }

// TreePath is dir's tree.toml — the root marker, and the single non-uniform
// truth filename (§8.1, §8.4).
func TreePath(dir string) string { return filepath.Join(dir, ".para", "tree.toml") }

// LogsDir is dir's journal directory (§3.1).
func LogsDir(dir string) string { return filepath.Join(dir, ".para", "logs") }

// ReadState reads and parses dir/.para/state.toml.
func ReadState(dir string) (State, error) {
	data, err := readFile(StatePath(dir))
	if err != nil {
		return State{}, err
	}
	s, err := DecodeState(data)
	if err != nil {
		return State{}, paraerr.Wrap(paraerr.KindValidation, err, fmt.Sprintf("reading %s", StatePath(dir)))
	}
	return s, nil
}

// ReadTree reads and parses dir/.para/tree.toml.
func ReadTree(dir string) (Tree, error) {
	data, err := readFile(TreePath(dir))
	if err != nil {
		return Tree{}, err
	}
	t, err := DecodeTree(data)
	if err != nil {
		return Tree{}, paraerr.Wrap(paraerr.KindValidation, err, fmt.Sprintf("reading %s", TreePath(dir)))
	}
	return t, nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", path)
		}
		return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", path))
	}
	return data, nil
}
