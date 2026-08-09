package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/colchuck-ai/para/internal/clock"
)

func TestContextRoundTrip(t *testing.T) {
	want := clock.Fixed{At: time.Date(2026, time.August, 4, 0, 0, 0, 0, time.UTC)}
	ctx := clock.WithContext(context.Background(), want)

	got := clock.FromContext(ctx)
	if !got.Now().Equal(want.Now()) {
		t.Errorf("FromContext(ctx).Now() = %v, want %v", got.Now(), want.Now())
	}
}

func TestFromContextWithoutValueFallsBackToSystem(t *testing.T) {
	got := clock.FromContext(context.Background())
	if _, ok := got.(clock.System); !ok {
		t.Errorf("FromContext(background) = %T, want clock.System", got)
	}
}
