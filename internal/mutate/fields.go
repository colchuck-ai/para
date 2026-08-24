package mutate

import (
	"errors"
	"strings"
	"time"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/tagexpr"
	"github.com/colchuck-ai/para/internal/truth"
)

// Fields is the set of field values one `add` or `set` was given, addressed by
// §15's one spelling per field — the same spelling the flag, the truth file,
// and the journal's `field` all use.
//
// It distinguishes "not given" from "given empty", which a plain State cannot:
// `set --description ""` is a value, `set --status done` alone leaves
// description untouched, and only the caller's flag parser knows which
// happened.
type Fields struct {
	scalars map[kindmeta.Field]string
	lists   map[kindmeta.Field][]string
}

// Set records a scalar field's value.
func (f *Fields) Set(field kindmeta.Field, value string) {
	if f.scalars == nil {
		f.scalars = map[kindmeta.Field]string{}
	}
	f.scalars[field] = value
}

// SetList records a list field's value. §15 has two — tags and scope — and
// both replace wholly rather than accumulating.
func (f *Fields) SetList(field kindmeta.Field, values []string) {
	if f.lists == nil {
		f.lists = map[kindmeta.Field][]string{}
	}
	f.lists[field] = values
}

// Has reports whether the field was given at all.
func (f Fields) Has(field kindmeta.Field) bool {
	_, scalar := f.scalars[field]
	_, list := f.lists[field]
	return scalar || list
}

// Names returns the fields given, in §15's row order. The order is the table's
// rather than the command line's so that a multi-field `set` reports and logs
// its changes in one declared order — never a map's (§0.2).
func (f Fields) Names() []kindmeta.Field {
	var out []kindmeta.Field
	for _, field := range kindmeta.AllFields() {
		if f.Has(field) {
			out = append(out, field)
		}
	}
	return out
}

// Len is how many fields were given.
func (f Fields) Len() int { return len(f.scalars) + len(f.lists) }

// Scalar returns the value given for a scalar field, and whether it was given
// at all. It exists so no caller has to reach past this type into its map.
func (f Fields) Scalar(field kindmeta.Field) (string, bool) {
	v, ok := f.scalars[field]
	return v, ok
}

// IsListField reports whether a field is one of §15's two list-valued fields —
// tags and scope, the only two whose value is a list rather than a scalar.
//
// It is exported because a caller parsing a command line has to know which
// setter to reach for, and asking §15 is better than a second copy of the
// answer.
func IsListField(field kindmeta.Field) bool {
	return field == kindmeta.FieldTags || field == kindmeta.FieldScope
}

// isList is the unexported spelling this package uses.
func isList(field kindmeta.Field) bool { return IsListField(field) }

// resolve applies the given fields onto cur and returns the state that results,
// with every value normalised to the form it is stored in.
//
// Validation happens in two passes because §15's rules are of two kinds. Most
// are about a single value — a status a kind does not have, a tag that is
// really an operator — and are checked as the value is normalised. The rest are
// about the state as a whole — `created` may not postdate `due`, `start` may not
// equal `target` — and cannot be judged until every field given in the same
// command has landed, since one command may set both sides of the comparison.
func (e *Env) resolve(kind kindmeta.Kind, cur truth.State, f Fields) (truth.State, error) {
	next := cur

	for _, field := range f.Names() {
		if !kindmeta.Has(kind, field) {
			return next, unknownField(kind, field)
		}
		if isList(field) {
			values, err := normaliseList(field, f.lists[field])
			if err != nil {
				return next, err
			}
			setList(&next, field, values)
			continue
		}
		// `type` decides how start and target parse, so it is read from the
		// state being built rather than from the state on disk — one command
		// may create both.
		value, err := e.normalise(kind, field, f.scalars[field], next)
		if err != nil {
			return next, err
		}
		next.SetField(field, value)
	}

	if err := e.checkState(kind, next); err != nil {
		return next, err
	}
	return next, nil
}

