// Package review implements design §20: the six groups, what each of them can
// contain, and the ordering within a group.
//
// It is a classifier over the read side and nothing more. Every value it tests
// — §3.6's attention, §1.7's cascade, §4's pace, §7's chain — is derived by
// `view`, and the entities it tests are found by `query`. That is deliberate:
// `review` is the command that decides whether something needs looking at, and
// a second opinion here about what `stale` or `at-risk` means would be a
// disagreement between `review` and `show` about one entity — which is the
// defect neither command's own tests would catch.
//
// What is genuinely this package's is the part §20 states and nothing else
// implements: which kinds a group can contain, and the ordering by distance past
// the threshold, "because the ordering is the point".
package review

import (
	"slices"
	"strings"
	"time"

	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/query"
	"github.com/colchuck-ai/para/internal/view"
)

// Group is one of §20's six reasons something is worth looking at.
type Group string

const (
	// GroupStale is no note or measurement within `<kind>.stale-after` (§3.6).
	GroupStale Group = "stale"
	// GroupBlocked is a status of `blocked`, with no timer at all.
	GroupBlocked Group = "blocked"
	// GroupOverdue is open and past `due`.
	GroupOverdue Group = "overdue"
	// GroupBehind is a key-result whose pace is below `at-risk-pace`.
	GroupBehind Group = "behind"
	// GroupSkills is a skill untouched for longer than its `review.cadence`.
	GroupSkills Group = "skills"
	// GroupSuppressed is an unexpired suppression (§28), sorted soonest-until-first.
	GroupSuppressed Group = "suppressed"
)

// groups is §20's table order, which is the order sections print. It is
// declared once because it is not an implementation detail: it runs from what
// has gone quiet to what has gone wrong to what nobody has read in a year,
// with suppressions last as the forward-looking exception.
var groups = []Group{GroupStale, GroupBlocked, GroupOverdue, GroupBehind, GroupSkills, GroupSuppressed}

// Groups returns every group, in §20's order.
func Groups() []Group { return slices.Clone(groups) }

// Options is one `review`.
type Options struct {
	// Scope narrows the review to a region of the tree, including the entity
	// it names. The empty locator is the whole tree.
	Scope locator.Locator

	// Only selects the groups to run. Empty runs all six, which is what a bare
	// `para review` asks: the flags name a subset, and naming none of them is
	// not the same as naming an empty one.
	Only []Group

	// All includes terminal-status items and archived ones (§20).
	All bool

	// Limit truncates each group independently. Zero means no limit.
	//
	// Per group rather than overall, because §20 groups by reason and a limit
	// spent on the first group would silence whole reasons rather than
	// shortening the answer — which is the opposite of what a limit is for when
	// the output is already partitioned.
	Limit int
}

// Item is one entity in one group, with the measurement that put it there.
type Item struct {
	Entity view.Entity

	// Threshold is the §7 knob that put this item in its group, and the level
	// that supplied it. Found is false for the two groups measured against
	// something other than a configured number: `blocked` has no timer at all
	// (§20), and `overdue` is measured against the entity's own stored `due`.
	Threshold view.Threshold

	// Days is the whole-day count the group is about — days since attention for
	// `stale`, `blocked` and `skills`, days past the deadline for `overdue`.
	// HasDays is false for `behind`, whose measure is a pace and not a count.
	Days    int
	HasDays bool

	// Over is how far past the threshold this item is, and the ordering key
	// within its group (§20's "ordered within a group by distance past the
	// threshold"). It is `at-risk-pace` minus the pace for `behind`, days minus
	// the threshold for the two groups that have one — `stale` and `skills` —
	// and the day count itself for the two that do not, `blocked` having no
	// timer at all and `overdue` measuring against a date rather than a number.
	//
	// Comparable within a group and meaningless across them, which is why it
	// never leaves one.
	Over float64
}

// Pace is the key-result's pace, and false where there is none.
//
// It exists so that a caller displaying the `behind` group does not have to
// reach through Entity.KeyResult and depend on an invariant only behindItem
// enforces — a nil there would be a panic in the output layer rather than a
// wrong number, which is the worse of the two failures.
func (i Item) Pace() (float64, bool) {
	if i.Entity.KeyResult == nil {
		return 0, false
	}
	return i.Entity.KeyResult.Outlook.Pace, i.Entity.KeyResult.Outlook.HasPace
}

// Section is one group's findings.
type Section struct {
	Group Group
	// Items is the group's findings in §20's order, truncated by Options.Limit.
	Items []Item
	// Total is what the group found before the limit — §23's `total` beside its
	// `shown`.
	Total int
}

