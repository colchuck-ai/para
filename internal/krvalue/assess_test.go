package krvalue_test

import (
	"math"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/krvalue"
)

func ts(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parsing %q: %v", s, err)
	}
	return at
}

// base is §16.1's worked key-result, halfway through its window: created on
// 2026-01-01, due at the end of 2026-12-31, read on 2026-07-02 (elapsed ≈ 0.5).
func base(t *testing.T) krvalue.Assessment {
	return krvalue.Assessment{
		Type:          krvalue.TypeRatio,
		Baseline:      0,
		Target:        100,
		Current:       50,
		HasCurrent:    true,
		Created:       ts(t, "2026-01-01T00:00:00Z"),
		Deadline:      ts(t, "2026-12-31T23:59:59Z"),
		HasDeadline:   true,
		Now:           ts(t, "2026-07-02T12:00:00Z"),
		AtRiskPace:    0.8,
		HasAtRiskPace: true,
	}
}

func TestAssessProgressPaceAndStatus(t *testing.T) {
	got := krvalue.Assess(base(t))

	if !got.HasProgress || got.Progress != 0.5 {
		t.Errorf("progress: got %v (has %v), want 0.5", got.Progress, got.HasProgress)
	}
	if !got.HasPace || math.Abs(got.Pace-1) > 0.01 {
		t.Errorf("pace: got %v (has %v), want ~1", got.Pace, got.HasPace)
	}
	if got.Status != krvalue.StatusOnTrack {
		t.Errorf("status: got %s, want on-track", got.Status)
	}
	if got.PastDue {
		t.Error("PastDue: a key-result read mid-window is not past due")
	}
}

func TestAssessStatusTable(t *testing.T) {
	cases := []struct {
		name string
		edit func(*krvalue.Assessment)
		want krvalue.Status
	}{
		{
			// Reaching the target beats everything, even past the deadline (§4.3).
			name: "achieved beats missed",
			edit: func(a *krvalue.Assessment) {
				a.Current = 100
				a.Now = ts(t, "2027-03-01T00:00:00Z")
			},
			want: krvalue.StatusAchieved,
		},
		{
			name: "missed beats at-risk",
			edit: func(a *krvalue.Assessment) {
				a.Current = 1
				a.Now = ts(t, "2027-03-01T00:00:00Z")
			},
			want: krvalue.StatusMissed,
		},
		{
			name: "pace below the threshold is at-risk",
			edit: func(a *krvalue.Assessment) { a.Current = 10 },
			want: krvalue.StatusAtRisk,
		},
		{
			// §7: unset everywhere means the check never fires.
			name: "no at-risk-pace threshold, so never at-risk",
			edit: func(a *krvalue.Assessment) {
				a.Current = 10
				a.HasAtRiskPace = false
			},
			want: krvalue.StatusOnTrack,
		},
		{
			// §4.2: undefined pace never reads at-risk.
			name: "no deadline, so no pace and never at-risk",
			edit: func(a *krvalue.Assessment) {
				a.Current = 10
				a.HasDeadline = false
			},
			want: krvalue.StatusOnTrack,
		},
		{
			// §4.3: dropped is the one settable key-result status, and it is an
			// override rather than a reading.
			name: "dropped wins over the arithmetic",
			edit: func(a *krvalue.Assessment) {
				a.Current = 100
				a.Dropped = true
			},
			want: krvalue.StatusDropped,
		},
		{
			// §4.2: a boolean's pace would read at-risk for its whole life and
			// then flip, which is noise.
			name: "boolean has no pace",
			edit: func(a *krvalue.Assessment) {
				a.Type = krvalue.TypeBoolean
				a.Target, a.Current = 1, 0
			},
			want: krvalue.StatusOnTrack,
		},
		{
			// A key-result with a target and no reading yet: progress is exactly
			// 0, so it can go at-risk rather than sitting quiet (§4.2).
			name: "no measurement yet still assesses",
			edit: func(a *krvalue.Assessment) {
				a.Current, a.HasCurrent = 0, false
			},
			want: krvalue.StatusAtRisk,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := base(t)
			tc.edit(&a)
			if got := krvalue.Assess(a).Status; got != tc.want {
				t.Errorf("status: got %s, want %s", got, tc.want)
			}
		})
	}
}

