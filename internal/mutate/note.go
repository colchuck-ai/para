package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/truth"
)

// Note appends a note to an entity's or a container's journal (§18.6).
//
// It is one of the two verbs that move the clock (§3.6) — unless noAttention
// is set, which is `para note --no-attention`: a note that records something
// true about the entity without claiming it was tended, so an unrelated note
// cannot buy false silence from a stale-after threshold watching a specific
// recurring activity. Every other verb that touches truth uses `change`
// events, which never move the clock regardless of this flag; noAttention
// exists only because note is the one verb whose entire purpose is to move it.
//
// This is a parameter here rather than a sibling method the way
// journal.NewNote/NewNoteNoAttention split the same choice: Env.Note has on
// the order of a dozen call sites, not journal.NewNote's ~75, so threading the
// bool through costs a small, one-time diff rather than protecting a call-site
// count large enough to be worth a second public name.
//
// dryRun is not such a parameter: it is Add's second shape (add.go's own
// AddDryRun, para-uk5), not noAttention's, because every one of Note's dozen
// call sites wants the always-writing form and none of them wants to pass
// `false` to get it (para-ato).
func (e *Env) Note(loc locator.Locator, text string, at string, noAttention bool) (Result, error) {
	return e.note(loc, text, at, noAttention, false)
}

// NoteDryRun rehearses Note (§19): nothing is written, not the journal entry
// and not the CLAUDE.md/mirror surface a note against a skill would otherwise
// reach.
func (e *Env) NoteDryRun(loc locator.Locator, text string, at string, noAttention bool) (Result, error) {
	return e.note(loc, text, at, noAttention, true)
}

func (e *Env) note(loc locator.Locator, text string, at string, noAttention, dryRun bool) (Result, error) {
	if flatten(text) == "" {
		return Result{}, paraerr.New(paraerr.KindUsage, "a note needs some text")
	}
	subj, err := e.load(loc)
	if err != nil {
		return Result{}, err
	}
	when, err := e.eventTime(at)
	if err != nil {
		return Result{}, err
	}

	note := journal.NewNote(when, text)
	if noAttention {
		note = journal.NewNoteNoAttention(when, text)
	}
	events := []journal.Event{note}

	prior, err := journal.ReadAll(truth.LogsDir(subj.dir))
	if err != nil {
		return Result{}, err
	}
	writeState := e.writeThroughCache(subj, prior, events)

	wrote, err := apply(e, []*plan{{subj: subj, events: events, writeState: writeState}}, nil, dryRun)
	return e.syncSurface(Result{Locator: loc, Kind: subj.kind, Wrote: wrote, NoteRecorded: true}, err, dryRun, nil, nil)
}
