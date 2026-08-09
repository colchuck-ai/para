// Package krvalue implements key-result arithmetic (design §4): the three
// value grammars, unclamped progress, pace and every case where it is
// undefined, and derived status. Every function is pure — no clock, no
// filesystem — and takes the instants or decimals it needs as arguments;
// callers own retrieving "today" from a Clock and "current" from the
// journal's latest measurement.
package krvalue

import (
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/colchuck-ai/para/internal/paraerr"
)

// Type is a key-result's measurement grammar (§4.1). Required at creation
// and never settable afterward — changing it would invalidate every
// measurement already logged.
type Type string

const (
	TypeNumber  Type = "number"
	TypeRatio   Type = "ratio"
	TypeBoolean Type = "boolean"
)

// types is §4.1's three grammars, in the order every message and every
// completion lists them. It is the one place the set is written down: the
// `--type` flag's help, the validation that refuses anything else, and the
// shell completion all read it, so adding a fourth grammar is one edit rather
// than three that can disagree.
var types = []Type{TypeNumber, TypeRatio, TypeBoolean}

// TypeNames returns the three grammars as strings, in §4.1's order — for the
// messages and the completions that print them.
func TypeNames() []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = string(t)
	}
	return out
}

// IsType reports whether raw names one of the three grammars.
func IsType(raw string) bool { return slices.Contains(types, Type(raw)) }

// Value is a start, target, or measurement reading in its type's grammar,
// preserved exactly as entered. A ratio's denominator belongs to the
// reading, not the key-result — it legitimately varies between
// measurements — so it is kept as the string it was written in, never
// reduced to a decimal (§4.1).
type Value struct {
	Type    Type
	Raw     string
	Decimal float64
}

var (
	// numberPattern is §4.1's number grammar: a plain decimal, e.g. "42",
	// "0.024", "480". strconv.ParseFloat alone is too permissive — it also
	// accepts "NaN", "Inf", scientific notation, hex floats, and digit-
	// separator underscores, none of which the design's grammar shows, and
	// a NaN or Inf measurement would silently break every derived
	// comparison in §4.2/§4.3 rather than being rejected at the boundary.
	numberPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)
	ratioPattern  = regexp.MustCompile(`^(\d+)/(\d+)$`)
)

// Parse validates raw against typ's grammar (§4.1) and returns the Value
// carrying both the verbatim reading and its decimal equivalent for
// arithmetic (§4.2).
func Parse(typ Type, raw string) (Value, error) {
	switch typ {
	case TypeNumber:
		if !numberPattern.MatchString(raw) {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid number", raw)
		}
		d, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid number", raw)
		}
		return Value{Type: typ, Raw: raw, Decimal: d}, nil

	case TypeRatio:
		m := ratioPattern.FindStringSubmatch(raw)
		if m == nil {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid ratio (want <numerator>/<denominator>)", raw)
		}
		num, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid ratio", raw)
		}
		den, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid ratio", raw)
		}
		if den == 0 {
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q: a ratio's denominator cannot be zero", raw)
		}
		return Value{Type: typ, Raw: raw, Decimal: num / den}, nil

	case TypeBoolean:
		switch raw {
		case "true":
			return Value{Type: typ, Raw: raw, Decimal: 1}, nil
		case "false":
			return Value{Type: typ, Raw: raw, Decimal: 0}, nil
		default:
			return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a valid boolean (want true or false)", raw)
		}

	default:
		return Value{}, paraerr.Newf(paraerr.KindValidation, "%q is not a key-result type", typ)
	}
}

// ValidateBounds enforces §4.1's bounds on a key-result's start and target:
//   - a boolean key-result rejects an explicit start — false is its only
//     baseline — and requires target = true, since a target of false would
//     be achieved at birth;
//   - for number and ratio, an explicit start equal to target is rejected,
//     since it makes §4.2's denominator zero. (target's own requiredness,
//     and the case where start is left unset to default to the first
//     logged measurement, are validated elsewhere — this only judges the
//     values it is given.)
func ValidateBounds(typ Type, start *Value, target Value) error {
	if typ == TypeBoolean {
		if start != nil {
			return paraerr.New(paraerr.KindValidation, "a boolean key-result cannot set start — false is its only baseline")
		}
		if target.Raw != "true" {
			return paraerr.New(paraerr.KindValidation, "a boolean key-result's target must be true")
		}
		return nil
	}
	if start != nil && start.Decimal == target.Decimal {
		return paraerr.New(paraerr.KindValidation, "target must differ from start")
	}
	return nil
}

// Progress computes (current − start) / (target − start) (§4.2): not
// clamped, so overshoot reads above 1 and a regression below baseline reads
// negative — both are true and both are worth seeing. hasMeasurement false
// means no measurement has ever been logged, and progress is defined to be
// exactly 0 in that case, not derived from start, so an untouched
// key-result can go at-risk instead of sitting quiet.
func Progress(start, target, current float64, hasMeasurement bool) (float64, error) {
	if !hasMeasurement {
		return 0, nil
	}
	if target == start {
		return 0, paraerr.New(paraerr.KindValidation, "key-result target equals start")
	}
	return (current - start) / (target - start), nil
}

// Elapsed computes (today − created) / (due − created) (§4.2). ok is false
// when there is no due date, or when due does not postdate created — a
// zero or negative window has no meaningful fraction, matching the "no due"
// case rather than dividing by zero.
func Elapsed(created, due, today time.Time, hasDue bool) (elapsed float64, ok bool) {
	if !hasDue {
		return 0, false
	}
	total := due.Sub(created)
	if total <= 0 {
		return 0, false
	}
	return today.Sub(created).Seconds() / total.Seconds(), true
}

// Pace computes progress / elapsed (§4.2). ok is false — pace undefined —
// when elapsed itself is undefined, when elapsed ≤ 0, or when typ is
// boolean: a boolean's progress is 0 until done and 1 after, so pace would
// read at-risk for its entire life and then flip to achieved — noise, not
// signal.
func Pace(progress float64, elapsed float64, elapsedOK bool, typ Type) (pace float64, ok bool) {
	if typ == TypeBoolean || !elapsedOK || elapsed <= 0 {
		return 0, false
	}
	return progress / elapsed, true
}

// Status is a key-result's derived status (§1.7, §4.3): never set by hand,
// except Dropped, the one settable key-result status.
type Status string

const (
	StatusOnTrack  Status = "on-track"
	StatusAtRisk   Status = "at-risk"
	StatusMissed   Status = "missed"
	StatusAchieved Status = "achieved"
	StatusDropped  Status = "dropped"
)

// DerivedStatus implements §4.3's table, in the table's own precedence:
// achieved beats missed (reaching the target matters even past due),
// missed beats at-risk (a blown deadline is not the same reading as a slow
// pace), and undefined pace never reads at-risk. It does not latch: called
// again after a regression, it reads on-track (or at-risk) again, because
// it is a pure function of the current progress and pace, not of history.
//
// `dropped` is deliberately not an input: §4.3 states plainly that it "is
// the only settable key-result status" (§1.7's field matrix agrees:
// "derived (`dropped` settable)"), so it is a stored override the caller
// checks before ever computing a derived status, not a fifth row in this
// table.
func DerivedStatus(progress float64, paceDefined bool, pace, atRiskPace float64, pastDue bool) Status {
	switch {
	case progress >= 1:
		return StatusAchieved
	case pastDue:
		return StatusMissed
	case paceDefined && pace < atRiskPace:
		return StatusAtRisk
	default:
		return StatusOnTrack
	}
}
