package mutate

import (
	"github.com/colchuck-ai/para/internal/config"
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

// syncSurface refreshes both IDE compatibility surfaces — Claude Code's
// (§6.1) and Cursor's (§6.2) — when the mutation that just landed changed
// what either holds, and folds what it did into the result. The two are
// independent surfaces with their own on/off keys, so both refresh on every
// skill mutation regardless of which, if either, is currently on; each
// surface's own render decides whether that means a write or a residue
// removal.
//
// It takes and returns the (Result, error) pair so that a verb ends with
// `return e.syncSurface(...)` around whatever it already returned — one
// expression per verb, which is harder to forget than a separate statement and
// greppable when the set of verbs changes.
//
// dryRun is a parameter rather than a second copy of this function for the
// same reason apply's is: `add`'s rehearsal must ask the identical question a
// real run does, or the two could name a different set of surface files for
// no reason but having derived it twice.
//
// adding and removing are refreshSurface's parameters of the same names —
// both empty for every verb but AddDryRun and remove's dry run on a skill,
// the two cases where the subject syncSurface is refreshing the surface for
// does not match what tree.SkillIDs sees on disk (para-ato).
func (e *Env) syncSurface(res Result, err error, dryRun bool, adding, removing []string) (Result, error) {
	if err != nil || res.Kind != kindmeta.KindSkill {
		return res, err
	}
	res, err = e.refreshSurface(res, dryRun, adding, removing, nil)
	if err != nil {
		return res, err
	}
	return e.refreshCursorSurface(res, dryRun, adding, removing, nil)
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
//
// adding names the skill AddDryRun is rehearsing the creation of, so both
// halves of the surface can answer as if it already existed without it
// actually being written. removing names the skill a rehearsed `remove` is
// about to delete, so both halves answer as if it were already gone even
// though tree.SkillIDs still sees it on disk (para-ato). Every other caller
// passes both nil, because every other caller's subject already matches disk
// by the time the surface is refreshed.
//
// pendingConfig is ConfigChange's own gap in that "fresh resolver" fix: a
// fresh resolver still reads disk, and under `config set --dry-run` nothing
// has been written to disk at all, so the fresh read would answer with the
// *old* value — the very bug this function exists to avoid, just moved from
// a stale cache to a stale file. pendingConfig is the root config.toml bytes
// the command was called to write; every caller but ConfigChange passes nil,
// because every other caller's mutation is not itself a change to that file.
func (e *Env) refreshSurface(res Result, dryRun bool, adding, removing []string, pendingConfig []byte) (Result, error) {
	env := rebuild.NewEnv(e.Root)
	if pendingConfig != nil {
		if err := overrideRootConfig(env, pendingConfig); err != nil {
			return res, err
		}
	}

	wrote, removed, err := env.WriteClaudeSurface(dryRun, adding, removing)
	res.Wrote = append(res.Wrote, wrote...)
	res.Removed = append(res.Removed, removed...)
	if err != nil {
		return res, err
	}

	// The mirror after the pointer files, for the reason rebuild sequences them
	// the same way: a copy-mode mirror reproduces files this mutation may have
	// just rewritten.
	changes, err := env.SyncMirror(dryRun, nil, adding, removing)
	res.Mirror = append(res.Mirror, changes...)
	return res, err
}

// refreshGitAttributes puts §9's block into the root's .gitattributes or takes
// it out, after a `config set` of the key that decides which.
//
// It builds a fresh resolver for the reason refreshSurface does, and takes the
// same pendingConfig override for the same reason: under `config set
// --dry-run` nothing has been written to disk for a fresh resolver to read.
func (e *Env) refreshGitAttributes(res Result, dryRun bool, pendingConfig []byte) (Result, error) {
	env := rebuild.NewEnv(e.Root)
	if pendingConfig != nil {
		if err := overrideRootConfig(env, pendingConfig); err != nil {
			return res, err
		}
	}
	wrote, removed, err := env.WriteGitAttributes(dryRun)
	res.Wrote = append(res.Wrote, wrote...)
	res.Removed = append(res.Removed, removed...)
	return res, err
}

// overrideRootConfig seeds env's resolver with data as the tree root's own
// config.toml — config.Chain(nil)'s one level, which is where every RootOnly
// key resolves (config.RootOnly) and so the only level refreshSurface and
// refreshGitAttributes ever need to override.
func overrideRootConfig(env *rebuild.Env, data []byte) error {
	levels, err := config.Chain(nil)
	if err != nil {
		return err
	}
	return env.Resolver.Override(levels[0].File, data)
}