// Result is a whole review. Only non-empty sections are present: a heading with
// nothing beneath it says there is a group to look at when there is not.
type Result struct {
	Sections []Section
	// Total and Shown are §23's counts, summed across the sections.
	Total int
	Shown int
}

// Run classifies everything in scope into §20's groups.
//
// §20's two exclusions are applied in two places, each where the rule already
// lives: archived things never leave the walk (`IncludeArchived` below), and
// terminal ones are dropped here (`excluded`). The archived half carries one
// qualification the design does not spell out and §16.2 already settles for
// `list` — naming an archived locator makes what is under it legitimate,
// because the alternative is an empty answer to an explicit question.
func Run(env *view.Env, opts Options) (Result, error) {
	found, err := query.List(env, query.Options{
		Scope: opts.Scope,
		// Terminal hiding is `list`'s rule, with `list`'s exceptions (§16.2);
		// §20's is a different one, applied below, so nothing is dropped here.
		Filter:          query.Filter{All: true},
		IncludeSelf:     true,
		IncludeArchived: opts.All,
	})
	if err != nil {
		return Result{}, err
	}

	want := selected(opts.Only)
	sections := map[Group][]Item{}
	for _, ent := range found.Entities {
		if excluded(ent, opts) {
			continue
		}
		for _, g := range groups {
			if !want[g] {
				continue
			}
			item, ok, err := classify(env, ent, g)
			if err != nil {
				return Result{}, err
			}
			if ok {
				sections[g] = append(sections[g], item)
			}
		}
	}

	var out Result
	// groups, never the map: no map's iteration order may reach output (§0.2),
	// and this one decides the order the sections print in.
	for _, g := range groups {
		items := sections[g]
		if len(items) == 0 {
			continue
		}
		if g == GroupSuppressed {
			orderSuppressed(items)
		} else {
			order(items)
		}
		total := len(items)
		if opts.Limit > 0 && total > opts.Limit {
			items = items[:opts.Limit]
		}
		out.Sections = append(out.Sections, Section{Group: g, Items: items, Total: total})
		out.Total += total
		out.Shown += len(items)
	}
	return out, nil
}

// selected turns Options.Only into a set, treating empty as all six.
func selected(only []Group) map[Group]bool {
	out := map[Group]bool{}
	if len(only) == 0 {
		for _, g := range groups {
			out[g] = true
		}
		return out
	}
	for _, g := range only {
		out[g] = true
	}
	return out
}

// excluded is the terminal half of §20's "terminal items and archived things
// are excluded unless `--all`". The archived half is `query.IncludeArchived`,
// set from the same flag — writing it here as well would put one rule in two
// packages, and the copy here could only ever agree or be wrong.
//
// Terminal is the *effective* status (§1.7), so a whole subtree goes quiet when
// its project is dropped, rather than each entity needing its own terminal
// status. And `missed` is deliberately not terminal (§1.7), which is what makes
// a blown deadline unhideable — the one thing §20's exclusion must not reach.
func excluded(ent view.Entity, opts Options) bool {
	return ent.Terminal && !opts.All
}

// classify decides whether ent belongs in g, and with what measurement.
func classify(env *view.Env, ent view.Entity, g Group) (Item, bool, error) {
	switch g {
	case GroupStale:
		// §20: skills are reached by `--skills` and nothing else, because a
		// skill's threshold is `review.cadence` and putting it here would mean
		// one group reading two different knobs.
		if ent.Kind == kindmeta.KindSkill {
			return Item{}, false, nil
		}
		return staleItem(env, ent)
	case GroupSkills:
		if ent.Kind != kindmeta.KindSkill {
			return Item{}, false, nil
		}
		return staleItem(env, ent)
	case GroupBlocked:
		return blockedItem(env, ent)
	case GroupOverdue:
		return overdueItem(env, ent)
	case GroupBehind:
		return behindItem(env, ent)
	case GroupSuppressed:
		return suppressedItem(env, ent)
	default:
		return Item{}, false, nil
	}
}

// staleItem is both timer groups: `--stale` and `--skills` differ only in which
// kinds they admit, since the key each reads is config.StaleKey's answer and
// view.Stale is the one place the comparison is made (§7, §20).
//
// A threshold that resolves nowhere never fires, which is §7's rule stated
// outright — "unset everywhere means the check never fires" — and the reason
// the three thresholds deliberately have no built-in default.
func staleItem(env *view.Env, ent view.Entity) (Item, bool, error) {
	if suppressed(env, ent) {
		return Item{}, false, nil
	}
	th, stale, err := env.Stale(ent)
	if err != nil || !stale {
		return Item{}, false, err
	}
	days := env.DaysSince(ent.Attention)
	return Item{
		Entity:    ent,
		Threshold: th,
		Days:      days,
		HasDays:   true,
		Over:      float64(days) - th.Value,
	}, true, nil
}

