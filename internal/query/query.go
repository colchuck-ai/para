// Package query is `list`'s half of the read surface: the walk that finds
// entities, §17's filters over them, and the sort and limit applied to what
// survives (design §16.2, §17).
//
// It is a layer over `tree` and `view` and holds no rules of its own about what
// an entity *is*. What it does own is what §16.2 calls transparency — the
// difference between the tree's shape and the reader's — which is why `--direct`
// and the walk live together: both are answers to "how deep is this, as far as
// the output is concerned", and a container is invisible to both.
package query

import (
	"slices"
	"strings"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/tagexpr"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/view"
)

// Filter is §17's table, parsed. A zero Filter matches every live entity.
type Filter struct {
	// Tags is a parsed --tags expression, or nil (§17).
	Tags tagexpr.Expr
	// Match is --match's text, compared case-insensitively against the thing's
	// own text: name, description, tags, and journal note bodies.
	Match string
	// Status filters on effective status (§1.7), never on the stored one.
	Status string
	// Priority filters on a field only some kinds have; one that has no
	// priority never matches.
	Priority string
	// Overdue keeps only what is open and past `due`.
	Overdue bool
	// Direct keeps only immediate children, counted after container
	// transparency (§17).
	Direct bool
	// All includes terminal-status items, which are otherwise hidden (§16.2).
	All bool
}

// Options is one `list`: where to look, what to keep, and how to present it.
type Options struct {
	// Scope is the container or entity to list beneath. The empty locator is
	// the whole tree. Listing is of what is *beneath* the locator, never of the
	// locator itself — `show` is how you see the thing you named (§16.1).
	Scope locator.Locator

	Filter Filter

	Sort    SortKey
	Reverse bool
	// Limit truncates the sorted result. Zero means no limit.
	Limit int
}

// Result is what `list` prints, and the three counts §17 and §23 require it to
// be able to explain.
type Result struct {
	Entities []view.Entity

	// Total is what the filters matched, before --limit — the M in §17's
	// "showing 20 of 143".
	Total int
	// Hidden is how many of those were then withheld for having a terminal
	// status, and HiddenStatuses names which statuses did it, in §1.7's order.
	//
	// It is a separate number from Total on purpose: folding them together
	// would leave one count with two causes and two different fixes, when the
	// fixes are `--all` and a wider filter respectively.
	Hidden         int
	HiddenStatuses []string
}

// Truncated reports whether --limit dropped anything.
func (r Result) Truncated() bool { return len(r.Entities) < r.Total }

// List walks the tree beneath opts.Scope and returns the entities that survive
// §17's filters, sorted and limited (§16.2).
//
// Containers are traversed and never returned: "a row you can neither set nor
// act on is noise" (§16.2). `archive/` is traversed only when the scope names
// it, because archived things are not hidden, they are somewhere else (§1.6).
func List(env *view.Env, opts Options) (Result, error) {
	if err := checkScope(env, opts.Scope); err != nil {
		return Result{}, err
	}

	var matched []view.Entity
	hidden := map[string]bool{}
	hiddenCount := 0

	err := walk(env.Root, opts.Scope, func(node tree.Node) error {
		if !visible(node, opts) {
			return nil
		}
		state, err := truth.ReadState(node.Path)
		if err != nil {
			return err
		}
		ent, err := env.Derive(node.Locator, node.Kind, node.Path, state)
		if err != nil {
			return err
		}
		ok, err := matches(env, ent, opts.Filter)
		if err != nil || !ok {
			return err
		}
		// Terminal hiding happens after matching, so `showing N of M` keeps
		// meaning what §17 says — what the filters matched — and the hidden
		// count is a second, separately explained number.
		if ent.Terminal && !opts.Filter.All {
			hiddenCount++
			hidden[ent.EffectiveStatus] = true
			return nil
		}
		matched = append(matched, ent)
		return nil
	})
	if err != nil {
		return Result{}, err
	}

	if err := sortEntities(matched, opts.Sort, opts.Reverse); err != nil {
		return Result{}, err
	}
	out := Result{
		Entities:       matched,
		Total:          len(matched),
		Hidden:         hiddenCount,
		HiddenStatuses: orderStatuses(hidden),
	}
	if opts.Limit > 0 && len(out.Entities) > opts.Limit {
		out.Entities = out.Entities[:opts.Limit]
	}
	return out, nil
}

// checkScope refuses a scope that names nothing, so that a mistyped locator is
// an error rather than an empty list — which reads as "you have none of those".
func checkScope(env *view.Env, scope locator.Locator) error {
	if len(scope) == 0 {
		return nil
	}
	exists, err := tree.Exists(env.Root, scope)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	// A stub is a real position in the tree with real things beneath it
	// (§1.6), so listing beneath one is a legitimate question.
	stub, err := tree.IsStub(env.Root, scope)
	if err != nil {
		return err
	}
	if stub {
		return nil
	}
	return paraerr.Newf(paraerr.KindNotFound, "%s does not exist", scope)
}