// normalise validates one scalar value against §15 and returns the string that
// gets stored.
func (e *Env) normalise(kind kindmeta.Kind, field kindmeta.Field, raw string, st truth.State) (string, error) {
	if raw == "" {
		return "", paraerr.Newf(paraerr.KindValidation, "%s cannot be empty — use `para unset` to remove it", field)
	}

	switch field {
	case kindmeta.FieldStatus:
		if !kindmeta.IsStatus(kind, raw) {
			legal := kindmeta.SettableStatuses(kind)
			if len(legal) == 0 {
				return "", paraerr.Newf(paraerr.KindValidation, "%s has no status", article(kind.String()))
			}
			return "", paraerr.Newf(paraerr.KindValidation,
				"%q is not a %s status — one of %s", raw, kind, strings.Join(legal, ", "))
		}
		return raw, nil

	case kindmeta.FieldPriority:
		if _, ok := kindmeta.PriorityRank(raw); !ok {
			return "", paraerr.Newf(paraerr.KindValidation,
				"%q is not a priority — one of %s", raw, strings.Join(kindmeta.Priorities(), ", "))
		}
		return raw, nil

	case kindmeta.FieldCreated:
		t, err := e.timestamp(field, raw)
		if err != nil {
			return "", err
		}
		if err := ptime.CheckNotFuture(t, e.Now); err != nil {
			return "", paraerr.Newf(paraerr.KindValidation, "created %s", err)
		}
		return stamp(t), nil

	case kindmeta.FieldDue:
		// Validated, then stored exactly as typed — §8.3's truth files write
		// `due = "2026-09-30"` and §16.1 prints it back the same way.
		//
		// This is the one timestamp field that is *not* resolved to a UTC
		// instant, and the asymmetry with `created` is the point. §15.1's
		// store-as-UTC rule is about instants: something happened, and every
		// reader must agree when. A deadline is a date somebody chose, and
		// zero-filling it in the typist's offset would make the stored value
		// depend on where they were sitting — `2026-09-30` becoming
		// `2026-09-30T07:00:00Z` in summer and `T08:00:00Z` in winter, for a
		// deadline that meant neither. A date on disk means the same day to
		// every reader already, which is what §3.5 was asking for.
		//
		// A deadline is also the one timestamp that is *supposed* to be in the
		// future, so it takes neither of §15's bounds directly; `created` is
		// checked against it in checkState.
		//
		// Validated through ptime.Deadline, not e.timestamp/ParseAt: a year and
		// a year-month are due's own coarser precisions (para-xbb), which §15.1's
		// floor-at-a-full-date grammar does not admit — checkState calls
		// ptime.Deadline too, so gating here on the narrower ParseAt would
		// refuse a value checkState would otherwise accept, which is exactly
		// the bug (a `--due 2027` that ptime.Deadline itself parses fine).
		if _, err := ptime.Deadline(raw); err != nil {
			return "", paraerr.Newf(paraerr.KindValidation, "%s: %s", field, unwrapMessage(err))
		}
		return strings.TrimSpace(raw), nil

	case kindmeta.FieldType:
		if krvalue.IsType(raw) {
			return raw, nil
		}
		return "", paraerr.Newf(paraerr.KindValidation,
			"%q is not a key-result type — one of %s", raw, strings.Join(krvalue.TypeNames(), ", "))

	case kindmeta.FieldStart, kindmeta.FieldTarget:
		typ := krvalue.Type(st.Type)
		if typ == "" {
			return "", paraerr.Newf(paraerr.KindValidation, "--%s needs a --type to say how to read it", field)
		}
		v, err := krvalue.Parse(typ, raw)
		if err != nil {
			return "", err
		}
		// Stored exactly as written: a ratio's denominator belongs to the
		// reading and must survive as typed (§4.1).
		return v.Raw, nil

	default:
		// name and description are free prose. They are flattened because they
		// reach one-line formats — README frontmatter, a rule's routing
		// sentence, `list` output — where a raw newline breaks the structure.
		return flatten(raw), nil
	}
}

// timestamp parses a §15.1 value, zero-filling in the local offset and storing
// UTC: "what you type is local; what is stored is UTC".
func (e *Env) timestamp(field kindmeta.Field, raw string) (time.Time, error) {
	t, err := ptime.ParseAt(raw, e.Local())
	if err != nil {
		return time.Time{}, paraerr.Newf(paraerr.KindValidation, "%s: %s", field, unwrapMessage(err))
	}
	return t, nil
}

// stamp is the one spelling a timestamp is stored in: RFC 3339, in UTC.
//
// §15.1 fixes it for `created`, and `due` follows for the same reason §3.5
// gives: one representation on disk means one answer to "which day is this" for
// every reader, and no comparison anywhere has to reason about two offsets.
//
// It delegates to ptime.Stamp rather than formatting again, so that doctor's
// stale-projection check and rebuild's backfill (§28.4) — neither of which can
// import this package — compare against the identical formatting this package
// writes.
func stamp(t time.Time) string { return ptime.Stamp(t) }

// eventTime resolves an event's instant: --at if given, in §15.1's progressive
// precision zero-filled in the local offset, else now. Never in the future.
func (e *Env) eventTime(at string) (time.Time, error) {
	if at == "" {
		return e.Now.UTC(), nil
	}
	t, err := ptime.ParseAt(at, e.Local())
	if err != nil {
		return time.Time{}, err
	}
	if err := ptime.CheckNotFuture(t, e.Now); err != nil {
		return time.Time{}, err
	}
	// To the second, for the reason NewEnv gives: a fractional instant is
	// unique to §3.1's collision check and invisible in every file it reaches.
	return t.UTC().Truncate(time.Second), nil
}

