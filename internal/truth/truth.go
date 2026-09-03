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
// Field order on write is `kind` (§8.3, §30) followed by §15's row order —
// kindmeta.AllFields() — for every kind, filtered to the fields actually
// present. One declared order rather than one per kind, because §8.4's
// argument for uniform filenames applies just as well here: a per-kind order
// would be a second copy of the kind, which the file now states outright.
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
// Present: kind (§8.3, §30), which was in the list below until §1.3 stopped
// deriving a kind from a position.
//
// §30 makes it the only copy. It is not yet: nothing writes it, and every
// consumer still takes its kind from kindmeta.KindOf's path derivation, so
// today the field is only what a human or a migration puts there
// (para-sxt.4, para-sxt.7, para-sxt.10).
//
// Absent by construction, and deliberately not fields here: id, parent, and
// locator, all of which come from the path; and updated, current, progress,
// pace, and derived status, all of which are computed (§2.5, §8.3) and never
// written anywhere.
//
// Attention and Suppression are also computed rather than typed, but as of
// §28.4 they are the exception: present in the file once written, so that
// review and show can read a cached answer instead of re-scanning the
// journal. Neither is a kindmeta.Field, which is what keeps them unreachable
// from set/unset: State.Field/SetField's kindmeta.Field switch has no case
// for either, so they are addressable only as ordinary Go struct fields, and
// the only writer that is meant to touch them is the shared
// attention/suppression derivation a later phase adds (§28.4's "one
// function") — nothing in this package writes either field itself. An empty
// Attention or a zero Suppression means "not yet cached," never "false" or
// "zero" (§28.4) — every read path must fall back to a live journal
// derivation when it finds either absent.
type State struct {
	// Kind is state.toml's `kind` key (§8.3, §30) — the only place a
	// directory's kind is written down, now that §1.3 no longer derives it
	// from the position.
	//
	// It is a string rather than a kindmeta.Kind for the reason Status,
	// Priority, Type and Direction are: a closed vocabulary is stored as
	// written and judged by Check, so a hand-edited value outside the
	// vocabulary survives a rewrite as the visible error it is (§10's
	// `invalid`) instead of being silently normalised or dropped. A
	// kindmeta.Kind has no representation for "banana" other than
	// KindUnknown, so decoding into one would make EncodeState normalise or
	// discard the very value doctor needs to name. Parsing itself is not the
	// problem — Check parses on every read — losing the original is.
	//
	// It is not a kindmeta.Field, which is what puts it out of reach of set
	// and unset — the same device Attention and Suppression use. There is no
	// `--kind` flag: an address carries a noun, and the noun is the kind
	// (§0 principle 1).
	Kind        string
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
	Ref         string
	Direction   string
	Attention   string
	Suppression Suppression
}

