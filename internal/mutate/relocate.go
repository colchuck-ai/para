package mutate

import (
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/writeset"
)

// fieldLocator is the `field` a relocation's change event carries.
//
// It is deliberately not one of §15's fields, because nothing stores a locator —
// §8.3 lists it as absent by construction, since it is the path. §18.3 calls this
// out as "the single place a derived value enters a journal", and earns the
// exception on the grounds that a journal is a record of what happened rather
// than a copy of current state: the entity's own history is the one place a move
// has to remain visible after the fact.
const fieldLocator = "locator"

// Verb is the word a relocation's summary leads with (§26), and the operation its
// parents' child events carry (§3.1).
type Verb string

// The three relocating verbs. `remove` is not one: it relocates nothing, and its
// result shape is different enough to be its own type (see Removal).
const (
	VerbMoved      Verb = "moved"
	VerbArchived   Verb = "archived"
	VerbUnarchived Verb = "unarchived"
)

func (v Verb) childOp() journal.ChildOp {
	switch v {
	case VerbArchived:
		return journal.ChildOpArchived
	case VerbUnarchived:
		return journal.ChildOpUnarchived
	default:
		return journal.ChildOpMoved
	}
}

// entityMove is one entity whose locator changes.
type entityMove struct {
	From locator.Locator
	To   locator.Locator

	// Subtree reports whether everything beneath From moved with it. True for
	// the entity the verb was given; false for an ancestor `unarchive`
	// reinstated, whose other children stayed archived behind a stub (§1.6).
	//
	// It is load-bearing for scope rewriting: a `scope` entry naming something
	// beneath a reinstated ancestor must not be rewritten, because that thing did
	// not move.
	Subtree bool
}

// ScopeRewrite is one skill whose `scope` a relocation rewrote (§5.4).
type ScopeRewrite struct {
	Skill   locator.Locator
	Entries int
}

// scopePlan is a ScopeRewrite plus what to write for it.
type scopePlan struct {
	loc     locator.Locator
	entries []string
	from    string
	to      string
}

// Relocation is a planned move, archive, or unarchive: everything the verb
// decided, settled in full before a byte is touched.
//
// Planning and applying are two calls because `--dry-run` has to be honest
// (§19). A rehearsal that re-derived its own answer would be rehearsing a
// different computation than the one it is standing in for; here the same value
// is either reported or applied, so the only difference between the two is
// whether Apply was called.
type Relocation struct {
	Verb Verb
	From locator.Locator
	To   locator.Locator
	Kind kindmeta.Kind

	// Descendants is how many containers and entities travel with the named
	// entity — §26's "(3 descendants moved with it)".
	Descendants int

	// Reinstated are the archived ancestors `unarchive` brought back as live
	// entities, shallowest first, at their new locators (§1.6).
	Reinstated []locator.Locator

	// The four things that can happen to a stub, each as archive-side locators.
	// A stub is a locator segment with no entity behind it (§1.6), so all four
	// are named that way and the CLI turns them into the directory paths §26
	// prints.
	//
	// StubsCreated are placeholders `archive` left for live ancestors that stayed
	// behind. StubAdopted is the one a relocation turned into an entity, or nil.
	// StubsDemoted are archived entities `unarchive` left behind as bare
	// directories because an archived sibling stayed put. StubsRemoved are the
	// ones left recording nothing at all.
	StubsCreated []locator.Locator
	StubAdopted  locator.Locator
	StubsDemoted []locator.Locator
	StubsRemoved []locator.Locator

	// ScopeRewrites is every skill whose scope entries this relocation rewrote.
	ScopeRewrites []ScopeRewrite

	env         *Env
	reloc       writeset.Relocation
	moves       []entityMove
	descendants []locator.Locator // post-relocation locators
	scopes      []scopePlan
}

// Apply performs the relocation and writes everything that follows from it.
//
// The order is fixed by writeset: the bytes move first, then every entity whose
// locator changed re-derives its projections at the new path, then the parents at
// both ends log the containment change (§2.3, §3.3, §18.3).
func (r *Relocation) Apply() (Result, error) {
	e := r.env
	res := Result{Locator: r.To, Kind: r.Kind}

	ops, err := writeset.Relocate(r.reloc)
	res.Wrote = e.wrote(ops)
	if err != nil {
		return res, err
	}

	subjects, err := r.subjectPlans()
	if err != nil {
		return res, err
	}
	parents, err := r.parentPlans()
	if err != nil {
		return res, err
	}

	wrote, err := apply(e, subjects, parents)
	res.Wrote = append(res.Wrote, wrote...)
	return res, err
}

