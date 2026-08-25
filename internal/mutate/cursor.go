package mutate

import "github.com/colchuck-ai/para/internal/rebuild"

// refreshCursorSurface re-derives the Cursor IDE compatibility surface
// (§6.2) after a mutation or `config set` of either `emit.cursor` key.
//
// It is claude.go's refreshSurface, mirrored against the Cursor target: a
// fresh resolver for the same reason (a `config set` has just rewritten the
// config.toml the command's own resolver already cached), and the same
// pendingConfig override for the same reason (a dry run has written nothing
// for a fresh resolver to read). It is a separate function rather than a
// branch inside refreshSurface because §6.2 says emit.cursor is independent
// of emit.claude — both surfaces refresh on their own, and either or both
// may be on at once.
//
// adding and removing carry the same rehearsal meaning refreshSurface's do,
// though CursorSurface itself has no use for them (a skill's rule file is
// not a shared pointer file with an import list to adjust) — they exist here
// only to reach SyncCursorMirror, the mirror's own rehearsal case.
func (e *Env) refreshCursorSurface(res Result, dryRun bool, adding, removing []string, pendingConfig []byte) (Result, error) {
	env := rebuild.NewEnv(e.Root)
	if pendingConfig != nil {
		if err := overrideRootConfig(env, pendingConfig); err != nil {
			return res, err
		}
	}

	wrote, removed, err := env.WriteCursorRules(dryRun)
	res.Wrote = append(res.Wrote, wrote...)
	res.Removed = append(res.Removed, removed...)
	if err != nil {
		return res, err
	}

	changes, err := env.SyncCursorMirror(dryRun, nil, adding, removing)
	res.Mirror = append(res.Mirror, changes...)
	return res, err
}