// blockedItem is §20's second row: status is `blocked`, and **no timer** —
// blocked is always listed.
//
// It reads the effective status (§1.7) for the same reason `--status` does: a
// blocked objective under a dropped project is not blocked any more, it is
// dropped, and the cascade is what says so.
//
// With no threshold there is nothing to be past, so the ordering falls back to
// the same clock every other group's does — longest untouched first, which is
// §20's ordering with a threshold of zero rather than an ordering of its own.
func blockedItem(env *view.Env, ent view.Entity) (Item, bool, error) {
	if ent.EffectiveStatus != kindmeta.StatusBlocked {
		return Item{}, false, nil
	}
	days := env.DaysSince(ent.Attention)
	return Item{Entity: ent, Days: days, HasDays: true, Over: float64(days)}, true, nil
}

// overdueItem is §20's third row, which is §17's `--overdue` filter exactly:
// open and past `due`. view.Entity.Overdue is that one predicate.
func overdueItem(env *view.Env, ent view.Entity) (Item, bool, error) {
	if !ent.Overdue() {
		return Item{}, false, nil
	}
	days := env.DaysSince(ent.Deadline)
	return Item{Entity: ent, Days: days, HasDays: true, Over: float64(days)}, true, nil
}

// behindItem is §20's fourth row: a key-result whose pace is below
// `at-risk-pace`.
//
// The comparison is spelled the way §4.3's is — strictly below, undefined pace
// never qualifying — because they are the same question asked twice. A
// key-result that reads `at-risk` and is not listed as behind, or the reverse,
// would be para contradicting itself about one number.
func behindItem(env *view.Env, ent view.Entity) (Item, bool, error) {
	kr := ent.KeyResult
	if kr == nil || !kr.Outlook.HasPace {
		return Item{}, false, nil
	}
	th, err := env.AtRiskPace(ent.Locator)
	if err != nil || !th.Found {
		return Item{}, false, err
	}
	if kr.Outlook.Pace >= th.Value {
		return Item{}, false, nil
	}
	return Item{Entity: ent, Threshold: th, Over: th.Value - kr.Outlook.Pace}, true, nil
}

// suppressedItem is §20's sixth row and §28's read-path payoff: every entity
// or skill with an unexpired suppression, read via view.ActiveSuppression.
// Containers never qualify — a suppression on one would have nothing to
// suppress (§20).
func suppressedItem(env *view.Env, ent view.Entity) (Item, bool, error) {
	if ent.Kind == kindmeta.KindContainer {
		return Item{}, false, nil
	}
	end, ok := activeSuppressionUntil(env, ent)
	if !ok {
		return Item{}, false, nil
	}
	days := env.DaysUntil(end)
	return Item{
		Entity: ent,
		Threshold: view.Threshold{
			Key:   "suppression.until",
			Value: float64(days),
			Found: true,
		},
		Days:    days,
		HasDays: true,
		Over:    float64(days),
	}, true, nil
}

// suppressed reports whether ent carries an unexpired suppression, via the
// same view.ActiveSuppression fold staleItem and the skills path exclude.
func suppressed(env *view.Env, ent view.Entity) bool {
	_, ok := activeSuppressionUntil(env, ent)
	return ok
}

// activeSuppressionUntil is the unexpired half of §28.2's fold, delegated to
// view.ActiveSuppression so show and review share one read-path implementation.
func activeSuppressionUntil(env *view.Env, ent view.Entity) (time.Time, bool) {
	_, _, end, ok := env.ActiveSuppression(ent)
	return end, ok
}

// orderSuppressed is §20's exception: soonest-until-first, because "what's
// coming off the shelf" is the question that group exists to answer (§28).
func orderSuppressed(items []Item) {
	slices.SortStableFunc(items, func(a, b Item) int {
		switch {
		case a.Over < b.Over:
			return -1
		case a.Over > b.Over:
			return 1
		}
		return strings.Compare(a.Entity.Locator.String(), b.Entity.Locator.String())
	})
}

// order is §20's "ordered within a group by distance past the threshold":
// furthest past first, because the reason to group by reason is to work down
// each list. Ties break on locator, so the order is total and stable and a
// second run of an unchanged tree prints the same bytes (§0.2).
func order(items []Item) {
	slices.SortStableFunc(items, func(a, b Item) int {
		switch {
		case a.Over > b.Over:
			return -1
		case a.Over < b.Over:
			return 1
		}
		return strings.Compare(a.Entity.Locator.String(), b.Entity.Locator.String())
	})
}
