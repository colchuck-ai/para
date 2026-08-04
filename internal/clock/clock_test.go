package clock_test

import (
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
)

func TestSystemNowIsCloseToWallClock(t *testing.T) {
	before := time.Now()
	got := clock.System{}.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("System{}.Now() = %v, want between %v and %v", got, before, after)
	}
}

func TestFixedAlwaysReturnsTheSameInstant(t *testing.T) {
	want := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	f := clock.Fixed{At: want}

	if got := f.Now(); !got.Equal(want) {
		t.Errorf("Fixed.Now() = %v, want %v", got, want)
	}
	if got := f.Now(); !got.Equal(want) {
		t.Errorf("second call: Fixed.Now() = %v, want %v (must not change)", got, want)
	}
}

func TestClockInterfaceIsSatisfied(t *testing.T) {
	var _ clock.Clock = clock.System{}
	var _ clock.Clock = clock.Fixed{}
}
