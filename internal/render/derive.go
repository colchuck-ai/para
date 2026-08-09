package render

import (
	"strconv"
	"strings"

	"github.com/colchuck-ai/para/internal/journal"
	"github.com/colchuck-ai/para/internal/krvalue"
)

// reading is one measurement event with the two columns §4.4 derives from it —
// the decimal equivalent of the reading and the progress it represents.
//
// Both MEASUREMENTS.csv and ACTIVITY.md's measurement line need these numbers,
// so they are derived in exactly one place. Derivation is best-effort: a
// key-result whose state contradicts its measurements is doctor's `invalid`
// finding to report (§10), not a reason for rendering to fail, so a reading
// whose arithmetic does not work out carries HasDerived false and every
// renderer prints the raw value alone.
type reading struct {
	Event    journal.Event
	Value    krvalue.Value
	Decimal  float64
	Progress float64

	HasDerived bool
}

// readings derives the columns for every measurement event in in.Events, in
// chronological order.
//
// The baseline is §4.1's rule: an explicit `start` if the key-result has one,
// otherwise the first measurement ever logged. That is why a key-result's
// incremental render needs its whole measurement history and not just the day's
// (see In.Events) — the baseline is a function of the oldest reading, so
// omitting it would change every later row's progress.
func readings(in In) []reading {
	typ := krvalue.Type(in.State.Type)

	var events []journal.Event
	for _, e := range in.Events {
		if e.Kind == journal.KindMeasurement {
			events = append(events, e)
		}
	}
	if len(events) == 0 {
		return nil
	}

	parsed := make([]reading, 0, len(events))
	for _, e := range events {
		r := reading{Event: e}
		if v, err := krvalue.Parse(typ, e.Value); err == nil {
			r.Value = v
			r.Decimal = krvalue.NormalizeZero(v.Decimal)
		}
		parsed = append(parsed, r)
	}

	baseline, baselineOK := baselineDecimal(in, typ, parsed)
	target, targetOK := parseDecimal(typ, in.State.Target)

	for i := range parsed {
		if parsed[i].Value.Raw == "" || !baselineOK || !targetOK {
			continue
		}
		p, err := krvalue.Progress(baseline, target, parsed[i].Decimal, true)
		if err != nil {
			continue
		}
		parsed[i].Progress = krvalue.NormalizeZero(p)
		parsed[i].HasDerived = true
	}
	return parsed
}

// baselineDecimal resolves §4.1's start through the one rule krvalue owns. The
// oldest reading is found here rather than there because only this package
// holds the parsed rows: `parsed` is already in chronological order, so the
// first row carrying a value is the oldest one that could be a baseline.
func baselineDecimal(in In, typ krvalue.Type, parsed []reading) (float64, bool) {
	var oldest krvalue.Value
	var hasOldest bool
	for _, r := range parsed {
		if r.Value.Raw != "" {
			oldest, hasOldest = r.Value, true
			break
		}
	}
	return krvalue.Baseline(typ, in.State.Start, oldest, hasOldest)
}

func parseDecimal(typ krvalue.Type, raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	v, err := krvalue.Parse(typ, raw)
	if err != nil {
		return 0, false
	}
	return v.Decimal, true
}

// readingAt finds the derived reading for a given measurement event, matched on
// its exact stored instant — the same comparison §3.1 makes measurements unique
// by.
func readingAt(rs []reading, e journal.Event) (reading, bool) {
	for _, r := range rs {
		if r.Event.At.Equal(e.At) && r.Event.Value == e.Value {
			return r, true
		}
	}
	return reading{}, false
}

// percent renders a fraction as a percentage with the given number of decimal
// places: 0.08 at 1 place is "8.0%", 0.47 at 0 places is "47%". Progress is
// unclamped (§4.2), so a regression below baseline reads negative here and an
// overshoot reads above 100%.
func percent(f float64, places int) string {
	return strconv.FormatFloat(krvalue.NormalizeZero(f*100), 'f', places, 64) + "%"
}

// flatten collapses every run of whitespace in s — including the newlines a
// multi-line --note can carry — to a single space, and trims the ends. Notes
// reach ACTIVITY.md, README frontmatter, and rule files, all of which are
// line-oriented formats where a raw newline would break the structure rather
// than merely look untidy.
func flatten(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sentence terminates s with a period unless it already ends in terminal
// punctuation, so a note written as a sentence does not acquire a second one.
func sentence(s string) string {
	if s == "" {
		return ""
	}
	switch s[len(s)-1] {
	case '.', '!', '?', ':':
		return s
	default:
		return s + "."
	}
}
