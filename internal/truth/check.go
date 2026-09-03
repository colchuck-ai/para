package truth

import (
	"errors"
	"fmt"
	"time"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/ptoml"
	"github.com/colchuck-ai/para/internal/tagexpr"
)

// Schema is the .para/tree.toml schema version this para reads and writes
// (§8.1). It lives here because tree.toml's codec is here and because "schema
// and para-version must be readable before anything else is parsed" makes the
// number a property of the format rather than of any one command.
const Schema int64 = 1

// Problem is one rule a stored truth file breaks — §10's `invalid` finding,
// as data.
//
// It is a value rather than an error because a broken state.toml usually
// breaks more than one rule at once, and doctor reports findings rather than
// failing at the first: a file missing both `name` and `target` should say so
// in one pass, not over two repairs.
type Problem struct {
	// Field is the §15 field at fault, or "" for a rule about the state as a
	// whole and for a tree.toml key, which is not a §15 field at all.
	Field kindmeta.Field
	// Msg states the breach, without naming the file — the caller knows which
	// file it handed over and doctor prints the path itself.
	Msg string
}

func (p Problem) String() string {
	if p.Field == "" {
		return p.Msg
	}
	return string(p.Field) + ": " + p.Msg
}

// Check reports every §15 rule the stored state breaks, in §15's own row order
// so that two runs over one file name the same problem first (§0.2).
//
// It is the read-side counterpart of the validation `add` and `set` apply to
// what you type, and the rules themselves are shared rather than restated:
// which statuses a kind admits is kindmeta's, whether a reading fits a type is
// krvalue's, whether a timestamp parses is ptime's. What is here is the
// mapping from §15's table to those rules, once, so that a new row in the
// table cannot be enforced on the write path and forgotten on the read one.
//
// What it deliberately does not check is anything outside the file: whether a
// scope entry resolves is doctor's `scope-unresolved` (§5.4), and whether a
// `created` postdates *now* is a question about the clock rather than about
// the state — doctor asks it, because a Check that read the clock could not be
// a pure function of a file.
func Check(kind kindmeta.Kind, s State) []Problem {
	var out []Problem
	add := func(field kindmeta.Field, format string, args ...any) {
		out = append(out, Problem{Field: field, Msg: fmt.Sprintf(format, args...)})
	}

	for _, field := range kindmeta.AllFields() {
		present := s.Field(field) != "" || len(s.List(field)) > 0
		req := kindmeta.Requirement(kind, field)

		if req == kindmeta.NotApplicable {
			if present {
				add(field, "%s has no %s field", kind, field)
			}
			continue
		}
		if !present {
			if req == kindmeta.Required || req == kindmeta.RequiredFixed {
				add(field, "%s is required on %s and is missing", field, kind)
			}
			continue
		}
		if msg := checkValue(kind, field, s); msg != "" {
			add(field, "%s", msg)
		}
	}

	// kind is judged here rather than in the loop above because it is not a
	// §15 field (§30): no kindmeta.Field names it, which is what keeps it out
	// of reach of set and unset, so Requirement has nothing to say about it.
	// It is reported against no field, the way every other rule about the
	// state as a whole is, and its message is terse for the same reason every
	// checkValue message is — the vocabulary belongs in a CLI refusal, where
	// it helps, not in a doctor finding, where it is noise.
	//
	// Only a *present* value is judged, and two rules that look like this
	// one's business are deliberately not here:
	//
	//   - Absence is §10's `no-kind`, not `invalid`, and it is a question
	//     about the tree rather than the file: it is a finding only in a
	//     `schema = 2` tree, and Check cannot see the schema. truth.Schema is
	//     still 1 as of this comment, so no such tree exists yet — para-sxt.7
	//     bumps it and owns that finding.
	//   - Whether a stored kind is *legal where it sits* is §10's `misplaced`
	//     (§30.4). That needs §30.2's containment table and the parent's own
	//     kind, neither of which a single state file carries. para-sxt.9.
	if s.Kind != "" {
		if _, err := kindmeta.ParseKind(s.Kind); err != nil {
			add("", "kind: %q is not a kind", s.Kind)
		}
	}

	// The rules that judge the state as a whole, which cannot be decided one
	// field at a time because one command may set both sides of the
	// comparison. They are reported against the field a repair would edit.
	if err := CheckCreatedNotAfterDue(s); err != nil {
		add(kindmeta.FieldCreated, "%s", message(err))
	}
	if err := CheckBounds(kind, s); err != nil {
		add(kindmeta.FieldStart, "%s", message(err))
	}
	return out
}

