package clock

import "context"

type contextKey struct{}

// WithContext threads clk through ctx so it reaches every subsystem a
// command touches without any of them calling time.Now directly (§0.2).
func WithContext(ctx context.Context, clk Clock) context.Context {
	return context.WithValue(ctx, contextKey{}, clk)
}

// FromContext returns the Clock threaded onto ctx by WithContext, or System{}
// if none was set.
func FromContext(ctx context.Context) Clock {
	if clk, ok := ctx.Value(contextKey{}).(Clock); ok {
		return clk
	}
	return System{}
}
