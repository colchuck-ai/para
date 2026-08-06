package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/render"
)

// ConfigChange writes a level's config.toml and records the change in that
// level's own journal (§8.1: "the root's journal is thin but real — init,
// config changes, and child events for the four buckets").
//
// The event is a `change`, whose field/from/to are otherwise §15 field names
// (§3.1). A config key always contains a dot and a field name never does, so
// the two namespaces cannot collide, and ACTIVITY.md's existing change line
// renders it without a template of its own: "Changed **project.stale-after**
// from 14 to 30".
//
// The level's journal rather than the root's is where it lands, because §3.2 is
// the general rule and §8.1 is the root's instance of it: an event is written to
// the journal of the thing it happened to, and `config set --at projects` is
// something that happened to projects/.
//
// Only ACTIVITY.md is re-rendered. A config change alters no state, so no other
// projection of this level can have changed — with one exception that is
// deliberately left to `rebuild`: the emit knobs (§6.1, §9) decide which files
// exist across the *whole tree*, and honouring them here would make one config
// write walk every entity, which §2.3 forbids of a mutation.
func (e *Env) ConfigChange(loc locator.Locator, key, from, to string, file []byte) (Result, error) {
	subj, err := e.open(loc)
	if err != nil {
		return Result{}, err
	}

	events := []journal.Event{journal.NewChange(e.Now.UTC(), key, from, to, "")}
	p := &plan{
		subj:            subj,
		events:          events,
		config:          file,
		onlyProjections: []render.Renderer{render.Activity},
	}
	wrote, err := apply(e, []*plan{p}, nil)
	return Result{Locator: loc, Kind: subj.kind, Wrote: wrote}, err
}
