package address

import (
	"slices"

	"github.com/colchuck-ai/para/internal/locator"
)

// buckets are the four top-level containers §1.1 gives the tree, and — under
// archive/ — the three mirrors of them §1.6 gives it. This is
// kindmeta.IsContainer's old bucket list, moved here unchanged (plan Phase
// 17 task 6) rather than reimplemented from AllNouns: a bucket is a
// plain-directory-placement question about a Locator found on disk, asked
// by doctor's `untracked` finding, and that question has always had its own
// fixed four-word answer independent of which of R2's seven nouns treats an
// empty chain as a bucket too.
var buckets = []string{"projects", "areas", "resources", "archive"}

// IsBucket reports whether loc names one of §1.1's four buckets or one of
// the three mirrors of them the archive holds (§1.6).
//
// It lived in kindmeta beside IsContainer until Phase 17 task P17.6 deleted
// IsContainer: speaking the noun makes container inference unnecessary for
// every caller except the walk, which classifies bare directories it finds
// on disk and so keeps its own equivalent (tree.isContainerPosition). This
// function's own caller — doctor's `untracked` finding, which asks "is this
// plain directory sitting directly in a bucket, the one place only
// entities belong" about a Locator the walk already found — has nothing to
// do with a noun a user typed, so it moved here rather than to a walk
// package it does not otherwise depend on.
func IsBucket(loc locator.Locator) bool {
	switch len(loc) {
	case 1:
		return slices.Contains(buckets, loc[0])
	case 2:
		// The archive mirrors the three buckets that hold things, and only
		// those three: an archive of the archive names nothing (§1.6).
		return loc[0] == "archive" && loc[1] != "archive" && slices.Contains(buckets, loc[1])
	default:
		return false
	}
}

// IsArchiveRoot reports whether loc is exactly the archive root: the one
// bucket IsBucket admits that FromLocator always refuses. "The whole
// archive" takes no noun at all (§1.6, R7's `para list --archived` names it
// with no address), so there is no address for the bare locator "archive"
// to convert to — but the archive root is still a real container with its
// own generated files (§1.1's tree diagram: README.md, AGENTS.md, CLAUDE.md,
// ACTIVITY.md), so every caller that walks to it and needs a printable
// locator string — render's README and ACTIVITY.md among them — checks this
// first rather than each re-deriving the one-segment special case for
// itself.
func IsArchiveRoot(loc locator.Locator) bool {
	return len(loc) == 1 && loc[0] == "archive"
}

// String is the printable dotted form of loc for a generated projection
// (R1, R24): every site where a locator must be a single token converts
// through here rather than each re-deriving the same two exceptions to
// FromLocator.
//
// The empty locator — the tree root (§1.4), which has no locator of its
// own — prints as "": some callers (README.md's frontmatter) never reach
// this function with one because they branch on the root shape earlier;
// others (the `activity --recursive` rollup's locator column) print every
// subject's locator uniformly and need the empty string as the answer for
// the one subject that has none. The archive root prints as IsArchiveRoot's
// bucket word, for the reason that predicate documents. Every other locator
// converts through FromLocator.
func String(loc locator.Locator) (string, error) {
	switch {
	case len(loc) == 0:
		return "", nil
	case IsArchiveRoot(loc):
		return "archive", nil
	}
	addr, err := FromLocator(loc)
	if err != nil {
		return "", err
	}
	return addr.String(), nil
}