// walk visits every node beneath scope, plus the skills, in the §8.5 walk's
// stable order.
//
// Skills are entities (§5.1) and so are listed, but they live under
// .agents/skills/ rather than beneath a bucket (§1.4) — so they are reached
// only by an unscoped list or by one scoped to `skills`, never by walking down
// from `projects`.
func walk(root string, scope locator.Locator, visit func(tree.Node) error) error {
	if len(scope) == 0 {
		return tree.Walk(root, visit)
	}
	if scope[0] == "skills" {
		nodes, err := tree.Skills(root)
		if err != nil {
			return err
		}
		for _, n := range nodes {
			if err := visit(n); err != nil {
				return err
			}
		}
		return nil
	}
	nodes, err := tree.Subtree(root, scope)
	if err != nil {
		return err
	}
	for _, n := range nodes {
		// Subtree includes the scope itself; `list` is of what is beneath it.
		if len(n.Locator) == len(scope) {
			continue
		}
		if err := visit(n); err != nil {
			return err
		}
	}
	return nil
}

// visible decides whether a walked node can be a row at all, before any filter
// looks at its contents.
func visible(node tree.Node, opts Options) bool {
	switch {
	case node.IsContainer, node.Stub:
		// Traversed, never printed: a container has nothing to set and a stub
		// has nothing behind it (§16.2, §1.6).
		return false
	case node.Archived && !opts.Scope.IsArchived():
		// §16.2: archive/ is not traversed unless you name it. Reached here
		// rather than in the walk because the unscoped walk has to descend
		// through archive/ to find nothing, and a scope inside archive/ makes
		// every node under it legitimate.
		return false
	case opts.Filter.Direct && readerDepth(node.Locator, opts.Scope) != 1:
		return false
	}
	return true
}

// readerDepth is how far beneath scope a locator is *as the output shows it*:
// containers do not count, because §16.2 makes them transparent.
//
// §17 is explicit that this is the measure `--direct` uses — "`list
// projects.acme --direct` shows that project's objectives, one container down,
// because a flag that returned nothing here would be measuring a structure the
// output never shows. The rule is: apply transparency, then take immediate
// children."
func readerDepth(loc, scope locator.Locator) int {
	depth := 0
	for _, seg := range loc[len(scope):] {
		if !locator.IsReserved(seg) {
			depth++
		}
	}
	return depth
}

// matches applies §17's filters to one entity. Every filter is answered from
// state.toml and the path except `--match`, which is the one read path that
// must open journals, because note bodies live only there (§17).
func matches(env *view.Env, ent view.Entity, f Filter) (bool, error) {
	if f.Status != "" && ent.EffectiveStatus != f.Status {
		return false, nil
	}
	if f.Priority != "" && ent.State.Priority != f.Priority {
		return false, nil
	}
	if f.Overdue && !ent.Overdue() {
		return false, nil
	}
	if f.Tags != nil && !f.Tags.Match(func(tag string) bool {
		return slices.Contains(ent.State.Tags, tag)
	}) {
		return false, nil
	}
	if f.Match != "" {
		return matchesText(ent, f.Match)
	}
	return true, nil
}

// matchesText is §17's `--match`: "the thing's own text: name, description,
// tags, journal note bodies".
//
// The journal is read last and only when nothing cheaper matched, so an
// unscoped `--match` over a large tree still stops at the first hit for most
// entities. It is not restricted, and §17 says why: "a filter that refuses to
// run without a locator would be a guess about tree size, and the cost is
// proportional to what you asked for".
//
// Matching is case-insensitive substring. §17 gives no grammar for `--match` —
// unlike `--tags`, which gets a whole one — and the flag is described as "text",
// so anything more would be a language nobody asked for.
func matchesText(ent view.Entity, needle string) (bool, error) {
	want := strings.ToLower(needle)
	for _, field := range append([]string{ent.State.Name, ent.State.Description}, ent.State.Tags...) {
		if strings.Contains(strings.ToLower(field), want) {
			return true, nil
		}
	}

	events, err := journal.ReadAll(truth.LogsDir(ent.Dir))
	if err != nil {
		return false, err
	}
	for _, e := range events {
		// Note bodies, on any event kind: a `note` is a field on all four
		// (§3.1), so a reason attached to a `set` is as findable as a bare one.
		if e.Note != "" && strings.Contains(strings.ToLower(e.Note), want) {
			return true, nil
		}
	}
	return false, nil
}

// orderStatuses puts the statuses that caused hiding into §1.7's order, so the
// "9 hidden (done, dropped)" line reads the same way every time (§0.2 forbids a
// map's iteration order reaching output).
func orderStatuses(seen map[string]bool) []string {
	var out []string
	for _, kind := range kindOrder {
		for _, status := range kindmeta.TerminalStatuses(kind) {
			if seen[status] && !slices.Contains(out, status) {
				out = append(out, status)
			}
		}
	}
	return out
}
