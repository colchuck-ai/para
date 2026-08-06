package view

import (
	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/krvalue"
	"github.com/colchuck-ai/para/internal/locator"
	"github.com/colchuck-ai/para/internal/truth"
)

// keyResult derives §4's quantities for one key-result from its stored fields
// and the two ends of its measurement history.
//
// Two ends, and not the history: §4.2's `current` is the newest reading and
// §4.1's default baseline is the oldest, and nothing else in §16's output is a
// function of what happened in between. `log` and `activity` are where the
// middle of a journal is wanted, and they are asked for by name.
//
// Nothing here decides what the numbers mean — krvalue.Assess does, and it is
// the same call `measure` makes (§18.6), so the status printed back from a
// reading and the status `show` prints for it a moment later are one function
// of one set of inputs.
func (e *Env) keyResult(loc locator.Locator, state truth.State, dir string, ent Entity) (KeyResult, error) {
	typ := krvalue.Type(state.Type)
	var out KeyResult

	logs := truth.LogsDir(dir)
	if newest, ok, err := journal.LatestOf(logs, journal.KindMeasurement); err != nil {
		return KeyResult{}, err
	} else if ok {
		if v, err := krvalue.Parse(typ, newest.Value); err == nil {
			out.Current, out.HasCurrent = v, true
		}
	}

	// The oldest reading is only consulted where it could be the baseline: a
	// key-result with an explicit `start` never falls back to it (§4.1), and one
	// with a boolean type cannot have one at all.
	var oldest krvalue.Value
	var hasOldest bool
	if state.Start == "" && typ != krvalue.TypeBoolean {
		if first, ok, err := journal.OldestOf(logs, journal.KindMeasurement); err != nil {
			return KeyResult{}, err
		} else if ok {
			oldest, hasOldest = parseReading(typ, first.Value)
		}
	}

	baseline, hasBaseline := krvalue.Baseline(typ, state.Start, oldest, hasOldest)
	if hasBaseline {
		// Start is reported as a value rather than a decimal because §16.1
		// prints the reading as written — "480/9000", not 0.053 (§4.1).
		if state.Start != "" {
			out.Start, out.HasStart = parseReading(typ, state.Start)
		} else {
			out.Start, out.HasStart = oldest, hasOldest
		}
	}
	target, hasTarget := parseReading(typ, state.Target)
	out.Target, out.HasTarget = target, hasTarget

	if !hasBaseline || !hasTarget {
		// Truth that does not add up is doctor's `invalid` finding (§10). A
		// stored `dropped` still stands, because nobody derived it.
		if state.Status == string(krvalue.StatusDropped) {
			out.Outlook.Status = krvalue.StatusDropped
		}
		return out, nil
	}

	atRisk, err := e.AtRiskPace(loc)
	if err != nil {
		return KeyResult{}, err
	}
	out.Outlook = krvalue.Assess(krvalue.Assessment{
		Type:          typ,
		Baseline:      baseline,
		Target:        target.Decimal,
		Current:       out.Current.Decimal,
		HasCurrent:    out.HasCurrent,
		Created:       ent.Created,
		HasCreated:    ent.HasCreated,
		Deadline:      ent.Deadline,
		HasDeadline:   ent.HasDeadline,
		Now:           e.Now,
		AtRiskPace:    atRisk.Value,
		HasAtRiskPace: atRisk.Found,
		Dropped:       state.Status == string(krvalue.StatusDropped),
	})
	return out, nil
}

// parseReading parses a stored reading, reporting false for one that does not
// fit its key-result's type — again doctor's finding, not a read's failure.
func parseReading(typ krvalue.Type, raw string) (krvalue.Value, bool) {
	if raw == "" {
		return krvalue.Value{}, false
	}
	v, err := krvalue.Parse(typ, raw)
	if err != nil {
		return krvalue.Value{}, false
	}
	return v, true
}
