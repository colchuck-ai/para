package mutate

import (
	"time"

	"github.com/colchuck-ai/para/internal/config"
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/kindmeta"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
	"github.com/colchuck-ai/para/internal/truth"
)

// Measured is the reading `measure` reports back (§26): the value as logged,
// its decimal equivalent, the progress it represents, and the status that
// follows from it.
type Measured struct {
	Value    krvalue.Value
	Progress float64
	Status   krvalue.Status

	// HasProgress is false when the arithmetic is not available — a
	// key-result whose target does not parse, which is doctor's `invalid`
	// finding rather than a reason to refuse the reading (§10).
	HasProgress bool
}

// Measure logs a reading against a key-result (§18.6).
//
// The value must follow the key-result's own `type`, and a mismatch names both
// (§18.6). A reading colliding with an existing one at the exact stored instant
// is refused (§3.1) — progressive precision makes `--at 2026-01-03` an exact
// midnight, so typing it twice is a collision while `2026-01-03` and
// `2026-01-03T09:02` are not.
func (e *Env) Measure(loc locator.Locator, value, at, note string) (Result, error) {
	subj, err := e.load(loc)
	if err != nil {
		return Result{}, err
	}
	if subj.kind != kindmeta.KindKeyResult {
		return Result{}, paraerr.Newf(paraerr.KindValidation,
			"%s is a %s — only a key-result takes measurements", loc, subj.kind)
	}

	typ := krvalue.Type(subj.state.Type)
	v, err := krvalue.Parse(typ, value)
	if err != nil {
		return Result{}, paraerr.Newf(paraerr.KindValidation,
			"value %s is not a %s (%s)", value, typ, grammarOf(typ))
	}

	when, err := e.eventTime(at)
	if err != nil {
		return Result{}, err
	}
	prior, err := journal.ReadAll(truth.LogsDir(subj.dir))
	if err != nil {
		return Result{}, err
	}
	if err := journal.CheckMeasurementUnique(prior, when); err != nil {
		return Result{}, e.collision(when)
	}

	events := []journal.Event{journal.NewMeasurement(when, v.Raw, note)}
	measured, err := e.derive(subj, v, append(prior, events...))
	if err != nil {
		return Result{}, err
	}

	wrote, err := apply(e, []*plan{{subj: subj, events: events}}, nil)
	return Result{Locator: loc, Kind: subj.kind, Wrote: wrote, Measured: measured}, err
}

// collision restates a duplicate-instant refusal in both the stored form and
// the local one §26 prints, since the two differ by exactly the conversion the
// user did not perform.
func (e *Env) collision(when time.Time) error {
	return paraerr.Newf(paraerr.KindConflict, "a measurement already exists at %s (%s local)",
		when.UTC().Format(time.RFC3339), when.In(e.Local()).Format(time.RFC3339))
}

// derive computes the line §26 prints back: progress against the key-result's
// baseline and target, and the status that follows (§4.2, §4.3).
func (e *Env) derive(subj *subject, v krvalue.Value, events []journal.Event) (*Measured, error) {
	out := &Measured{Value: v}
	typ := krvalue.Type(subj.state.Type)

	oldest, hasOldest := oldestReading(typ, events)
	baseline, ok := krvalue.Baseline(typ, subj.state.Start, oldest, hasOldest)
	if !ok {
		return out, nil
	}
	target, err := krvalue.Parse(typ, subj.state.Target)
	if err != nil {
		return out, nil
	}

	atRisk, err := e.atRiskPace(subj.loc)
	if err != nil {
		return nil, err
	}
	created, hasCreated := ptime.StoredAt(subj.state.Created)
	due, hasDue := ptime.DeadlineOf(subj.state.Due)

	// The same call the read path makes (view), so `measure`'s reply and a
	// later `show` cannot disagree about the key-result they both describe.
	outlook := krvalue.Assess(krvalue.Assessment{
		Type:          typ,
		Baseline:      baseline,
		Target:        target.Decimal,
		Current:       v.Decimal,
		HasCurrent:    true,
		Created:       created,
		HasCreated:    hasCreated,
		Deadline:      due,
		HasDeadline:   hasDue,
		Now:           e.Now,
		AtRiskPace:    atRisk.value,
		HasAtRiskPace: atRisk.set,
		Dropped:       subj.state.Status == string(krvalue.StatusDropped),
	})
	out.Progress, out.HasProgress = outlook.Progress, outlook.HasProgress
	out.Status = outlook.Status
	return out, nil
}

// threshold is a resolved config threshold: the value, and whether any level
// supplied one at all. The two are separate because §7's thresholds have no
// built-in default, so "unset everywhere" is an answer rather than a zero.
type threshold struct {
	value float64
	set   bool
}

// atRiskPace resolves key-result.at-risk-pace through the chain (§7). Unset
// everywhere means the check never fires, so a key-result in a tree that never
// set one never reads at-risk.
func (e *Env) atRiskPace(loc locator.Locator) (threshold, error) {
	res, err := e.Resolver.Resolve(loc, config.KeyAtRiskPace)
	if err != nil {
		return threshold{}, err
	}
	if !res.Found {
		return threshold{}, nil
	}
	v, ok := res.Float()
	return threshold{value: v, set: ok}, nil
}

// oldestReading is the earliest measurement in events, by `at` and never by
// position (§3.1) — a backdated --at can put it anywhere in the file. It is
// what krvalue.Baseline falls back to when a key-result set no explicit start
// (§4.1); this package finds it because only this package holds the events.
func oldestReading(typ krvalue.Type, events []journal.Event) (krvalue.Value, bool) {
	oldest := time.Time{}
	var found krvalue.Value
	for _, ev := range events {
		if ev.Kind != journal.KindMeasurement {
			continue
		}
		v, err := krvalue.Parse(typ, ev.Value)
		if err != nil {
			continue
		}
		if oldest.IsZero() || ev.At.Before(oldest) {
			oldest, found = ev.At, v
		}
	}
	return found, !oldest.IsZero()
}

// grammarOf names what a type's values look like, for the mismatch message
// §18.6 requires to name both (§26: "type ratio expects <numerator>/…").
func grammarOf(typ krvalue.Type) string {
	switch typ {
	case krvalue.TypeRatio:
		return "type ratio expects <numerator>/<denominator>"
	case krvalue.TypeBoolean:
		return "type boolean expects true or false"
	default:
		return "type number expects a decimal number"
	}
}
