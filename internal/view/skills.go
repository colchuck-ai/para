package view

import (
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
)

// SkillReach is one skill whose scope reaches an entity, and the entry that
// reached it — §16.1's `skills` line, which is "`para rules` folded in" (§13.1):
// "why is this rule in my context" answerable without a second command.
type SkillReach struct {
	// Locator is the skill, e.g. skills.signups-report.
	Locator locator.Locator
	// Name is the skill's `name` field, for output that wants it.
	Name string
	// Via is the scope entry that covered the entity, as stored. It is empty
	// for a skill with no scope at all, which §5.2 defines as the whole tree —
	// a distinction worth keeping, because "scope projects" and "unscoped" are
	// different answers to why a rule is in context.
	Via string
	// WholeTree reports the unscoped case.
	WholeTree bool
}

// SkillsReaching lists the skills whose scope covers loc, in the walk's stable
// order (§5.2's "walk from the entity to root collecting matches, computed when
// asked, never stored").
//
// Scope lives in exactly one place — a skill's state.toml (§5.2) — so this reads
// the skills and not the tree: there is no entity-side opt-in to consult and no
// second matcher to run. That also bounds the cost by the number of skills
// rather than by the size of the tree, which is what §5.4's "no entity→skill
// index" is affordable without.
func (e *Env) SkillsReaching(loc locator.Locator) ([]SkillReach, error) {
	skills, err := tree.Skills(e.Root)
	if err != nil {
		return nil, err
	}

	var out []SkillReach
	for _, node := range skills {
		state, err := truth.ReadState(node.Path)
		if err != nil {
			return nil, err
		}
		reach, ok := reaches(state.Scope, loc)
		if !ok {
			continue
		}
		reach.Locator, reach.Name = node.Locator, state.Name
		out = append(out, reach)
	}
	return out, nil
}

// reaches reports whether a scope list covers loc, and by which entry.
//
// An empty scope is the whole tree (§5.2: "omitting scope entirely means the
// whole tree ... absence already says it"). An entry covers its own locator and
// everything beneath it — containment, not a glob language — so the test is a
// prefix test on segments, never on the printed string: `areas.health` must not
// be read as covering `areas.healthcare`.
//
// The first covering entry wins. Two entries can both cover an entity when one
// nests inside the other, and naming the nearest would be a better answer than
// naming the first — but scope order is the author's, and §5.2 gives no
// precedence rule to appeal to, so the honest report is the one they wrote
// first rather than an ordering para invented.
func reaches(scope []string, loc locator.Locator) (SkillReach, bool) {
	if len(scope) == 0 {
		return SkillReach{WholeTree: true}, true
	}
	for _, entry := range scope {
		parsed, err := locator.Parse(entry)
		if err != nil {
			// A scope entry that is not a locator is doctor's finding (§10);
			// treating it as a match would put a rule in context on the
			// strength of a typo.
			continue
		}
		if covers(parsed, loc) {
			return SkillReach{Via: entry}, true
		}
	}
	return SkillReach{}, false
}

// covers reports whether entry names loc or an ancestor of it.
func covers(entry, loc locator.Locator) bool {
	if len(entry) > len(loc) {
		return false
	}
	for i, seg := range entry {
		if loc[i] != seg {
			return false
		}
	}
	return true
}