// checkValue judges one present field's stored value, and returns "" when it
// is sound.
func checkValue(kind kindmeta.Kind, field kindmeta.Field, s State) string {
	switch field {
	case kindmeta.FieldStatus:
		if !kindmeta.IsStatus(kind, s.Status) {
			legal := kindmeta.SettableStatuses(kind)
			if len(legal) == 0 {
				return fmt.Sprintf("%q is stored where %s has no status", s.Status, kind)
			}
			return fmt.Sprintf("%q is not a %s status", s.Status, kind)
		}
	case kindmeta.FieldPriority:
		if _, ok := kindmeta.PriorityRank(s.Priority); !ok {
			return fmt.Sprintf("%q is not a priority", s.Priority)
		}
	case kindmeta.FieldType:
		// para-6g7: type's meaning depends on the kind (kindmeta.
		// TypeIsMeasurement), the same way status's vocabulary already
		// does — a key-result's type is §4.1's closed measurement grammar, a
		// link's is an opaque tag para never interprets, so it needs no
		// shape check beyond the presence Check's own required-field loop
		// already gives it.
		if kindmeta.TypeIsMeasurement(kind) {
			if _, ok := legalType(s.Type); !ok {
				return fmt.Sprintf("%q is not a key-result type", s.Type)
			}
		}
	case kindmeta.FieldDirection:
		if !kindmeta.IsLinkDirection(s.Direction) {
			return fmt.Sprintf("%q is not a link direction", s.Direction)
		}
	case kindmeta.FieldStart, kindmeta.FieldTarget:
		// A value whose shape contradicts its key-result's type (§10). A type
		// that is itself unreadable is reported on its own row and says
		// nothing further here: every reading would fail against it, which
		// would be one broken field reported three times.
		typ, ok := legalType(s.Type)
		if !ok {
			if s.Type == "" {
				return fmt.Sprintf("%s is stored with no type to read it by", field)
			}
			return ""
		}
		if _, err := krvalue.Parse(typ, s.Field(field)); err != nil {
			return message(err)
		}
	case kindmeta.FieldCreated:
		if _, err := ptime.ParseAt(s.Created, time.UTC); err != nil {
			return message(err)
		}
	case kindmeta.FieldDue:
		if _, err := ptime.Deadline(s.Due); err != nil {
			return message(err)
		}
	case kindmeta.FieldTags:
		for _, tag := range s.Tags {
			if tagexpr.IsReserved(tag) {
				return fmt.Sprintf("%q cannot be a tag: it is an operator in a --tags expression", tag)
			}
			if !tagexpr.ValidTag(tag) {
				return fmt.Sprintf("%q is not a valid tag", tag)
			}
		}
	case kindmeta.FieldScope:
		for _, entry := range s.Scope {
			if _, err := address.ParseDotted(entry); err != nil {
				return fmt.Sprintf("scope entry %q: %s", entry, message(err))
			}
		}
	}
	return ""
}

// legalType reads a stored `type` as one of §4.1's three, reporting false for
// anything else — including the empty string, which is a missing required
// field rather than a wrong one.
func legalType(raw string) (krvalue.Type, bool) {
	switch typ := krvalue.Type(raw); typ {
	case krvalue.TypeNumber, krvalue.TypeRatio, krvalue.TypeBoolean:
		return typ, true
	default:
		return "", false
	}
}

// CheckCreatedNotAfterDue is §15's bound between the two stored timestamps: a
// thing cannot have been created after the deadline it was given.
//
// It is exported, and takes a whole State rather than two strings, because the
// write path applies the same rule to the state it is about to write (§18.1,
// §18.2) — the rule is one function with two callers rather than one rule
// spelled twice, which is what keeps a `set` and a `doctor` from disagreeing
// about the same pair of values.
func CheckCreatedNotAfterDue(s State) error {
	if s.Created == "" || s.Due == "" {
		return nil
	}
	created, err := ptime.ParseAt(s.Created, time.UTC)
	if err != nil {
		// The field's own row already reports this; a bound cannot be judged
		// against a value that will not parse.
		return nil
	}
	due, err := ptime.Deadline(s.Due)
	if err != nil {
		return nil
	}
	return ptime.CheckNotAfterDue(created, due)
}

// CheckBounds is §4.1's rule about a key-result's two endpoints — the same one
// `add --start --target` applies, for the same reason CheckCreatedNotAfterDue
// is shared.
func CheckBounds(kind kindmeta.Kind, s State) error {
	if kind != kindmeta.KindKeyResult || s.Type == "" || s.Target == "" {
		return nil
	}
	typ := krvalue.Type(s.Type)
	target, err := krvalue.Parse(typ, s.Target)
	if err != nil {
		return nil
	}
	var start *krvalue.Value
	if s.Start != "" {
		v, err := krvalue.Parse(typ, s.Start)
		if err != nil {
			return nil
		}
		start = &v
	}
	return krvalue.ValidateBounds(typ, start, target)
}