// normaliseList validates a tags or scope list.
func normaliseList(field kindmeta.Field, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		switch field {
		case kindmeta.FieldTags:
			if tagexpr.IsReserved(v) {
				return nil, paraerr.Newf(paraerr.KindValidation,
					"%q cannot be a tag: it is an operator in a --tags expression", v)
			}
			if !tagexpr.ValidTag(v) {
				return nil, paraerr.Newf(paraerr.KindValidation, "%q is not a valid tag (letters, digits, and hyphens)", v)
			}
		case kindmeta.FieldScope:
			// A scope entry is a dotted address — the same string every
			// command prints (§5.2, R24). Whether it resolves is deliberately
			// not checked here: an entry may name something that has not been
			// created yet, and an entry that names nothing is doctor's
			// `scope-unresolved` finding to report (§5.4, §10). This is the
			// write-side half of truth.Check's FieldScope rule
			// (internal/truth/check.go) — the same rule, so `add`/`set` and
			// `doctor` cannot disagree about one entry.
			addr, err := address.ParseDotted(v)
			if err != nil {
				return nil, paraerr.Newf(paraerr.KindValidation, "scope entry %q: %s", v, unwrapMessage(err))
			}
			v = addr.String()
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, paraerr.Newf(paraerr.KindValidation, "%s cannot be empty — use `para unset` to remove it", field)
	}
	return out, nil
}

func setList(st *truth.State, field kindmeta.Field, values []string) {
	if field == kindmeta.FieldScope {
		st.Scope = values
		return
	}
	st.Tags = values
}

// checkState runs the rules that judge a state as a whole rather than one value
// at a time.
//
// The rules themselves are `truth`'s, because doctor applies the same two to
// stored truth (§10's `invalid`): a state a `set` accepts and a `doctor` then
// faults would be para disagreeing with itself about one pair of values, and
// the only way two callers cannot disagree is for there to be one rule. What
// stays here is this path's own posture — it refuses where doctor reports, so
// the parse failures below are errors rather than a separate finding, and the
// stored value they refuse to write against is named.
func (e *Env) checkState(kind kindmeta.Kind, st truth.State) error {
	if st.Created != "" && st.Due != "" {
		if _, err := ptime.ParseAt(st.Created, time.UTC); err != nil {
			return paraerr.Newf(paraerr.KindValidation, "created: %s", unwrapMessage(err))
		}
		if _, err := ptime.Deadline(st.Due); err != nil {
			return paraerr.Newf(paraerr.KindValidation, "due: %s", unwrapMessage(err))
		}
	}
	if err := truth.CheckCreatedNotAfterDue(st); err != nil {
		return paraerr.Newf(paraerr.KindValidation, "created %s", unwrapMessage(err))
	}
	return truth.CheckBounds(kind, st)
}

// diff reports the fields that actually changed between two states, in §15's
// row order — the fields that get an event each (§18.2) and a line each in the
// output.
func diff(kind kindmeta.Kind, before, after truth.State) []Change {
	var out []Change
	for _, field := range kindmeta.AllFields() {
		if !kindmeta.Has(kind, field) {
			continue
		}
		from, to := display(before, field), display(after, field)
		if from != to {
			out = append(out, Change{Field: field, From: from, To: to})
		}
	}
	return out
}

// display is a field's value as one string — the spelling a journal's from/to
// carries and the output prints. A list joins on ", " because that is how §22
// and §16.1 already print one.
func display(st truth.State, field kindmeta.Field) string {
	if isList(field) {
		return strings.Join(st.List(field), ", ")
	}
	return st.Field(field)
}

func unknownField(kind kindmeta.Kind, field kindmeta.Field) error {
	return paraerr.Newf(paraerr.KindValidation, "%s has no %s field", article(kind.String()), field)
}

// article prefixes a kind with the right indefinite article, since "a area"
// and "a objective" read as typos in an error message a user is already
// annoyed by.
func article(noun string) string {
	if strings.ContainsRune("aeiou", rune(noun[0])) {
		return "an " + noun
	}
	return "a " + noun
}

// unwrapMessage is a nested error's own sentence, for embedding in a sentence
// that supplies its own context.
func unwrapMessage(err error) string {
	var pe *paraerr.Error
	if errors.As(err, &pe) {
		return pe.Msg
	}
	return err.Error()
}

// flatten collapses runs of whitespace, including the newlines a shell
// here-doc can carry, and trims the ends.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }
