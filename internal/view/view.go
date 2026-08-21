// Package view is the read side of the tree: it derives, at read time, every
// value §2.5 forbids storing, and assembles the entities `show`, `list`, `log`,
// and `activity` print (design §16, §17).
//
// It is the counterpart of `render`, and the division between them is the one
// §2.5 draws. `render` turns truth into files, so it may only see values that
// are safe to commit — nothing clock-dependent, nothing ancestor-dependent,
// nothing aggregated. `view` turns truth into an answer for right now, so it
// computes exactly the things `render` may not:
//
//   - `attention`, and the staleness that follows from it (§3.6, §7);
//   - a key-result's `current`, `progress`, `pace`, and derived status (§4);
//   - effective status and archival dormancy, which are ancestor-dependent
//     (§1.6, §1.7);
//   - which skills reach an entity, and therefore which rules govern it (§5.2).
//
// The arithmetic itself is not here: §4 lives in `krvalue`, so that the status
// `measure` prints back from a reading it has not yet written and the status
// `show` prints from the same reading a moment later cannot disagree. What is
// here is the reading — which files to open, and how little of them.
//
// Nothing here writes. Nothing here is cached across a command, because the
// values are functions of *now* and a command is one instant (§3.6).
package view

import (
	"slices"
	"time"

	"github.com/colchuck-ai/para/internal/address"
	"github.com/colchuck-ai/para/internal/clock"
	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/tree"
	"github.com/colchuck-ai/para/internal/truth"
)

// Env is what a read command needs from outside itself: which tree, what time it
// is, and the resolver that answers §7's chained questions.
//
// Now is read once per command for the same reason a mutation's is (§3.6): every
// `days ago`, every `stale`, and every `pace` in one command's output has to be
// measured from one instant, or two lines of the same table disagree about today.
type Env struct {
	Root     string
	Now      time.Time
	Resolver *config.Resolver

	// Local is the zone `--local` renders in (§16.2.1). Output is UTC unless a
	// command asks otherwise; this is where the reader's own zone arrives.
	Local *time.Location
}

// NewEnv builds an Env for one read command against the tree rooted at root.
func NewEnv(root string, clk clock.Clock) *Env {
	now := clk.Now().Truncate(time.Second)
	return &Env{Root: root, Now: now, Resolver: config.NewResolver(root), Local: now.Location()}
}

// Entity is one entity or container as a read command sees it: its truth, and
// everything derived from it, the path, the journal, and the clock.
type Entity struct {
	Locator locator.Locator
	Kind    kindmeta.Kind
	State   truth.State

	// Dir is the entity's absolute filesystem path, carried so that a command
	// reading its journal does not resolve the locator a second time.
	Dir string

	// Container marks a container, which `list` never prints as a row and
	// always traverses through (§16.2).
	Container bool
	// Archived is location, not a field: under `archive/` is archived (§1.6).
	Archived bool

	// Created is the stored creation instant. HasCreated is false for truth
	// that has none or whose `created` will not parse — doctor's `invalid`
	// finding to report (§10), not a reason for a read to fail. The flag is
	// carried rather than left to a zero-time test because the zero time is a
	// real instant, and §4.2's window needs to know the difference.
	Created    time.Time
	HasCreated bool
	// Attention is §3.6's clock: the newest note or measurement, else `created`.
	Attention time.Time
	// AttentionKind is the kind of the event that set Attention — journal.KindNote
	// or journal.KindMeasurement — or empty when nothing beat `created` (§18.6,
	// §20). It lets review and list say what the clock last saw without a reader
	// opening ACTIVITY.md.
	AttentionKind journal.Kind
	// AttentionNote is the note's own text when AttentionKind is journal.KindNote,
	// and empty otherwise — a measurement's value is not the kind of thing a
	// reader distinguishes one measurement from another by, so only a note's
	// text is carried.
	AttentionNote string

	// Deadline is the last instant the stored `due` admits, and PastDue whether
	// Now is beyond it (§4.3, §17's `--overdue`).
	Deadline    time.Time
	HasDeadline bool
	PastDue     bool

	// EffectiveStatus is the entity's own status unless an ancestor's is
	// terminal, in which case it is the ancestor's (§1.7's cascade). For a
	// key-result it is the derived status (§4.3), or `dropped` where that is
	// stored.
	EffectiveStatus string
	// Terminal reports whether EffectiveStatus means this is over.
	Terminal bool
	// DormantUnder names the terminal ancestor quieting this entity, and is
	// empty when nothing is. It is reported rather than merely applied because
	// §16.1 requires `show` to say *why* something stopped appearing in `list`:
	// "its own fields do not explain why".
	DormantUnder locator.Locator

	// KeyResult carries §4's arithmetic, and is nil for every other kind.
	KeyResult *KeyResult
}

