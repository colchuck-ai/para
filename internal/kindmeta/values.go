package kindmeta

import "slices"

// §1.7's status vocabulary, per kind, in the order an error message lists it.
//
// It lives beside the §15 field matrix because it is the same kind of fact —
// what a kind admits — and because `add`, `set`, `list --status`, and `review`
// must all agree on it. §1.7's other two columns are elsewhere on purpose: the
// derived key-result statuses belong to krvalue, which computes them, and
// terminality is read by the commands that hide terminal items (§16.2, §20).
// StatusBlocked is the one status code reads by name rather than by position:
// `set` refuses it without a reason (§18.2) and `review --blocked` is a group of
// nothing else (§20). It is declared here because this is where §1.7's
// vocabulary is, and a status word spelled in three packages is a status word
// one of them will eventually misspell.
const StatusBlocked = "blocked"

var (
	// plannableStatuses are a project's and an objective's, and the default is
	// the first of them.
	plannableStatuses = []string{"planned", "in-progress", StatusBlocked, "done", "dropped"}

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

// AllStatuses is every status some kind accepts, in §1.7's order and without
// repetition — the vocabulary to offer when nothing has said which kind is
// being addressed yet.
func AllStatuses() []string {
	out := slices.Clone(plannableStatuses)
	for _, s := range keyResultStatuses {
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
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

// terminalStatuses is §1.7's Terminal column: the statuses that mean a thing is
// over, and so hide it from `list` without `--all` (§16.2) and quiet its
// descendants (§1.7's cascade).
//
// It lives here rather than in the commands that read it — which is where Phase 8
// left it — because it is a third column of the same §1.7 table the two maps
// above are the first two columns of, and because `list`, `show`, and `review`
// have to agree on it. A key-result's `achieved` is in the table even though
// krvalue derives it: terminality is a property of the value, not of who computed
// it, and the caller passes whichever status it has in hand.
//
// `missed` is deliberately absent. §1.7 says so outright — "a blown deadline is
// the one thing that should not be hideable" — and it is the only place the
// terminal set is not simply "the last values in the vocabulary".
var terminalStatuses = map[Kind][]string{
	KindProject:   {"done", "dropped"},
	KindObjective: {"done", "dropped"},
	KindKeyResult: {"achieved", "dropped"},
}

// TerminalStatuses returns the statuses that are terminal for kind, in §1.7's
// order, or nil where none are.
func TerminalStatuses(kind Kind) []string {
	return slices.Clone(terminalStatuses[kind])
}

// IsTerminal reports whether status means kind is over.
//
// An area and a resource always answer false, and that is §1.6 rather than an
// omission: they have no status field at all, because location *is* their
// archival state. Something under `archive/` is dormant by path (§1.6), which is
// a separate question this does not answer.
func IsTerminal(kind Kind, status string) bool {
	return slices.Contains(terminalStatuses[kind], status)
}
