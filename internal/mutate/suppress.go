package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/truth"
)

// Suppress appends a suppress event (§18.7, §28.2): until quiets `review
// --stale`/`--skills` for the entity or skill until that date, and note is
// the required reason — the same rule §18.2 already applies to `blocked`,
// since a suppression with no recorded reason is worthless in six months.
//
// It never moves attention: suppression and attention are independent facts
// (§28.2), and letting one imply the other would reopen the hole `change`
// events were excluded from attention to close.
func (e *Env) Suppress(loc locator.Locator, until, note string) (Result, error) {
	if until == "" {
		return Result{}, paraerr.New(paraerr.KindUsage, "--until is required for suppress")
	}
	if _, err := ptime.Deadline(until); err != nil {
		return Result{}, paraerr.Newf(paraerr.KindValidation, "until: %s", unwrapMessage(err))
	}
	if flatten(note) == "" {
		return Result{}, paraerr.New(paraerr.KindUsage, "--note is required for suppress")
	}
	return e.suppress(loc, until, note)
}

// Unsuppress appends the same event kind with until empty, clearing whatever
// suppression is active (§28.2): state folds to the newest suppress event
// exactly as attention already folds to the newest note or measurement, so
// there is no second kind and no delete-style event to define.
func (e *Env) Unsuppress(loc locator.Locator, note string) (Result, error) {
	if flatten(note) == "" {
		return Result{}, paraerr.New(paraerr.KindUsage, "--note is required for unsuppress")
	}
	return e.suppress(loc, "", note)
}

func (e *Env) suppress(loc locator.Locator, until, note string) (Result, error) {
	subj, err := e.load(loc)
	if err != nil {
		return Result{}, err
	}

	events := []journal.Event{journal.NewSuppress(e.Now.UTC(), until, note)}
	prior, err := journal.ReadAll(truth.LogsDir(subj.dir))
	if err != nil {
		return Result{}, err
	}
	writeState := e.writeThroughCache(subj, prior, events)

	wrote, err := apply(e, []*plan{{subj: subj, events: events, writeState: writeState}}, nil, false)
	return e.syncSurface(Result{Locator: loc, Kind: subj.kind, Wrote: wrote}, err, false, nil, nil)
}