// subjectPlans is everything the relocation rewrites: each moved entity in full,
// each descendant's README, and each skill whose scope moved with it.
func (r *Relocation) subjectPlans() ([]*plan, error) {
	e := r.env
	var out []*plan

	for _, m := range r.moves {
		subj, err := e.open(m.To)
		if err != nil {
			return nil, err
		}
		out = append(out, &plan{
			subj:   subj,
			events: []journal.Event{journal.NewChange(e.Now.UTC(), fieldLocator, m.From.String(), m.To.String(), "")},
		})
	}

	// A descendant's own fields did not change, so it gets no event and no
	// state.toml rewrite — one home per event (§3.2), and the thing that
	// happened here happened to the entity that was named. What did change is
	// its locator, and README.md's frontmatter is the only projection that
	// carries one: ACTIVITY.md, MEASUREMENTS.csv, SKILL.md, and the derived
	// rules name none. So the walk §19 licenses for these four verbs stays as
	// narrow as one file per descendant.
	for _, loc := range r.descendants {
		subj, err := e.open(loc)
		if err != nil {
			return nil, err
		}
		out = append(out, &plan{subj: subj, onlyProjections: []render.Renderer{render.Readme}})
	}

	for _, sp := range r.scopes {
		subj, err := e.open(sp.loc)
		if err != nil {
			return nil, err
		}
		subj.state.Scope = sp.entries
		out = append(out, &plan{
			subj:       subj,
			writeState: true,
			// A `change` event, because the skill's stored field genuinely
			// changed and §3.1 defines that kind as exactly this. Para made the
			// edit on the user's behalf (§5.4 owns the rename), and without the
			// event the skill's scope would differ from what its history says
			// was last set, with nothing anywhere to say why.
			events: []journal.Event{journal.NewChange(r.env.Now.UTC(), string(kindmeta.FieldScope), sp.from, sp.to, "")},
		})
	}
	return out, nil
}

// parentPlans is the containment half: one event at the parent the entity left
// and one at the parent it arrived in (§18.3, §18.5).
//
// Events are gathered per parent rather than per move, because one container can
// be both ends of one relocation and can also be an end of two — `unarchive`
// reinstates an ancestor and moves a child into it in the same operation. Two
// plans for one directory would each render its ACTIVITY.md from the same
// starting bytes and the second would overwrite the first, losing a line.
func (r *Relocation) parentPlans() ([]*plan, error) {
	op := r.Verb.childOp()

	var order []locator.Locator
	events := map[string][]journal.Event{}
	add := func(parent locator.Locator, ev journal.Event) {
		key := parent.String()
		if _, seen := events[key]; !seen {
			order = append(order, parent)
		}
		events[key] = append(events[key], ev)
	}

	for _, m := range r.moves {
		// §3.1's table gives `child` events `from`/`to` locators "for moves" and
		// nothing else. Archival needs neither: it prepends or drops one
		// `archive` segment, so both ends follow from the parent's own locator
		// and the child's id. A move is the case where they genuinely say
		// something the event otherwise could not.
		var from, to string
		if op == journal.ChildOpMoved {
			from, to = m.From.String(), m.To.String()
		}
		oldParent := r.relocated(m.From[:len(m.From)-1])
		newParent := m.To[:len(m.To)-1]

		// The old parent names the id that left; the new one names the id that
		// arrived. They differ only for a rename, which is also the case where
		// the two ends are one journal — so the one event kept is the one that
		// says what left, and its from/to carries where it went.
		add(oldParent, journal.NewChild(r.env.Now.UTC(), op, m.From[len(m.From)-1], from, to, ""))
		if oldParent.String() != newParent.String() {
			add(newParent, journal.NewChild(r.env.Now.UTC(), op, m.To[len(m.To)-1], from, to, ""))
		}
	}

	var out []*plan
	for _, parent := range order {
		p, err := r.env.parentPlanAt(parent, events[parent.String()])
		if err != nil {
			return nil, err
		}
		if p != nil {
			out = append(out, p)
		}
	}
	return out, nil
}

// relocated maps a pre-relocation locator to where it ends up, which is itself
// unless this relocation moved it.
func (r *Relocation) relocated(loc locator.Locator) locator.Locator {
	for _, m := range r.moves {
		if m.From.String() == loc.String() {
			return m.To
		}
	}
	return loc
}

