package mutate

// refreshCursorSurface re-derives the Cursor IDE compatibility surface
// (§6.2) after a mutation or `config set` of either `emit.cursor` key.
//
// Phase 16 task 7 owns the write path; until then this is a no-op stub so
// `config set` recognises the keys without leaving a branch to fill in later.
func (e *Env) refreshCursorSurface(res Result, dryRun bool, adding, removing []string, pendingConfig []byte) (Result, error) {
	return res, nil
}
