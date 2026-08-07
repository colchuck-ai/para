package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
)

// blockedStatus is the one status that may not be set without a reason (§18.2):
// "a blocker with no recorded reason is worthless in six months". The word
// itself is §1.7's, and lives with the rest of that vocabulary.
const blockedStatus = kindmeta.StatusBlocked

// Set writes any number of fields at once, one event per field changed, in one
// write-through pass (§18.2).
//
// Setting a field to the value it already holds writes nothing and exits 0 —
// no event, no projection rewrite, no clock movement (§15). Without that rule a
// shell loop buys permanent silence from every check in §20.
func (e *Env) Set(loc locator.Locator, f Fields, note string) (Result, error) {
	if f.Len() == 0 {
		return Result{}, paraerr.New(paraerr.KindUsage, "set needs at least one field")
	}
	subj, err := e.load(loc)
	if err != nil {
		return Result{}, err
	}

	for _, field := range f.Names() {
		if kindmeta.Requirement(subj.kind, field) == kindmeta.RequiredFixed {
			return Result{}, paraerr.Newf(paraerr.KindValidation,
				"%s is fixed at creation and can never be changed — delete and recreate", field)
		}
	}
	if status, given := f.Scalar(kindmeta.FieldStatus); given && status == blockedStatus && note == "" {
		// A domain rule about a value, not a rule about the shape of the
		// invocation, so it is KindValidation like every neighbouring field
		// rule — and it is checked before anything is compared, since the user
		// is asserting the status whether or not it turns out to have moved.
		return Result{}, paraerr.Newf(paraerr.KindValidation, "--note is required when setting status to %s", blockedStatus)
	}

	next, err := e.resolve(subj.kind, subj.state, f)
	if err != nil {
		return Result{}, err
	}
	return e.write(subj, f, next, note)
}

// Unset removes fields, which is a different act from setting them to nothing:
// `unset skills.x scope` widens a skill back to the whole tree (§5.2, §15), and
// `unset projects.x due` is how a deadline stops existing.
//
// Two fields refuse: `created`, because everything has a creation time, and
// `type`, which is fixed at creation. So does any field the kind requires —
// unsetting `name` would leave an entity that cannot render its own README.
func (e *Env) Unset(loc locator.Locator, fields []kindmeta.Field) (Result, error) {
	if len(fields) == 0 {
		return Result{}, paraerr.New(paraerr.KindUsage, "unset needs at least one field")
	}
	subj, err := e.load(loc)
	if err != nil {
		return Result{}, err
	}

	var f Fields
	next := subj.state
	for _, field := range fields {
		if !kindmeta.Has(subj.kind, field) {
			return Result{}, unknownField(subj.kind, field)
		}
		switch kindmeta.Requirement(subj.kind, field) {
		case kindmeta.Required:
			return Result{}, paraerr.Newf(paraerr.KindValidation, "%s is required and cannot be unset", field)
		case kindmeta.RequiredFixed:
			return Result{}, paraerr.Newf(paraerr.KindValidation,
				"%s is fixed at creation and cannot be unset — delete and recreate", field)
		}
		if field == kindmeta.FieldCreated {
			return Result{}, paraerr.New(paraerr.KindValidation, "created cannot be unset — everything has a creation time")
		}
		// Recorded as given so the no-op report can name it, the same way a
		// `set` names a field already holding its value.
		f.Set(field, "")
		clearField(&next, field)
	}
	return e.write(subj, f, next, "")
}

// clearField removes a field from a state, whichever of the two shapes it has.
func clearField(st *truth.State, field kindmeta.Field) {
	if isList(field) {
		setList(st, field, nil)
		return
	}
	st.SetField(field, "")
}

// write is the half `set` and `unset` share: diff, log one event per changed
// field, and write through.
func (e *Env) write(subj *subject, f Fields, next truth.State, note string) (Result, error) {
	changes := diff(subj.kind, subj.state, next)
	res := Result{Locator: subj.loc, Kind: subj.kind, Changes: changes}
	for _, field := range f.Names() {
		if !changed(changes, field) {
			res.NoOps = append(res.NoOps, NoOp{Field: field, Value: display(subj.state, field)})
		}
	}

	if len(changes) == 0 {
		// Nothing about the thing changed. A --note given anyway is still an
		// act of attention the user performed, and §26 records it: "no change
		// (status already blocked); note recorded". It becomes a note event of
		// its own, since there is no change event left to carry it.
		if note == "" {
			return res, nil
		}
		events := []journal.Event{journal.NewNote(e.Now.UTC(), note)}
		wrote, err := apply(e, []*plan{{subj: subj, events: events}}, nil)
		res.Wrote, res.NoteRecorded = wrote, true
		return e.syncSurface(res, err)
	}

	events := make([]journal.Event, 0, len(changes))
	for _, c := range changes {
		// The reason rides on every change event rather than on one of them.
		// A journal line is read on its own — `log --kind change` filters to
		// exactly these — and §18.2 requires a blocker to carry its reason, so
		// a line that dropped it would be the line that mattered.
		events = append(events, journal.NewChange(e.Now.UTC(), string(c.Field), c.From, c.To, note))
	}

	subj.state = next
	p := &plan{subj: subj, events: events, writeState: true, createdMoved: changed(changes, kindmeta.FieldCreated)}
	wrote, err := apply(e, []*plan{p}, nil)
	res.Wrote = wrote
	return e.syncSurface(res, err)
}

func changed(changes []Change, field kindmeta.Field) bool {
	for _, c := range changes {
		if c.Field == field {
			return true
		}
	}
	return false
}

// load opens the subject a mutating verb names, refusing a locator that
// addresses nothing. Containers are legal subjects for `note` but not for
// `set`; the field matrix says so on its own, since a container's only fields
// are name, description, and created.
func (e *Env) load(loc locator.Locator) (*subject, error) {
	if len(loc) == 0 {
		return nil, paraerr.New(paraerr.KindValidation, "the tree root has no fields to set — see `para config`")
	}
	exists, err := tree.Exists(e.Root, loc)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", loc)
	}
	return e.open(loc)
}
