package mutate

import (
	"github.com/colchuck-ai/para/internal/config"
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
// Only ACTIVITY.md is re-rendered at the level itself. A config change alters
// no state, so no other projection of this level can have changed.
//
// The two `emit.claude` keys are the exception, and it is one §26 spells out:
// `config set emit.claude true` prints the config file, eight CLAUDE.md files,
// and a link per skill. That is legal here and not a §2.3 violation, because
// the Claude surface is a fixed set of locations rather than a walk — see
// claude.go. `emit.gitattributes` is deliberately *not* a second exception: it
// governs a delimited block inside a file whose other lines belong to the
// repository (§9), so turning it off leaves a block to remove rather than a
// file, and that is `rebuild`'s to do.
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
	res := Result{Locator: loc, Kind: subj.kind, Wrote: wrote}
	if err == nil && config.AffectsClaudeSurface(key) {
		return e.refreshSurface(res)
	}
	// Otherwise the ordinary rule, unchanged: a config change on a *skill* has
	// rewritten that skill's ACTIVITY.md, which a copy-mode mirror holds. Going
	// through syncSurface rather than re-testing the kind here is what keeps
	// "any mutation whose subject is a skill" one rule with one gate — written
	// twice, it would be two gates that eventually disagree, and this is the
	// verb they disagreed on.
	return e.syncSurface(res, err)
}