// UnknownStateKeys returns the keys in a state.toml's bytes that §15 has no
// field for, in the order they appear.
//
// DecodeState ignores them rather than refusing the file, so that one stray key
// does not make every read of an entity fail — but ignoring is not the same as
// accepting, and this is where the difference is paid: an unknown key is either
// a typo that silently does nothing (`descripton`) or a field written by a para
// newer than this one, and both are worth a word from doctor (§10's `invalid`).
//
// `attention` and `suppression.until`/`suppression.note` are known here too
// (§28.4): they are not §15 fields — no kindmeta.Field names them — but they
// are a legitimate part of state.toml's schema as of §28, generated rather
// than typed, and flagging them as unrecognised would make every suppressed
// or attention-cached entity read as `invalid`.
func UnknownStateKeys(data []byte) ([]string, error) {
	known := make(map[string]bool, len(kindmeta.AllFields())+4)
	for _, f := range kindmeta.AllFields() {
		known[string(f)] = true
	}
	// kind is known here for the same reason attention is: not a §15 field, no
	// kindmeta.Field names it, and yet a legitimate part of state.toml's
	// schema as of §30 — flagging it would make every entity in a migrated
	// tree read as `invalid`. Whether its *value* is legal is Check's
	// question; this function only asks whether the key belongs.
	known[stateKeyKind] = true
	known[stateKeyAttention] = true
	known[suppressionTableKey+"."+suppressionKeyUntil] = true
	known[suppressionTableKey+"."+suppressionKeyNote] = true
	return unknownKeys(data, known)
}

// MistypedStateKeys reports the keys that belong in a state.toml but hold a
// value of the wrong TOML type — `kind = 5`, `name = true`, `tags = "growth"`
// — in the order they are written on the way out (kind, then §15's rows, then
// attention).
//
// It exists because UnknownStateKeys cannot see them and DecodeState will not
// complain: every typed accessor on ptoml.Document reports a type mismatch as
// *absence*, so `kind = 5` decodes to no kind at all and the whole file reads
// clean. That was survivable while `kind` was an unknown key — the unknown-key
// check caught it — and stopped being survivable the moment §30 made it a
// known one, which is the coverage this restores.
//
// It reports keys rather than values because the repair is the same whatever
// the wrong type was: write the value the key's row in §15 calls for.
// [suppression]'s two members are not checked here; a mistyped one is the
// same class of hole and wants the same fix, tracked separately rather than
// half-solved.
func MistypedStateKeys(data []byte) ([]string, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return nil, err
	}
	// Presence comes from Keys, not from Value. ptoml.Value reports ok=false
	// for every TOML type it has no Value case for — dates, datetimes, arrays
	// of anything but strings — so probing presence with it would classify a
	// wrongly-typed value of those types as *absent*, which is the exact
	// conflation this function exists to undo. Keys enumerates what the
	// document actually holds, whatever its type.
	keys, err := doc.Keys()
	if err != nil {
		return nil, err
	}
	written := make(map[string]bool, len(keys))
	for _, k := range keys {
		written[k] = true
	}

	var out []string
	present := func(key string) bool { return written[key] }
	isString := func(key string) bool {
		_, ok := doc.String(key)
		return ok
	}
	isStringArray := func(key string) bool {
		_, ok := doc.StringArray(key)
		return ok
	}
	check := func(key string, wellTyped func(string) bool) {
		if present(key) && !wellTyped(key) {
			out = append(out, key)
		}
	}

	check(stateKeyKind, isString)
	for _, f := range kindmeta.AllFields() {
		switch f {
		case kindmeta.FieldTags, kindmeta.FieldScope:
			check(string(f), isStringArray)
		default:
			check(string(f), isString)
		}
	}
	check(stateKeyAttention, isString)
	return out, nil
}

// UnknownTreeKeys is UnknownStateKeys for .para/tree.toml, whose keys are
// §8.1's five rather than §15's eleven.
func UnknownTreeKeys(data []byte) ([]string, error) {
	known := map[string]bool{
		treeKeySchema: true, treeKeyParaVersion: true,
		treeKeyName: true, treeKeyDescription: true, treeKeyCreated: true,
	}
	return unknownKeys(data, known)
}

func unknownKeys(data []byte, known map[string]bool) ([]string, error) {
	doc, err := ptoml.Decode(data)
	if err != nil {
		return nil, err
	}
	keys, err := doc.Keys()
	if err != nil {
		return nil, err
	}
	var out []string
	for _, key := range keys {
		if !known[key] {
			out = append(out, key)
		}
	}
	return out, nil
}

// CheckTree reports the rules .para/tree.toml breaks (§8.1). The root's
// identity lives here rather than in a state.toml, so its required keys are
// its own and §15's field matrix says nothing about them.
func CheckTree(t Tree) []Problem {
	var out []Problem
	add := func(format string, args ...any) {
		out = append(out, Problem{Msg: fmt.Sprintf(format, args...)})
	}

	switch {
	case t.Schema == 0:
		add("schema is required and is missing")
	case t.Schema != Schema:
		// A tree written by a newer para is not a tree this one may rewrite:
		// §8.1 puts schema first precisely so that it is readable before
		// anything else is parsed.
		add("schema %d is not the schema this para reads (%d)", t.Schema, Schema)
	}
	if t.Name == "" {
		add("name is required and is missing")
	}
	if t.Created != "" {
		if _, err := ptime.ParseAt(t.Created, time.UTC); err != nil {
			add("created: %s", message(err))
		}
	}
	return out
}

// message is a nested error's own sentence, for embedding in one that supplies
// its own context.
func message(err error) string {
	var pe *paraerr.Error
	if errors.As(err, &pe) {
		return pe.Msg
	}
	return err.Error()
}