// Dormant reports whether the entity is quieted by something other than its own
// status — a terminal ancestor, or living under `archive/` (§1.6, §1.7).
func (e Entity) Dormant() bool { return len(e.DormantUnder) > 0 || e.Archived }

// Name is the entity's name, falling back to its id so that a row is never
// blank. A `name` is required of every kind (§15), so the fallback only fires on
// truth `doctor` would report as invalid.
func (e Entity) Name() string {
	if e.State.Name != "" {
		return e.State.Name
	}
	return e.ID()
}

// ID is the entity's last locator segment.
func (e Entity) ID() string {
	if len(e.Locator) == 0 {
		return ""
	}
	return e.Locator[len(e.Locator)-1]
}

// Overdue is §17's `--overdue` filter: open and past `due`. Terminal is what
// "open" excludes — something done last week is not overdue, it is done — and
// `missed` is deliberately not terminal (§1.7), so a blown deadline stays
// visible, which is the whole point of the flag.
func (e Entity) Overdue() bool { return e.PastDue && !e.Terminal }

// KeyResult is §4's derived arithmetic for one key-result. Every field here is
// forbidden from truth (§2.5), which is why they are computed together and in
// one place.
type KeyResult struct {
	// Current is the newest reading, and HasCurrent is false before the first
	// one — a key-result with a target and no measurement yet.
	Current    krvalue.Value
	HasCurrent bool

	// Start is the baseline §4.1 resolves: the explicit `start` if there is one,
	// else the first measurement ever logged.
	Start    krvalue.Value
	HasStart bool

	// Target is the key-result's target as stored.
	Target    krvalue.Value
	HasTarget bool

	// Outlook is §4.2's progress and pace and §4.3's status, from krvalue.
	Outlook krvalue.Outlook
}

// Threshold is a resolved §7 knob and where it came from.
//
// The provenance is not decoration: §7 says outright that chained resolution is
// "only defensible if it is visible", and §16.1 requires `show` to name the file
// a threshold came from. Carrying them together is what makes forgetting it hard.
type Threshold struct {
	// Key is the config key resolved, which differs by kind: a skill's
	// staleness threshold is `review.cadence` and every other kind's is
	// `<kind>.stale-after` (§20).
	Key   string
	Value float64
	// From is the config.toml that supplied the value, root-relative and
	// slash-separated, or "" when the built-in default did.
	From string
	// Found is false for a key with no value and no default, which §7 defines as
	// the check never firing.
	Found bool
}

// Load reads the entity loc names and derives everything about it.
//
// It reads one state.toml, the journal files backing §3.6's clock, and — for a
// key-result — the two ends of its measurement history for §4's arithmetic. It
// walks to root only for the ancestor-dependent values §1.7's cascade defines,
// which is a read and not a write, so §2.3's "nothing walks to root" does not
// reach it.
func (e *Env) Load(loc locator.Locator) (Entity, error) {
	kind, err := tree.KindAt(loc)
	if err != nil {
		return Entity{}, err
	}
	exists, err := tree.Exists(e.Root, loc)
	if err != nil {
		return Entity{}, err
	}
	if !exists {
		stub, err := tree.IsStub(e.Root, loc)
		if err != nil {
			return Entity{}, err
		}
		if stub {
			// A stub has no noun and no address (R11): it is named by its
			// on-disk path, the same way doctor's own stub findings are.
			path, pathErr := loc.Path()
			if pathErr != nil {
				path = loc.String()
			}
			return Entity{}, paraerr.Newf(paraerr.KindNotFound,
				"%s/ is a stub — a locator segment with no entity behind it (§1.6)", path)
		}
		return Entity{}, paraerr.Newf(paraerr.KindNotFound, "%s does not exist", viewAddr(loc))
	}
	dir, err := tree.ResolvePath(e.Root, loc)
	if err != nil {
		return Entity{}, err
	}
	state, err := truth.ReadState(dir)
	if err != nil {
		return Entity{}, err
	}
	return e.Derive(loc, kind, dir, state)
}

