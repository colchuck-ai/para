//go:build para_testhooks

package clock

import (
	"fmt"
	"os"
	"time"
)

// FromEnv reads PARA_NOW (RFC 3339) and PARA_TZ, honored only under the
// para_testhooks build tag (§0.2). testscript sets these so script tests
// never depend on the host wall clock or the host's local zone.
//
// PARA_NOW's own offset determines the instant; PARA_TZ (default UTC)
// determines the zone the returned Clock reports it in, matching the
// local-wall-clock-with-recorded-offset model of design §15.1.
func FromEnv() (Clock, error) {
	raw := os.Getenv("PARA_NOW")
	if raw == "" {
		return nil, nil
	}

	loc := time.UTC
	if tz := os.Getenv("PARA_TZ"); tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return nil, fmt.Errorf("clock: invalid PARA_TZ %q: %w", tz, err)
		}
		loc = l
	}

	t, err := time.ParseInLocation(time.RFC3339, raw, loc)
	if err != nil {
		return nil, fmt.Errorf("clock: invalid PARA_NOW %q: %w", raw, err)
	}

	return Fixed{At: t.In(loc)}, nil
}