// TestAssessOnTheDeadlineDayIsNotMissed is why Deadline exists: a bare `due`
// date is the whole day, so a key-result is not late while its day is running.
func TestAssessOnTheDeadlineDayIsNotMissed(t *testing.T) {
	a := base(t)
	a.Deadline = ts(t, "2026-09-30T23:59:59Z")
	a.Current = 1

	a.Now = ts(t, "2026-09-30T17:00:00Z")
	if got := krvalue.Assess(a); got.PastDue {
		t.Errorf("17:00 on the due date reads past due (status %s)", got.Status)
	}
	a.Now = ts(t, "2026-10-01T00:00:00Z")
	if got := krvalue.Assess(a); !got.PastDue || got.Status != krvalue.StatusMissed {
		t.Errorf("the day after: PastDue=%v status=%s, want true/missed", got.PastDue, got.Status)
	}
}

// TestAssessWithoutATargetDerivesNothing covers the truth doctor reports as
// invalid: a target equal to the baseline makes §4.2's denominator zero, and
// deriving a confident status from it would be worse than deriving none.
func TestAssessWithoutATargetDerivesNothing(t *testing.T) {
	a := base(t)
	a.Target = a.Baseline

	got := krvalue.Assess(a)
	if got.HasProgress || got.HasPace || got.Status != "" {
		t.Errorf("got %+v, want nothing derived", got)
	}
}

// TestAssessDroppedNeedsNoArithmetic: a dropped key-result whose truth does not
// add up still reads dropped, because the status was stored, not computed.
func TestAssessDroppedNeedsNoArithmetic(t *testing.T) {
	a := base(t)
	a.Target, a.Dropped = a.Baseline, true
	if got := krvalue.Assess(a).Status; got != krvalue.StatusDropped {
		t.Errorf("status: got %q, want dropped", got)
	}
}

func TestAssessProgressIsUnclamped(t *testing.T) {
	a := base(t)
	a.Current = 150
	if got := krvalue.Assess(a).Progress; got != 1.5 {
		t.Errorf("overshoot: got %v, want 1.5", got)
	}

	a.Baseline, a.Current = 20, 10
	if got := krvalue.Assess(a).Progress; got >= 0 {
		t.Errorf("below baseline: got %v, want negative", got)
	}
}

func TestBaseline(t *testing.T) {
	oldest := krvalue.Value{Type: krvalue.TypeRatio, Raw: "480/9000", Decimal: 480.0 / 9000}

	cases := []struct {
		name       string
		typ        krvalue.Type
		start      string
		oldest     krvalue.Value
		hasOldest  bool
		want       float64
		wantHasVal bool
	}{
		{
			name: "an explicit start wins", typ: krvalue.TypeRatio, start: "0/9000",
			oldest: oldest, hasOldest: true, want: 0, wantHasVal: true,
		},
		{
			name: "no start falls back to the oldest reading (§4.1)", typ: krvalue.TypeRatio,
			oldest: oldest, hasOldest: true, want: 480.0 / 9000, wantHasVal: true,
		},
		{
			name: "no start and no reading has no baseline", typ: krvalue.TypeRatio,
		},
		{
			// A boolean cannot set start — false is its only baseline (§4.1) —
			// so it has one from birth, with no reading needed.
			name: "a boolean's baseline is always false", typ: krvalue.TypeBoolean,
			want: 0, wantHasVal: true,
		},
		{
			// A start that does not parse is doctor's invalid finding; deriving
			// from a value nobody can read would be inventing one.
			name: "an unparseable start has no baseline", typ: krvalue.TypeNumber, start: "lots",
			oldest: oldest, hasOldest: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := krvalue.Baseline(tc.typ, tc.start, tc.oldest, tc.hasOldest)
			if ok != tc.wantHasVal || (ok && got != tc.want) {
				t.Errorf("Baseline: got %v/%v, want %v/%v", got, ok, tc.want, tc.wantHasVal)
			}
		})
	}
}
