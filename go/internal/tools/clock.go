package tools

import (
	"context"
	"time"
)

// clockCtxKey namespaces the reference clock on a context. Its own unexported
// type is what keeps the value from colliding with another package's.
type clockCtxKey struct{}

// WithClock attaches the reference clock the audit report resolves a relative
// since_offset against. The cross-language behavior fixtures pin a report whose
// window is relative, and a wall-clock window has no reproducible answer.
func WithClock(ctx context.Context, clock func() time.Time) context.Context {
	if clock == nil {
		return ctx
	}

	return context.WithValue(ctx, clockCtxKey{}, clock)
}

// ClockFromContext answers the attached reference time, or the wall clock when
// the caller attached none.
//
// Exported because the generated handler reads it: LOCAL_AMBIENT_CLOCK renders
// in this language as the expression the call site hands its operation.
func ClockFromContext(ctx context.Context) time.Time {
	clock, ok := ctx.Value(clockCtxKey{}).(func() time.Time)
	if !ok {
		return time.Now()
	}

	return clock()
}
