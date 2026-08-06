// Package mutate implements the verbs that write (design §18): add, set,
// unset, note, measure, move, remove, archive, and unarchive.
//
// Every one of them is the same shape, and this file is that shape: validate
// against the §15 field matrix, decide what actually changed, build the
// journal events for it, re-render every projection the subject owns, and hand
// the whole set to writeset for one ordered, truth-first write (§0.2). A verb
// file below decides what changed; nothing below decides how it is written.
//
// Three rules are enforced here rather than in each verb, because each of them
// is a claim about the tree that a single verb could quietly break:
//
//   - **A projection is written only when its bytes differ from what is on
//     disk.** Write-through is complete — every projection is re-derived on
//     every mutation (§2.3) — but a file whose bytes did not change is a file
//     nothing wrote. That is what makes §23's "mutations print what they
//     wrote" honest, and it is why `measure` does not touch README.md: §2.3
//     names it in prose, but a key-result's frontmatter carries no derived
//     value (§2.5), so there is nothing in it for a measurement to change.
//
//   - **Nothing walks a subtree and nothing walks to root** (§2.3). A mutation
//     reaches exactly its subject's files, plus the parent's journal and
//     ACTIVITY.md when containment changed. `add` reaches one further, to the
//     eager child container §18.1 requires. The four relocating verbs are the
//     other exception, and §19 names them as such — "the operations that touch
//     more than one entity's worth of bytes" — but even they stay narrow: a
//     descendant of a moved entity gets its README.md re-rendered and nothing
//     else, because that is the only projection carrying a locator.
//
//   - **ACTIVITY.md takes the cheap incremental write only where it is
//     provably equivalent to the full one** — see plan.fullActivity.
package mutate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/render"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
	"github.com/colchuck-ai/para/internal/writeset"
)

// Env is everything a mutation needs from outside itself: which tree, what
// time it is, and the resolver that answers §7's chained questions.
//
// Now is read once per command rather than per file, so every event and every
// timestamp a single mutation writes names the same instant. A mutation that
// straddled two instants would be one whose journal could not be replayed into
// the state it produced.
type Env struct {
	// Root is the tree root, as an absolute OS path.
	Root string
	// Now is the instant the whole mutation happens at.
	Now time.Time
	// Resolver answers config questions, caching each config.toml it reads —
	// one per command, as Phase 7 fixed.
	Resolver *config.Resolver
}

// NewEnv builds an Env for one command against the tree rooted at root.
//
// Now is truncated to the second, which is the precision every timestamp in the
// design is written at: §3.1's journal lines, §4.4's CSV column, §8.3's truth
// files, and §3.4's filenames all stop there. Carrying microseconds would make
// two readings a millisecond apart distinct to §3.1's exact-instant uniqueness
// check and identical in the MEASUREMENTS.csv row it produces — one duplicate
// row that no rebuild could remove, because both events are genuinely there.
func NewEnv(root string, clk clock.Clock) *Env {
	return &Env{Root: root, Now: clk.Now().Truncate(time.Second), Resolver: config.NewResolver(root)}
}

// Local is the offset `--at`'s progressive precision zero-fills in (§15.1):
// "what you type is local; what is stored is UTC". It comes from the clock so
// that a test can fix it and never depend on the host zone (§0.2).
func (e *Env) Local() *time.Location { return e.Now.Location() }

// Change is one field that actually changed, as `set` reports it and as the
// journal records it (§3.1).
type Change struct {
	Field kindmeta.Field
	From  string
	To    string
}

// NoOp is a field already holding the value it was asked for: §15's rule that
// setting a field to its current value writes nothing, reported so the user
// learns why nothing happened rather than assuming it worked.
type NoOp struct {
	Field kindmeta.Field
	Value string
}

// Result is what a mutation did. Wrote carries every file touched, in write
// order, as a root-relative slash-separated path — the form §23 prints.
type Result struct {
	Locator locator.Locator
	Kind    kindmeta.Kind
	Changes []Change
	NoOps   []NoOp
	Wrote   []string

	// NoteRecorded is true when a --note was written as a note event of its
	// own, which happens only when nothing else about the mutation changed
	// (§26: "no change (status already blocked); note recorded").
	NoteRecorded bool

	// Measured is the derived line `measure` prints (§26), and nil for every
	// other verb.
	Measured *Measured
}

