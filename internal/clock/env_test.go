//go:build !para_testhooks

package clock_test

import (
	"testing"

	"github.com/colchuck-ai/para/internal/clock"
)

// Without the para_testhooks build tag (i.e. every production build), FromEnv
// must ignore PARA_NOW and PARA_TZ entirely (§0.2): production must not read
// them.
func TestFromEnvIgnoresEnvWithoutTestHooksTag(t *testing.T) {
	t.Setenv("PARA_NOW", "2026-08-04T09:30:00Z")
	t.Setenv("PARA_TZ", "America/New_York")

	got, err := clock.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("FromEnv() = %v, want nil clock outside para_testhooks builds", got)
	}
}
