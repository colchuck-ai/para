package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
)

// Note appends a note to an entity's or a container's journal (§18.6).
//
// It is one of the two verbs that move the clock (§3.6), which is why it is a
// verb of its own rather than `set` on a field: attending to something is an
// event, and `attention` is defined as the newest note or measurement.
func (e *Env) Note(loc locator.Locator, text string, at string) (Result, error) {
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

	events := []journal.Event{journal.NewNote(when, text)}
	wrote, err := apply(e, []*plan{{subj: subj, events: events}}, nil)
	return e.syncSurface(Result{Locator: loc, Kind: subj.kind, Wrote: wrote, NoteRecorded: true}, err)
}
