package mutate

import (
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/rebuild"
)

// This file is the write path's half of the Claude Code compatibility surface
// (§6.1), and it exists because of a tension §6.1 and §2.3 leave between them.
//
// §6.1 says a skill's import list "regenerates from the same scope walk that
// produces the rules … so adding or removing a skill keeps it correct with no
// separate bookkeeping". §2.3 says nothing walks a subtree on write and nothing
// walks to root on write. Taken together they look contradictory, and Phase 12
// left the tree honestly stale until `rebuild` rather than guess.
//
// The resolution is that the surface is a **fixed set**, not a walk. CLAUDE.md
// is emitted at exactly eight locations (§6) and the mirror holds exactly one
// entry per skill, so refreshing both costs a number of writes that depends on
// how many skills there are and not at all on how big the tree is. That is not
// what §2.3 forbids: §2.3 forbids a mutation whose cost grows with the tree.
//
// Which mutations refresh it:
//
//   - **any mutation whose subject is a skill.** Not only add and remove: in
//     copy mode the mirror holds the skill's own SKILL.md and ACTIVITY.md, so a
//     `note` on a skill drifts it too. Symlink mode cannot drift, which is why
//     it is the default — but the mode is configuration and the write path must
//     be right in both.
//   - **a `config set` of either emit.claude key**, which is §26's own
//     transcript: turning the flag on writes the config file, the eight
//     CLAUDE.md files, and a link per skill, in one command.
//
// `measure` is the one verb not wired in, and it does not need to be: a
// measurement is a key-result's event (§4.4) and a skill is not a key-result.

// syncSurface refreshes the Claude Code compatibility surface when the mutation
// that just landed changed what it holds, and folds what it did into the
// result.
//
// It takes and returns the (Result, error) pair so that a verb ends with
// `return e.syncSurface(...)` around whatever it already returned — one
// expression per verb, which is harder to forget than a separate statement and
// greppable when the set of verbs changes.
func (e *Env) syncSurface(res Result, err error) (Result, error) {
	if err != nil || res.Kind != kindmeta.KindSkill {
		return res, err
	}
	return e.refreshSurface(res)
}

// refreshSurface re-derives both halves of the surface and folds the writes,
// removals, and mirror changes into res.
//
// The resolver is a fresh one rather than the command's. `config set` has just
// rewritten the very config.toml the command's resolver read and cached, so
// asking that resolver whether `emit.claude` is on would answer with the value
// the command was called to change — the surface would then be refreshed
// against the old setting, which is the one bug this whole file exists to
// prevent. Re-reading a handful of small files is the price, and the surface
// refresh is a bounded piece of work anyway.
func (e *Env) refreshSurface(res Result) (Result, error) {
	env := rebuild.NewEnv(e.Root)

	wrote, removed, err := env.WriteClaudeSurface()
	res.Wrote = append(res.Wrote, wrote...)
	res.Removed = append(res.Removed, removed...)
	if err != nil {
		return res, err
	}

	// The mirror after the pointer files, for the reason rebuild sequences them
	// the same way: a copy-mode mirror reproduces files this mutation may have
	// just rewritten.
	changes, err := env.SyncMirror(false)
	res.Mirror = append(res.Mirror, changes...)
	return res, err
}
