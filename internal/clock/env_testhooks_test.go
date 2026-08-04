//go:build para_testhooks

package clock_test

import (
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
)

func TestFromEnvUnsetReturnsNil(t *testing.T) {
	t.Setenv("PARA_NOW", "")
	t.Setenv("PARA_TZ", "")

	got, err := clock.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("FromEnv() = %v, want nil clock when PARA_NOW is unset", got)
	}
}

func TestFromEnvParsesRFC3339(t *testing.T) {
	t.Setenv("PARA_NOW", "2026-08-04T09:30:00Z")
	t.Setenv("PARA_TZ", "")

	got, err := clock.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("FromEnv() = nil, want a Fixed clock")
	}

	want := time.Date(2026, time.August, 4, 9, 30, 0, 0, time.UTC)
	if now := got.Now(); !now.Equal(want) {
		t.Errorf("FromEnv().Now() = %v, want %v", now, want)
	}
}

func TestFromEnvHonorsParaTZForLocalDisplay(t *testing.T) {
	t.Setenv("PARA_NOW", "2026-08-04T09:30:00Z")
	t.Setenv("PARA_TZ", "America/New_York")

	got, err := clock.FromEnv()
	if err != nil {
		t.Fatalf("FromEnv() error = %v, want nil", err)
	}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("time.LoadLocation: %v", err)
	}
	want := time.Date(2026, time.August, 4, 9, 30, 0, 0, time.UTC).In(loc)

	now := got.Now()
	if !now.Equal(want) {
		t.Errorf("FromEnv().Now() = %v, want %v", now, want)
	}
	if now.Location().String() != loc.String() {
		t.Errorf("FromEnv().Now().Location() = %v, want %v", now.Location(), loc)
	}
}

func TestFromEnvRejectsMalformedNow(t *testing.T) {
	t.Setenv("PARA_NOW", "not-a-time")
	t.Setenv("PARA_TZ", "")

	if _, err := clock.FromEnv(); err == nil {
		t.Error("FromEnv() error = nil, want an error for malformed PARA_NOW")
	}
}

func TestFromEnvRejectsMalformedTZ(t *testing.T) {
	t.Setenv("PARA_NOW", "2026-08-04T09:30:00Z")
	t.Setenv("PARA_TZ", "Not/A_Zone")

	if _, err := clock.FromEnv(); err == nil {
		t.Error("FromEnv() error = nil, want an error for malformed PARA_TZ")
	}
}