// subject is one entity, container, or the tree root, loaded from disk and
// ready to render: everything render.In needs that does not depend on the
// mutation itself.
type subject struct {
	env  *Env
	loc  locator.Locator
	kind kindmeta.Kind
	dir  string // absolute OS path
	rel  string // root-relative, slash-separated; "" for the root

	state truth.State
	tree  truth.Tree
	cfg   render.Config
}

// open loads the subject a locator names. The empty locator is the tree root,
// whose identity lives in tree.toml rather than state.toml (§8.1).
func (e *Env) open(loc locator.Locator) (*subject, error) {
	kind, err := tree.KindAt(loc)
	if err != nil {
		return nil, err
	}
	s, err := e.subjectAt(loc, kind)
	if err != nil {
		return nil, err
	}

	if len(loc) == 0 {
		if s.tree, err = truth.ReadTree(s.dir); err != nil {
			return nil, err
		}
		return s, nil
	}
	if s.state, err = truth.ReadState(s.dir); err != nil {
		return nil, err
	}
	return s, nil
}

// subjectAt builds a subject without reading its truth — the shape `add` needs,
// since the truth it is about to write does not exist yet.
func (e *Env) subjectAt(loc locator.Locator, kind kindmeta.Kind) (*subject, error) {
	rel := ""
	if len(loc) > 0 {
		var err error
		if rel, err = loc.Path(); err != nil {
			return nil, err
		}
	}
	cfg, err := e.Resolver.RenderConfig(loc)
	if err != nil {
		return nil, err
	}
	return &subject{
		env:  e,
		loc:  loc,
		kind: kind,
		dir:  filepath.Join(e.Root, filepath.FromSlash(rel)),
		rel:  rel,
		cfg:  cfg,
	}, nil
}

// plan is one subject's contribution to a mutation: the events its journal
// gains and the truth files it rewrites. Which projections get written is not
// a decision a verb makes — it is every file the subject owns, filtered to the
// ones whose bytes actually changed.
type plan struct {
	subj   *subject
	events []journal.Event

	// writeState re-encodes state.toml. False for `note` and `measure`, which
	// change the journal without changing what the thing is.
	writeState bool

	// config is a new config.toml, or nil to leave it alone. `add` writes an
	// empty one so every .para/ has the same shape (§5.1, §8.4); `config set`
	// writes the only one that ever has content at write time.
	config []byte

	// creating marks a subject that does not exist yet, which forces
	// ACTIVITY.md's full mode and creates the empty logs/ directory.
	creating bool

	// createdMoved marks a mutation that changed `created`, which moves
	// ACTIVITY.md's "Created." line to a different day and so also forces full
	// mode (see render.CreatedDay).
	createdMoved bool

	// onlyProjections restricts which projections are rewritten. It is empty
	// for a subject — every file it owns — and holds ACTIVITY.md alone for a
	// parent, because a parent's stored fields say nothing about its children
	// (§8.2), so its README cannot have changed and §2.3's invariant is exact
	// about what a containment change reaches.
	onlyProjections []render.Renderer
}

// apply renders and writes one mutation: subjects first, the parents last.
func apply(env *Env, subjects []*plan, parents []*plan) ([]string, error) {
	var m writeset.Mutation
	for _, p := range subjects {
		built, err := p.build()
		if err != nil {
			return nil, err
		}
		m.Subjects = append(m.Subjects, built)
	}
	for _, p := range parents {
		built, err := p.build()
		if err != nil {
			return nil, err
		}
		m.Parents = append(m.Parents, built)
	}
	if len(m.Subjects) == 0 && len(m.Parents) == 0 {
		// A relocation can genuinely write nothing beyond the rename: a skill
		// has no parent journal (§1.4), so `move skills.a skills.b` whose
		// rendered files all match byte for byte leaves nothing for Apply to do.
		return nil, nil
	}

	ops, err := writeset.Apply(m)
	return env.wrote(ops), err
}

// wrote converts the write record into the root-relative paths §23 prints,
// keeping only the operations that put bytes in a file. A directory creation is
// not a file anyone wrote, and git does not carry an empty one anyway; a rename
// and a delete are not writes either, and the relocating verbs report them in
// their own summary lines instead (§26), where a destination and a blast radius
// can be named.
func (e *Env) wrote(ops writeset.Ops) []string {
	var out []string
	for _, op := range ops {
		if op.Kind != writeset.OpAppend && op.Kind != writeset.OpWrite {
			continue
		}
		out = append(out, e.rel(op.Path))
	}
	return out
}