// Derive assembles an Entity from truth already in hand, which is the shape the
// walk needs: `list` has read the state.toml as part of finding the entity and
// must not read it twice.
func (e *Env) Derive(loc locator.Locator, kind kindmeta.Kind, dir string, state truth.State) (Entity, error) {
	out := Entity{
		Locator:   loc,
		Kind:      kind,
		State:     state,
		Dir:       dir,
		Container: kind == kindmeta.KindContainer,
		Archived:  loc.IsArchived(),
	}
	out.Created, out.HasCreated = ptime.StoredAt(state.Created)
	out.Deadline, out.HasDeadline = ptime.DeadlineOf(state.Due)
	out.PastDue = out.HasDeadline && e.Now.After(out.Deadline)

	attentionEvent, ok, err := journal.AttentionEvent(truth.LogsDir(dir), out.Created)
	if err != nil {
		return Entity{}, err
	}
	if ok {
		out.Attention = attentionEvent.At
		out.AttentionKind = attentionEvent.Kind
		if attentionEvent.Kind == journal.KindNote {
			out.AttentionNote = attentionEvent.Note
		}
	} else {
		out.Attention = out.Created
	}

	if kind == kindmeta.KindKeyResult {
		kr, err := e.keyResult(loc, state, dir, out)
		if err != nil {
			return Entity{}, err
		}
		out.KeyResult = &kr
		out.EffectiveStatus = string(kr.Outlook.Status)
	} else {
		out.EffectiveStatus = state.Status
	}
	out.Terminal = kindmeta.IsTerminal(kind, out.EffectiveStatus)

	// The cascade, last, because it can replace both of the values computed
	// above: "a thing's effective status is its own, unless any ancestor's is
	// terminal" (§1.7). Nothing is written to a descendant; it is read from the
	// path.
	ancestor, status, err := e.terminalAncestor(loc)
	if err != nil {
		return Entity{}, err
	}
	if len(ancestor) > 0 {
		out.DormantUnder = ancestor
		out.EffectiveStatus = status
		out.Terminal = true
	}
	return out, nil
}

// EffectiveStatuses is every value EffectiveStatus can hold, in §1.7's order —
// the settable vocabulary every kind draws from, then the derived ones only a
// key-result reaches (§4.3), each spelled once.
//
// It lives here because this is where the value is produced: the two branches
// above are `state.Status` and `krvalue.Status`, and the cascade replaces one
// with another of the same two. Anything filtering or completing on
// `--status` is asking about *this* set, which is strictly wider than any one
// kind's settable statuses — `list --status on-track` is a legal, useful
// request that no kind's `set` would ever accept.
func EffectiveStatuses() []string {
	out := kindmeta.AllStatuses()
	for _, s := range []krvalue.Status{
		krvalue.StatusOnTrack, krvalue.StatusAtRisk,
		krvalue.StatusMissed, krvalue.StatusAchieved, krvalue.StatusDropped,
	} {
		if !slices.Contains(out, string(s)) {
			out = append(out, string(s))
		}
	}
	return out
}

// terminalAncestor finds the nearest ancestor whose own status is terminal, and
// that status (§1.7's cascade).
//
// It walks upward and stops at the first hit, because "any ancestor" makes the
// nearest one sufficient and there is nothing further up that could un-quiet it.
// A container is skipped: it has no status field at all, so it can neither quiet
// nor shield what is beneath it.
func (e *Env) terminalAncestor(loc locator.Locator) (locator.Locator, string, error) {
	for i := len(loc) - 1; i > 0; i-- {
		ancestor := loc[:i]
		kind, err := tree.KindAt(ancestor)
		if err != nil || kind == kindmeta.KindContainer {
			continue
		}
		if len(kindmeta.TerminalStatuses(kind)) == 0 {
			continue
		}
		exists, err := tree.Exists(e.Root, ancestor)
		if err != nil {
			return nil, "", err
		}
		if !exists {
			continue
		}
		dir, err := tree.ResolvePath(e.Root, ancestor)
		if err != nil {
			return nil, "", err
		}
		state, err := truth.ReadState(dir)
		if err != nil {
			return nil, "", err
		}
		// A key-result's terminal `achieved` is derived rather than stored, so
		// only a stored status can quiet a descendant here — and a key-result
		// has no descendants, so nothing is lost.
		if kindmeta.IsTerminal(kind, state.Status) {
			return ancestor, state.Status, nil
		}
	}
	return nil, "", nil
}

