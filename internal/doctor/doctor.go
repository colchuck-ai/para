// Package doctor is the deep scan: it walks every directory including inside
// content and reports the eleven findings §10 defines (design §21.2).
//
// It is read-only and has no `--fix`, deliberately. `rebuild` is the repair for
// the one finding that has one, and a `--fix` that repaired that class and not
// the others "would teach people that doctor cleans up after itself, which for
// orphan, misplaced, invalid, and scope-unresolved it cannot" (§21.2).
//
// Three things shape the implementation:
//
//   - **It walks differently from everything else.** Every other read uses the
//     §8.5 walk, which descends only into directories holding .para/state.toml.
//     That walk's one weakness is the thing doctor exists to find: hand-`mv` an
//     entity into a content subdirectory and it vanishes from every read (§8.5).
//     So doctor runs both — the fast walk for what para can see, and a full
//     filesystem scan for what is actually there — and the difference between
//     them is `orphan` and `misplaced`.
//
//   - **It does not follow symlinks** (§21.2, §6.1), because a mirrored skill
//     under .claude/skills/ would otherwise be walked as a second copy of a
//     skill that exists once — a phantom entity manufactured by the check meant
//     to find phantoms.
//
//   - **`stale-projection` is full fidelity** (§10), and it is not derived
//     here. It asks `rebuild` what it would write and compares byte for byte,
//     so the finding and its repair cannot disagree about what a projection
//     should contain. That is also why ACTIVITY.md is compared against every
//     journal file backing it rather than the newest: rebuild reads them all,
//     and the earliest day the two renderings disagree about is what dates the
//     report (§3.5).
//
// Nothing here is a rule of its own. What a field may hold is `truth.Check`'s,
// what a locator names is `kindmeta`'s, what a journal line is is `journal`'s,
// and what a projection should contain is `rebuild`'s. Doctor's own subject is
// which of those questions to ask about what, and how to say the answer.
package doctor

import (
	"cmp"
	"slices"

	"github.com/colchuck-ai/para/internal/locator"
)

// Kind is one of §10's findings. The spelling is the one the report prints and
// the one --json carries, so a script can assert on the finding rather than on
// prose (§0.5's argument for error kinds, applied to findings).
type Kind string

const (
	// KindOrphan is a state.toml the fast walk cannot reach — an entity
	// beneath content (§1.5, §8.5).
	KindOrphan Kind = "orphan"
	// KindMisplaced is a state.toml at a location that derives no kind.
	KindMisplaced Kind = "misplaced"
	// KindInvalid is truth that will not read, or reads into something §15
	// does not admit.
	KindInvalid Kind = "invalid"
	// KindJournal is a line that is not a journal event, reported with file
	// and line (§3.1).
	KindJournal Kind = "journal"
	// KindCollision is a reserved name used as an id (§1.4).
	KindCollision Kind = "collision"
	// KindScopeUnresolved is a skill's scope entry naming a locator that does
	// not exist (§5.4).
	KindScopeUnresolved Kind = "scope-unresolved"
	// KindOrphanRule is a derived rule file whose skill is gone (§5.3).
	KindOrphanRule Kind = "orphan-rule"
	// KindOrphanMirror is a para- prefixed entry under .claude/skills/ whose
	// skill is gone (§6.1).
	KindOrphanMirror Kind = "orphan-mirror"
	// KindBrokenLink is a symlink-mode mirror that does not resolve, including
	// one materialised as a plain file by a core.symlinks=false checkout
	// (§6.1).
	KindBrokenLink Kind = "broken-link"
	// KindStaleProjection is a generated file that differs from what would be
	// written now → `para rebuild` (§2.4, §10).
	KindStaleProjection Kind = "stale-projection"
	// KindUntracked is the one advisory: a plain directory sitting directly in
	// a bucket, the one place only entities belong.
	KindUntracked Kind = "untracked"
)

// order is §10's own table order, and therefore the order findings are
// reported in: the errors as §10 lists them, then the advisories. It is a
// declared slice rather than a map because the report's order must not come
// from a map's iteration (§0.2).
var order = []Kind{
	KindOrphan, KindMisplaced, KindInvalid, KindJournal, KindCollision,
	KindScopeUnresolved, KindOrphanRule, KindOrphanMirror, KindBrokenLink,
	KindStaleProjection,
	KindUntracked,
}

// Kinds returns §10's findings in the table's order.
func Kinds() []Kind { return slices.Clone(order) }

// Advisory reports whether a finding is one of §10's advisories — "your filing
// is loose" rather than "a read would be wrong or incomplete". Only `untracked`
// is, which is why this is a predicate rather than a severity field on every
// finding: one exception spelled once.
func (k Kind) Advisory() bool { return k == KindUntracked }

// Severity is how a finding prints in the first column and how it counts
// toward the exit code (§21.2).
func (k Kind) Severity() string {
	if k.Advisory() {
		return "advisory"
	}
	return "error"
}

// Finding is one thing doctor found.
type Finding struct {
	Kind Kind
	// Path is what is at fault, relative to the tree root and
	// slash-separated: a file for most findings, a directory for the ones
	// about placement.
	Path string
	// Line is the 1-based line of a `journal` finding, and 0 elsewhere. §10
	// requires a journal problem to be "reported with file and line", and a
	// line number is the only way to find one line in a file of thousands.
	Line int
	// Locator names the entity at fault where the path addresses one, and is
	// empty where it does not — an orphan is precisely a directory whose
	// locator para cannot derive.
	Locator locator.Locator
	// Detail is the sentence after the path: what is wrong, in words.
	Detail string
}

// Report is what one doctor run found.
type Report struct {
	Findings []Finding
}

// Errors is how many findings are error severity.
func (r Report) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if !f.Kind.Advisory() {
			n++
		}
	}
	return n
}

// Advisories is how many findings are advisory.
func (r Report) Advisories() int { return len(r.Findings) - r.Errors() }

// Clean reports whether doctor found nothing at all.
func (r Report) Clean() bool { return len(r.Findings) == 0 }

// ExitCode is §21.2's contract: 0 clean, 1 on any error, 2 when only
// advisories are present.
//
// The three codes are the interface, not a detail: "CI gates on 1 and ignores
// 2; an agent learns from the code alone whether judgement is required".
func (r Report) ExitCode() int {
	switch {
	case r.Errors() > 0:
		return 1
	case r.Advisories() > 0:
		return 2
	default:
		return 0
	}
}

// sortFindings puts the report in a fixed order: §10's finding order, then by
// path, then by line. Grouping by finding rather than by path is what makes a
// long report readable — the same question answered about several places reads
// as one problem, which is usually what it is.
func sortFindings(findings []Finding) {
	rank := make(map[Kind]int, len(order))
	for i, k := range order {
		rank[k] = i
	}
	slices.SortStableFunc(findings, func(a, b Finding) int {
		if c := cmp.Compare(rank[a.Kind], rank[b.Kind]); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return cmp.Compare(a.Line, b.Line)
	})
}
