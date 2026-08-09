// Package clock abstracts time.Now so that no library code calls it
// directly (design §0.2). Production wires in System; tests inject Fixed.
package clock

import "time"

// Clock returns the current instant.
type Clock interface {
	Now() time.Time
}

// System is the production Clock, backed by the real wall clock.
type System struct{}

func (System) Now() time.Time {
	return time.Now()
}

// Fixed is a Clock that always returns the same instant. Tests inject one to
// make anything time-dependent (created defaults, attention, elapsed, pace,
// journal filenames) deterministic.
type Fixed struct {
	At time.Time
}

func (f Fixed) Now() time.Time {
	return f.At
}