// Stale resolves the staleness threshold that applies to ent and reports
// whether it has been passed (§7, §16.1, §20).
//
// Which key that is depends on the kind and is config.StaleKey's answer, not
// this package's: a skill is measured by `review.cadence` and a container by
// nothing at all (§20).
func (e *Env) Stale(ent Entity) (Threshold, bool, error) {
	key, ok := config.StaleKey(ent.Kind)
	if !ok {
		return Threshold{}, false, nil
	}
	th, err := e.Threshold(ent.Locator, key)
	if err != nil || !th.Found {
		return th, false, err
	}
	return th, float64(e.DaysSince(ent.Attention)) > th.Value, nil
}

// AtRiskPace resolves `key-result.at-risk-pace` at loc (§4.3, §7).
func (e *Env) AtRiskPace(loc locator.Locator) (Threshold, error) {
	return e.Threshold(loc, config.KeyAtRiskPace)
}

// Threshold resolves one numeric §7 knob through the ancestor chain, keeping
// the level that supplied it.
func (e *Env) Threshold(loc locator.Locator, key string) (Threshold, error) {
	res, err := e.Resolver.Resolve(loc, key)
	if err != nil {
		return Threshold{}, err
	}
	out := Threshold{Key: key, Found: res.Found}
	if !res.Found {
		return out, nil
	}
	value, ok := res.Float()
	if !ok {
		return Threshold{}, paraerr.Newf(paraerr.KindValidation, "%s is not a number", key)
	}
	out.Value = value
	if level, ok := res.Source(); ok {
		out.From = level.File
	}
	return out, nil
}

// DaysSince is whole days from t to now, counted on the UTC calendar — what
// §7's `stale-after` measures in.
//
// Days on a calendar rather than elapsed 24-hour spans, because `stale-after
// 14` is a number of days and every generated file already agrees on which day
// an instant falls on (§3.5). Truncating elapsed hours instead would make a
// threshold cross at a different moment depending on the time of day the last
// note happened to land.
//
// UTC rather than the reader's zone, because a threshold is committed: two
// people running `review` on one tree from two continents must get one answer,
// which is the same argument §3.5 makes for the files. Displayed ages are the
// negotiable case and take DaysSinceIn.
func (e *Env) DaysSince(t time.Time) int { return e.DaysSinceIn(t, time.UTC) }

// DaysUntil is whole UTC days from now to t, the same count in the other
// direction.
func (e *Env) DaysUntil(t time.Time) int { return e.DaysUntilIn(t, time.UTC) }

// DaysSinceIn is §16.1's "31 days ago", counted on loc's calendar.
//
// The zone belongs here and not only on the timestamp beside it: `--local` moves
// an attention of 2026-03-05T02:00Z back to 2026-03-04, and a line reading
// "attention 2026-03-04 today" is two answers to one question. Whichever
// calendar the date is printed on is the one the age has to be counted on.
func (e *Env) DaysSinceIn(t time.Time, loc *time.Location) int {
	if t.IsZero() {
		return 0
	}
	return calendarDays(e.Now, loc) - calendarDays(t, loc)
}

// DaysUntilIn is §16.1's "in 181 days", counted on loc's calendar.
func (e *Env) DaysUntilIn(t time.Time, loc *time.Location) int {
	if t.IsZero() {
		return 0
	}
	return calendarDays(t, loc) - calendarDays(e.Now, loc)
}

// calendarDays is the number of whole days since the epoch on loc's calendar.
// Truncating the instant would be wrong for anything before 1970 and is no
// simpler.
func calendarDays(t time.Time, loc *time.Location) int {
	y, m, d := t.In(loc).Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// viewAddr is loc's dotted address (R24), the same conversion every other
// package that names a Locator in an error message asks of address.String
// (internal/mutate's relocateAddr; internal/cli's entityLocatorString and
// findingLocatorString; internal/doctor's doctorAddr; internal/rebuild's
// rebuildAddr; internal/query's queryAddr). loc reaches here only after
// tree.KindAt has already derived a kind from its shape, which is the same
// shape question address.FromLocator asks independently — so the raw
// Locator string is a defensive fallback only, for the one case the two
// disagree, not a path this package's own tests exercise.
func viewAddr(loc locator.Locator) string {
	s, err := address.String(loc)
	if err != nil {
		return loc.String()
	}
	return s
}
