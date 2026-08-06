package kindmeta

import "slices"

// §1.7's status vocabulary, per kind, in the order an error message lists it.
//
// It lives beside the §15 field matrix because it is the same kind of fact —
// what a kind admits — and because `add`, `set`, `list --status`, and `review`
// must all agree on it. §1.7's other two columns are elsewhere on purpose: the
// derived key-result statuses belong to krvalue, which computes them, and
// terminality is read by the commands that hide terminal items (§16.2, §20).
var (
	// plannableStatuses are a project's and an objective's, and the default is
	// the first of them.
	plannableStatuses = []string{"planned", "in-progress", "blocked", "done", "dropped"}

	// keyResultStatuses is the one settable value carved out of a derived
	// status (§4.3): a key-result's status is computed from its measurements,
	// except that you may declare you have stopped caring.
	keyResultStatuses = []string{"dropped"}
)

// SettableStatuses returns the statuses `add` and `set` accept for kind, in
// §1.7's order, or nil for a kind that has no status at all — an area or a
// resource, where "location is the answer" (§1.6), and a skill, which has no
// status and no off switch (§1.7).
func SettableStatuses(kind Kind) []string {
	switch kind {
	case KindProject, KindObjective:
		return slices.Clone(plannableStatuses)
	case KindKeyResult:
		return slices.Clone(keyResultStatuses)
	default:
		return nil
	}
}

// IsStatus reports whether s is a status kind will accept.
func IsStatus(kind Kind, s string) bool {
	return slices.Contains(SettableStatuses(kind), s)
}

// DefaultStatus is the status a kind is born with (§1.7's Default column), or
// "" where there is none.
//
// It is stored at creation rather than defaulted at read time. A project has a
// status from the moment it exists, and `state.toml` is where what a thing *is*
// lives (§2.2) — so a generated README frontmatter that omitted it would
// misinform every reader of the file, and every reader of the truth would need
// its own copy of this rule. §2.5's ban on stored values covers what is
// *derived*; a default is not derived, it is chosen.
func DefaultStatus(kind Kind) string {
	switch kind {
	case KindProject, KindObjective:
		return plannableStatuses[0]
	default:
		// A key-result's status is derived from its measurements (§4.3), so it
		// starts absent rather than at a value it did not earn.
		return ""
	}
}

// priorities is the closed, ordered set `priority` admits, most urgent first.
//
// The design never enumerates it — §8.3's example writes `high` and §15 lists
// the field as optional on three kinds — but §17 makes `priority` a `--sort`
// key, and sorting is what forces the decision: a free-text priority has no
// order that means anything, and lexical order would rank high, low, medium.
// Three levels is the smallest set that spells "more than this one" and "less
// than this one", and widening a closed set later is backwards-compatible
// where narrowing a free one is not.
var priorities = []string{"high", "medium", "low"}

// Priorities returns the legal priorities, most urgent first.
func Priorities() []string { return slices.Clone(priorities) }

// PriorityRank is p's position in the ordering, 0 being the most urgent, and
// whether p is a priority at all — the comparison §17's `--sort priority`
// needs.
func PriorityRank(p string) (int, bool) {
	i := slices.Index(priorities, p)
	return i, i >= 0
}
