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
// The keys that decide whether a generated file exists are the exception, and
// §26 spells the first of them out: `config set emit.claude true` prints the
// config file, eight CLAUDE.md files, and a link per skill. That is legal here
// and not a §2.3 violation, because the Claude surface is a fixed set of
// locations rather than a walk — see claude.go.
//
// `emit.gitattributes` is the same kind of key with a smaller reach: one file
// at the root, whose block goes in or comes out here (§9). It was left to
// `rebuild` until Phase 14, on the reasoning that removing a block is not
// removing a file — true, and not a reason to leave the tree stale. A command
// that succeeds and leaves `doctor` red is the failure Phase 13's review named
// twice, and the cost of not having it is one write at a fixed path.
//
// dryRun is a parameter rather than a second copy of this function for the
// same reason apply's is (para-ato): `config set --dry-run`'s rehearsal must
// ask apply and the surface refresh the identical question a real run does.
func (e *Env) ConfigChange(loc locator.Locator, key, from, to string, file []byte, dryRun bool) (Result, error) {
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
	wrote, err := apply(e, []*plan{p}, nil, dryRun)
	res := Result{Locator: loc, Kind: subj.kind, Wrote: wrote}
	if err == nil && config.AffectsClaudeSurface(key) {
		return e.refreshSurface(res, dryRun, nil, nil, file)
	}
	if err == nil && config.AffectsCursorSurface(key) {
		return e.refreshCursorSurface(res, dryRun, nil, nil, file)
	}
	if err == nil && key == config.KeyEmitGitattributes {
		res, err = e.refreshGitAttributes(res, dryRun, file)
	}
	// And then the ordinary rule, which every path falls through to: a config
	// change on a *skill* has rewritten that skill's ACTIVITY.md, which a
	// copy-mode mirror holds. Going through syncSurface rather than re-testing
	// the kind here is what keeps "any mutation whose subject is a skill" one
	// rule with one gate.
	//
	// The `emit.gitattributes` branch above *falls through* rather than
	// returning, and that is not an accident of style — it is this exact bug
	// found twice. Phase 13's review caught `ConfigChange` gating on the key
	// while every other verb gated on the kind, so a `config set --at skills.x`
	// skipped the mirror; a returning branch here would have reintroduced it for
	// one more key. `refreshSurface` returns directly because it has already
	// synced the mirror itself.
	return e.syncSurface(res, err, dryRun, nil, nil)
}
