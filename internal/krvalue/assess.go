package krvalue

import "time"

// Assessment is everything §4 needs to derive one key-result's numbers: the
// resolved decimals, the window, and the threshold.
//
// The decimals arrive already resolved rather than as the strings they were
// stored in, because the three callers hold their measurement history in three
// different shapes — a journal read, a rendered row, a pending event — and only
// they can say which reading is the oldest and which the newest. What they must
// not each decide is what follows from those numbers, which is Assess's job.
type Assessment struct {
	Type Type

	// Baseline is §4.1's resolved start: the explicit `start`, else the oldest
	// reading. Baseline returns it.
	Baseline float64
	// Target is the key-result's target as a decimal.
	Target float64

	// Current is the newest reading. HasCurrent false is a key-result with a
	// target and no measurement yet, whose progress is exactly 0 rather than
	// undefined (§4.2) — so it can go at-risk instead of sitting quiet.
	Current    float64
	HasCurrent bool

	// Created and Deadline bound §4.2's elapsed window, and both must be
	// present for it to exist. Deadline is the last instant the stored `due`
	// admits (ptime.Deadline), not the midnight that opens it, so nothing is
	// late while its own day is still running.
	//
	// HasCreated is not redundant with a non-zero Created, and leaving it out
	// is a real defect rather than a tidiness question: a `created` that will
	// not parse is doctor's `invalid` finding (§10) and arrives here as the
	// zero time, which would make the window run from the year 1 and give
	// every such key-result an elapsed fraction of almost exactly 1 — a
	// confident pace derived from a date nobody wrote.
	Created     time.Time
	HasCreated  bool
	Deadline    time.Time
	HasDeadline bool
	// Now is the instant being asked about — one per command (§3.6).
	Now time.Time

	// AtRiskPace is §7's `key-result.at-risk-pace`. Unset everywhere means the
	// check never fires, so a tree that never set one never reads at-risk.
	AtRiskPace    float64
	HasAtRiskPace bool

	// Dropped is the stored `dropped` status — §4.3's one settable key-result
	// status, and therefore an override rather than a reading.
	Dropped bool
}

// Outlook is §4's derived quantities for one key-result. Every field here is
// forbidden from truth (§2.5), which is why they are computed together.
type Outlook struct {
	Progress    float64
	HasProgress bool

	// Pace is undefined without a deadline, before any time has elapsed, and
	// for a boolean (§4.2). Undefined pace never reads at-risk.
	Pace    float64
	HasPace bool

	// Status is §4.3's derived status, or the stored `dropped`. It is empty
	// only when the arithmetic is unavailable — truth doctor reports as
	// invalid (§10) — because a confident status derived from numbers that do
	// not add up is worse than none.
	Status Status

	// PastDue reports that Now is beyond the deadline, which `missed` follows
	// from and §17's `--overdue` filter asks directly.
	PastDue bool
}

// Assess derives §4.2's quantities and §4.3's status from one key-result's
// resolved numbers.
//
// The order of operations is the design's, not an implementation detail: the
// stored `dropped` short-circuits everything (§4.3), progress is unclamped
// (§4.2), pace is undefined in three named cases and never reads at-risk when
// it is, and achieved beats missed beats at-risk. Deriving that order once is
// the point of this function — the write path prints a status back from a
// reading it has not yet appended (§18.6) and the read path prints one for
// every key-result it lists (§16.1), and the two disagreeing about the same
// key-result would be a defect no test of either alone would catch.
func Assess(a Assessment) Outlook {
	var out Outlook
	out.PastDue = a.HasDeadline && a.Now.After(a.Deadline)

	if a.Dropped {
		// Stored, not computed: it stands even where the arithmetic does not
		// work out, because nobody derived it in the first place.
		out.Status = StatusDropped
		return out
	}

	progress, err := Progress(a.Baseline, a.Target, a.Current, a.HasCurrent)
	if err != nil {
		return out
	}
	out.Progress, out.HasProgress = NormalizeZero(progress), true

	elapsed, elapsedOK := Elapsed(a.Created, a.Deadline, a.Now, a.HasCreated && a.HasDeadline)
	out.Pace, out.HasPace = Pace(out.Progress, elapsed, elapsedOK, a.Type)

	out.Status = DerivedStatus(out.Progress, out.HasPace && a.HasAtRiskPace, out.Pace, a.AtRiskPace, out.PastDue)
	return out
}

// Baseline is §4.1's start: the explicit `start` if the key-result has one,
// else the oldest reading ever logged.
//
// oldest is the caller's oldest measurement, which only the caller can find —
// ordering is by `at` and never by position (§3.1), and a backdated `--at` can
// put it anywhere in the file. hasOldest false means no reading has been logged.
//
// A boolean has a baseline from birth: it cannot set `start`, because false is
// its only baseline (§4.1).
func Baseline(typ Type, start string, oldest Value, hasOldest bool) (float64, bool) {
	if typ == TypeBoolean {
		return 0, true
	}
	if start != "" {
		v, err := Parse(typ, start)
		if err != nil {
			// A start nobody can read is doctor's `invalid` finding (§10).
			// Falling through to the oldest reading would silently substitute a
			// different baseline and quietly change every progress figure.
			return 0, false
		}
		return v.Decimal, true
	}
	if !hasOldest {
		return 0, false
	}
	return oldest.Decimal, true
}

// NormalizeZero collapses negative zero to positive zero. §4.2's progress
// formula produces it for real inputs: a key-result counting *down* — legal,
// since direction falls out of the arithmetic and only start == target is
// rejected — has a negative denominator, so its first reading at the baseline
// gives 0 / -150, which is -0.0. Formatting that prints "-0%" and "-0.0000",
// which reads as a defect rather than as no progress yet.
func NormalizeZero(f float64) float64 {
	if f == 0 {
		return 0
	}
	return f
}