// rel is an absolute path as §23 prints it: relative to the tree root, with
// forward slashes on every platform.
func (e *Env) rel(path string) string {
	r, err := filepath.Rel(e.Root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(r)
}

// build turns a plan into the file set writeset applies.
func (p *plan) build() (writeset.Subject, error) {
	// log.rotate-bytes is chain-resolved at the subject itself (§3.4, §7), so a
	// project that sets its own never decides when its parent's journal rotates.
	rotate, err := p.subj.env.Resolver.RotateBytes(p.subj.loc)
	if err != nil {
		return writeset.Subject{}, err
	}
	out := writeset.Subject{Dir: p.subj.dir, Events: p.events, Config: p.config, RotateBytes: rotate}
	if p.creating {
		// The journal starts empty (§18.1), so nothing else would create it.
		out.Dirs = []string{truth.LogsDir(p.subj.dir)}
	}
	if p.writeState {
		state, encodeErr := truth.EncodeState(p.subj.state)
		if encodeErr != nil {
			return out, encodeErr
		}
		out.State = state
	}

	projections, err := p.render()
	if err != nil {
		return out, err
	}
	out.Projections = projections
	return out, nil
}

// render produces the projections this plan must write: every file the subject
// owns (or `only` of them), rendered from truth as it will be after the
// mutation, minus the ones already holding those exact bytes.
func (p *plan) render() ([]writeset.File, error) {
	in := render.In{
		Locator:  p.subj.loc,
		Kind:     p.subj.kind,
		State:    p.subj.state,
		Tree:     p.subj.tree,
		Config:   p.subj.cfg,
		Existing: map[string][]byte{},
	}

	full := p.fullActivity()
	events, days, err := p.subj.eventsForRender(p.events, full)
	if err != nil {
		return nil, err
	}
	in.Events = events

	renderers := p.onlyProjections
	if len(renderers) == 0 {
		renderers = render.For(in)
	}
	renderers = withActivityMode(renderers, days)

	// CLAUDE.md's import list is the one thing a renderer needs that is not in
	// the subject's own truth (§6.1). It is loaded only when that renderer is in
	// the set, so a mutation on an entity never reads the rules directory.
	if slices.Contains(renderers, render.Claude) {
		ids, err := tree.SkillIDs(p.subj.env.Root)
		if err != nil {
			return nil, err
		}
		in.Rules = render.RuleFilenames(ids)
	}

	existing := map[string][]byte{}
	artifacts, err := render.Collect(in, renderers, func(path string) ([]byte, error) {
		data, err := os.ReadFile(filepath.Join(p.subj.env.Root, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			return nil, nil
		}
		if err != nil {
			return nil, paraerr.Wrap(paraerr.KindInternal, err, fmt.Sprintf("reading %s", path))
		}
		existing[path] = data
		return data, nil
	})
	if err != nil {
		return nil, err
	}

	var out []writeset.File
	for _, a := range artifacts {
		if prior, ok := existing[a.Path]; ok && bytes.Equal(prior, a.Bytes) {
			// Write-through re-derives every projection; it does not rewrite a
			// file that already holds the right bytes.
			continue
		}
		out = append(out, writeset.File{
			Path:  filepath.Join(p.subj.env.Root, filepath.FromSlash(a.Path)),
			Bytes: a.Bytes,
		})
	}
	return out, nil
}

// fullActivity decides ACTIVITY.md's mode (§3.5).
//
// Incremental — re-derive today's section, copy every prior day through — is
// the cheap write the design is built around, and it is correct only when the
// sections it does not visit cannot have changed. Three cases break that, and
// each of them falls back to re-deriving the whole file from the whole journal:
//
//   - a key-result, whose every measurement line ends in "N% of target", a
//     function of start, target, and the oldest reading (render refuses
//     incremental for one outright);
//   - a mutation that moves `created`, whose line lives in whichever day
//     section `created` names;
//   - a file that does not exist yet, which has no prior days to copy — so
//     splicing today onto nothing would silently drop every day before it.
//
// The third case is what makes a mutation on a hand-planted or hand-repaired
// tree produce the same bytes rebuild would, rather than a file doctor then
// reports as drifted.
func (p *plan) fullActivity() bool {
	if p.creating || p.createdMoved || p.subj.kind == kindmeta.KindKeyResult {
		return true
	}
	_, err := os.Stat(filepath.Join(p.subj.dir, "ACTIVITY.md"))
	return err != nil
}

// eventsForRender returns the journal as it will be after the mutation, and
// the days ACTIVITY.md should re-derive ("" for full mode).
//
// In incremental mode it reads only the journal files that can hold the
// affected days, which is what §3.5 means by "a write reads one journal file,
// not all of them".
func (s *subject) eventsForRender(added []journal.Event, full bool) ([]journal.Event, []string, error) {
	logs := truth.LogsDir(s.dir)
	if full {
		prior, err := journal.ReadAll(logs)
		if err != nil {
			return nil, nil, err
		}
		return merge(prior, added), nil, nil
	}

	days := render.DaysOf(added)
	since := startOfDay(added)
	prior, _, err := journal.ReadOnOrAfter(logs, since)
	if err != nil {
		return nil, nil, err
	}
	return merge(prior, added), days, nil
}

// startOfDay is midnight UTC on the earliest day the new events fall on — the
// point from which the journal must be read for every re-derived section to be
// complete.
func startOfDay(events []journal.Event) time.Time {
	earliest := time.Time{}
	for _, e := range events {
		if earliest.IsZero() || e.At.Before(earliest) {
			earliest = e.At
		}
	}
	return earliest.UTC().Truncate(24 * time.Hour)
}

// merge orders the journal as it will be on disk: by `at`, never by position
// (§3.1), with a new event landing after an existing one that shares its
// instant, which is the order the append itself produces.
func merge(prior, added []journal.Event) []journal.Event {
	out := make([]journal.Event, 0, len(prior)+len(added))
	out = append(out, prior...)
	out = append(out, added...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// withActivityMode substitutes the ACTIVITY.md renderer the write path chose.
// The mode is a property of this write, not of the file, so it cannot live in
// render.For — which is also what rebuild and doctor call, and both of them
// always want full mode.
func withActivityMode(renderers []render.Renderer, days []string) []render.Renderer {
	out := make([]render.Renderer, len(renderers))
	copy(out, renderers)
	for i, r := range out {
		if _, ok := r.(render.ActivityRenderer); ok {
			out[i] = render.ActivityRenderer{Days: days}
		}
	}
	return out
}

// parentPlan is the parent's half of a containment change: its own child event
// and the ACTIVITY.md that event lands in, and nothing else (§2.3, §3.3).
//
// A skill has no parent plan at all. Its directory sits in .agents/skills/,
// which is shared with content para does not own and has no journal — so there
// is no parent whose set of children changed (§1.4, §3.3).
func (e *Env) parentPlan(loc locator.Locator, events []journal.Event) (*plan, error) {
	if len(loc) < 2 {
		return nil, nil
	}
	return e.parentPlanAt(loc[:len(loc)-1], events)
}

// parentPlanAt is parentPlan addressed by the parent rather than by the child,
// which is the shape a relocation needs: its two ends are two parents, and one of
// them may not be able to hold an event at all.
//
// Three cases return no plan, and each is a place where there is no journal
// rather than a place para chose not to write one:
//
//   - a skill's parent, which is .agents/skills/ — a directory shared with
//     content para does not own, holding no .para/ (§1.4, §3.3);
//   - an archive stub, which is a bare directory by definition (§1.6);
//   - a parent that is not there at all, which `remove` can produce: deleting
//     the last thing beneath a stub takes the stub with it.
func (e *Env) parentPlanAt(parent locator.Locator, events []journal.Event) (*plan, error) {
	if len(parent) == 0 || parent.Bucket() == "skills" {
		return nil, nil
	}
	exists, err := tree.Exists(e.Root, parent)
	if err != nil || !exists {
		return nil, err
	}
	subj, err := e.open(parent)
	if err != nil {
		return nil, err
	}
	return &plan{subj: subj, events: events, onlyProjections: []render.Renderer{render.Activity}}, nil
}

// parents wraps the single optional parent the non-relocating verbs have, since
// only a relocation ever has two (§18.3).
func parents(p *plan) []*plan {
	if p == nil {
		return nil
	}
	return []*plan{p}
}
