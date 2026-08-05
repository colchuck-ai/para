package journal

import (
	"time"

	"github.com/colchuck-ai/para/internal/paraerr"
	"github.com/colchuck-ai/para/internal/ptime"
)

// CheckMeasurementUnique refuses at if an existing measurement event
// already sits at the exact same instant (§3.1: "compared at the exact
// stored instant, not the day"). Notes and changes may collide freely —
// only measurement events are checked.
func CheckMeasurementUnique(events []Event, at time.Time) error {
	for _, e := range events {
		if e.Kind == KindMeasurement && ptime.Equal(e.At, at) {
			return paraerr.Newf(paraerr.KindConflict, "a measurement already exists at %s", at.Format(time.RFC3339))
		}
	}
	return nil
}