// Suppression is state.toml's `[suppression]` table (§28.4): the newest
// suppress event's fields, cached. Until takes due's progressive precision
// (§15.1) rather than a resolved UTC instant — the same reason `due` itself
// is stored exactly as typed. The table's presence in the file is the signal
// (§28.4): the derivation this is meant for never writes one field without
// the other, so in ordinary use "no active suppression" is exactly "both
// empty," and the whole table is omitted rather than written with an empty
// until. EncodeState still preserves either field alone rather than
// discarding it, for the state a hand-edited file can be decoded into.
type Suppression struct {
	Until string
	Note  string
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
	case kindmeta.FieldRef:
		return s.Ref
	case kindmeta.FieldDirection:
		return s.Direction
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
	case kindmeta.FieldRef:
		s.Ref = v
	case kindmeta.FieldDirection:
		s.Direction = v
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

// stateKeyAttention is state.toml's cached-clock key (§28.4), written after
// every §15 field and before the [suppression] table — the same position
// §28.4's worked example shows it in, since it is generated into the same
// file the typed fields live in rather than a projection of its own.
const stateKeyAttention = "attention"

// stateKeyKind is state.toml's `kind` key (§8.3, §30), written before every
// §15 field.
//
// The parser does not depend on that order — ptoml parses the whole document
// and DecodeState fetches by name — so it is a choice about the
// reader, not the parser: §8.3's example leads with `kind`, and a human
// opening the file should learn what the thing is before reading fields whose
// meaning depends on it. A `type` and a `start` only mean something once you
// know you are looking at a key-result.
const stateKeyKind = "kind"

// suppressionTableKey and its two members are §28.4's `[suppression]` table.
// ptoml.Encode's Field.Key writes a literal dotted path rather than a TOML
// table header, so EncodeState composes the header by hand around a second
// ptoml.Encode call for the table's own two keys — the header itself carries
// no value ptoml's Field/Value grammar can express.
const (
	suppressionTableKey = "suppression"
	suppressionKeyUntil = "until"
	suppressionKeyNote  = "note"
)

// EncodeState renders s in §15's field order, omitting every absent field,
// followed by attention and [suppression] (§28.4) when either is cached. Two
// calls over the same State always produce identical bytes (§0.2).
func EncodeState(s State) ([]byte, error) {
	var fields []ptoml.Field
	// kind first (§8.3), and written exactly as it was handed over: an
	// unrecognised value is doctor's `invalid` to report (§10), and dropping
	// it here would turn a visible error into an absent key — a different and
	// quieter one — while also breaking this function's contract to lose
	// nothing it is given (§19), the same contract the [suppression] table
	// below is careful about.
	if s.Kind != "" {
		fields = append(fields, ptoml.Field{Key: stateKeyKind, Value: ptoml.String(s.Kind)})
	}
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
	if s.Attention != "" {
		fields = append(fields, ptoml.Field{Key: stateKeyAttention, Value: ptoml.String(s.Attention)})
	}
	data, err := ptoml.Encode(fields)
	if err != nil {
		return nil, err
	}

	// Presence of the table is the signal (§28.4): the derivation step never
	// writes one field without the other, so in the state every caller of
	// this function actually produces, "no active suppression" and "neither
	// field set" are the same thing. But EncodeState's own contract is to
	// lose nothing it is handed (§19's "nothing is corrupted and nothing is
	// lost" holds for every truth file, including one read back after a hand
	// edit): a State carrying a Note with no Until — reachable only by
	// decoding a hand-edited state.toml — still round-trips its Note rather
	// than silently discarding it the next time anything rewrites the file.
	if s.Suppression.Until == "" && s.Suppression.Note == "" {
		return data, nil
	}
	var tableFields []ptoml.Field
	if s.Suppression.Until != "" {
		tableFields = append(tableFields, ptoml.Field{Key: suppressionKeyUntil, Value: ptoml.String(s.Suppression.Until)})
	}
	if s.Suppression.Note != "" {
		tableFields = append(tableFields, ptoml.Field{Key: suppressionKeyNote, Value: ptoml.String(s.Suppression.Note)})
	}
	table, err := ptoml.Encode(tableFields)
	if err != nil {
		return nil, err
	}
	data = append(data, []byte("\n["+suppressionTableKey+"]\n")...)
	data = append(data, table...)
	return data, nil
}

// DecodeState parses one .para/state.toml. Keys para does not recognise are
// ignored rather than rejected: an unknown key is doctor's `invalid` finding
// to report (§10), not a reason for every read of the file to fail. A
// state.toml written before §28 has neither `attention` nor `[suppression]`
// at all, and that decodes to State's zero value for both — "not yet
// cached," which every caller must read as, never as false or zero (§28.4).
func DecodeState(data []byte) (State, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return State{}, err
	}
	var s State
	// An absent key leaves Kind "" — which is how a state.toml written before
	// §30 decodes, and is distinguishable from every legal value. Whether ""
	// is acceptable is Check's question, not this function's: decoding stays
	// lenient so that doctor can read a broken file in order to report it.
	if v, ok := doc.String(stateKeyKind); ok {
		s.Kind = v
	}
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
	if v, ok := doc.String(stateKeyAttention); ok {
		s.Attention = v
	}
	if v, ok := doc.String(suppressionTableKey + "." + suppressionKeyUntil); ok {
		s.Suppression.Until = v
	}
	if v, ok := doc.String(suppressionTableKey + "." + suppressionKeyNote); ok {
		s.Suppression.Note = v
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
