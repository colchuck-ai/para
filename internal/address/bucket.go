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
