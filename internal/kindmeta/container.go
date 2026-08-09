package kindmeta

import "github.com/colchuck-ai/para/internal/locator"

// buckets are the four top-level containers §1.1 gives the tree, and — under
// archive/ — the three mirrors of them §1.6 gives it.
var buckets = []string{"projects", "areas", "resources", "archive"}

// IsContainer reports whether loc names a legal container position (§1.2).
//
// KindOf never answers this, and cannot: a container's name is always a
// reserved word and a reserved word can never be an id, so every container
// locator is one KindOf is defined to refuse. What decides a container is
// therefore the *position* — the four buckets, the three archived mirrors,
// `objectives/` under a project, and `key-results/` under an objective — and
// that list is here beside the kind derivation because it is the same table:
// §1.3's rows are exactly these containers with an id appended.
//
// It exists because "a directory whose name is reserved" and "a container" are
// not the same set, and treating them as the same set is how a reserved word
// used as an id becomes invisible. `projects/skills/` is a project someone
// named `skills`, which §10 calls a `collision`; `projects/acme/key-results/`
// is a container in a place that has none, which §10 calls `misplaced`. Both
// look like containers to a rule that only tests the name.
func IsContainer(loc locator.Locator) bool {
	if len(loc) == 0 {
		return false
	}
	last := loc[len(loc)-1]

	if IsBucket(loc) {
		return true
	}

	parent, err := KindOf(loc[:len(loc)-1])
	if err != nil {
		return false
	}
	switch last {
	case "objectives":
		return parent.Kind == KindProject
	case "key-results":
		return parent.Kind == KindObjective
	default:
		return false
	}
}

// IsBucket reports whether loc names one of §1.1's four buckets or one of the
// three mirrors of them the archive holds (§1.6).
//
// It is a narrower question than IsContainer, and doctor's advisory is the one
// that asks it: `untracked` is a plain directory sitting **directly** in a
// bucket, "the one place only entities belong" (§10). Content inside an entity
// is content and is never reported, so the bucket list has to be the bucket
// list rather than the container list.
func IsBucket(loc locator.Locator) bool {
	switch len(loc) {
	case 1:
		return contains(buckets, loc[0])
	case 2:
		// The archive mirrors the three buckets that hold things, and only
		// those three: an archive of the archive names nothing (§1.6).
		return loc[0] == "archive" && loc[1] != "archive" && contains(buckets, loc[1])
	default:
		return false
	}
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
