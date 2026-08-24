package mutate

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/truth"
)

// writeThroughCache computes state.toml's §28.4 cached fields — attention
// and [suppression] — from subj's journal as it will read once added lands,
// and writes the result into subj.state if either differs from what is
// already cached there.
//
// note, measure, suppress, and unsuppress each call this instead of folding
// the journal themselves, so the four verbs that can move either value
// cannot drift from one another or from journal.Attention and
// journal.ActiveSuppression — the same definitions review and show already
// trust for the live answer (§28.4's "one function computes a derived
// value"). doctor's stale-projection check and rebuild are meant to call the
// same two journal functions, by the same rule, when those land.
//
// It reports whether anything changed, which is what a caller sets a plan's
// writeState from: state.toml is truth, and truth is rewritten exactly when
// what is being written to it actually changed, the same rule set/unset
// already follow for their own fields.
func (e *Env) writeThroughCache(subj *subject, prior, added []journal.Event) bool {
	if len(subj.loc) == 0 || subj.kind == kindmeta.KindContainer {
		return false
	}
	events := merge(prior, added)
	created, _ := ptime.StoredAt(subj.state.Created)

	attention := stamp(journal.Attention(events, created))
	until, note := journal.ActiveSuppression(events)
	suppression := truth.Suppression{Until: until, Note: note}

	if subj.state.Attention == attention && subj.state.Suppression == suppression {
		return false
	}
	subj.state.Attention = attention
	subj.state.Suppression = suppression
	return true
}