// carried records what travels with the named entity: its descendants at their
// new locators, and how many there are.
func (r *Relocation) carried() error {
	nodes, err := tree.Subtree(r.env.Root, r.From)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		if n.Locator.String() == r.From.String() {
			continue
		}
		r.Descendants++
		if n.Stub {
			// A stub has no README.md and no truth of its own; there is nothing
			// to re-render for it (§1.6).
			continue
		}
		rest := n.Locator[len(r.From):]
		r.descendants = append(r.descendants, append(slices.Clone(r.To), rest...))
	}
	return nil
}

// planScope computes the scope rewrites this relocation owes (§5.4).
//
// It is the mitigation §5.4 names for scope being an enumerated locator list:
// "an enumerated locator is a second copy of a path — the exact thing v2 §1.4
// refused — and this is the mitigation: para owns the rename." An entry covers
// its locator and everything beneath it (§5.2), so what has to be rewritten is
// every entry naming the moved locator or anything under it.
func (r *Relocation) planScope() error {
	skills, err := tree.Skills(r.env.Root)
	if err != nil {
		return err
	}
	for _, node := range skills {
		state, err := truth.ReadState(node.Path)
		if err != nil {
			return err
		}
		next, n := rewriteScope(state.Scope, r.moves)
		if n == 0 {
			continue
		}
		r.ScopeRewrites = append(r.ScopeRewrites, ScopeRewrite{Skill: node.Locator, Entries: n})
		r.scopes = append(r.scopes, scopePlan{
			loc:     node.Locator,
			entries: next,
			from:    strings.Join(state.Scope, ", "),
			to:      strings.Join(next, ", "),
		})
	}
	return nil
}

// rewriteScope rewrites the entries the moves invalidate and reports how many
// changed.
//
// A move matches an entry two ways, and the difference matters: it matches its
// own locator exactly, and — only when it carried its subtree — it matches
// anything beneath it. An ancestor `unarchive` reinstated carried no subtree, so
// an entry naming one of its still-archived children must be left alone.
func rewriteScope(scope []string, moves []entityMove) ([]string, int) {
	out := make([]string, 0, len(scope))
	seen := map[string]bool{}
	changed := 0

	for _, entry := range scope {
		next := entry
		for _, m := range moves {
			from := m.From.String()
			switch {
			case entry == from:
				next = m.To.String()
			case m.Subtree && strings.HasPrefix(entry, from+"."):
				next = m.To.String() + entry[len(from):]
			default:
				continue
			}
			break
		}
		if next != entry {
			changed++
		}
		// A rewrite can land on an entry the list already has — moving something
		// under a locator scope already names. Two identical entries would render
		// the same clause twice in the rule file (§5.3).
		if seen[next] {
			continue
		}
		seen[next] = true
		out = append(out, next)
	}
	return out, changed
}

// relocatable refuses the subjects none of the four verbs accept, so each verb
// states only what is peculiar to it.
//
// Containers and buckets are refused because they are part of their parent's
// shape rather than things in their own right: §18.1 creates `objectives/` with
// the project and gives no verb for detaching it, and a bucket is the tree's
// top-level furniture. Moving or removing one would leave a project with nowhere
// to put an objective and no error anywhere to say so.
// The verb is spelled twice because the refusals need both forms: a stub is
// "nothing to unarchive", a container "cannot be unarchived".
func (e *Env) relocatable(loc locator.Locator, base, past string) (kindmeta.Kind, error) {
	if len(loc) == 0 {
		return kindmeta.KindUnknown, paraerr.Newf(paraerr.KindValidation, "the tree root cannot be %s", past)
	}
	kind, err := tree.KindAt(loc)
	if err != nil {
		return kindmeta.KindUnknown, err
	}
	if kind == kindmeta.KindContainer {
		return kind, paraerr.Newf(paraerr.KindValidation,
			"%s is a container — it is part of its parent's shape and cannot be %s on its own", loc, past)
	}
	exists, err := tree.Exists(e.Root, loc)
	if err != nil {
		return kind, err
	}
	if !exists {
		isStub, err := tree.IsStub(e.Root, loc)
		if err != nil {
			return kind, err
		}
		if isStub {
			return kind, paraerr.Newf(paraerr.KindNotFound,
				"nothing to %s — %s is a stub, not an entity", base, loc)
		}
		return kind, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", loc)
	}
	return kind, nil
}
