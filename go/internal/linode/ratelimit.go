package linode

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// secondsPerMinute is the divisor that turns a per-minute rate into the
// per-second refill the bucket tracks internally.
const secondsPerMinute = 60.0

// RateLimiter is a token bucket that throttles outbound calls to a steady
// rate. Capacity equals the per-minute budget so a fully-replenished bucket
// permits one minute's worth of burst, then settles to the steady refill
// rate. Constructed with rate <= 0, NewRateLimiter returns nil and Wait
// becomes a no-op (limiter disabled).
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
}

// NewRateLimiter returns a token bucket limiter at perMinute rate. A
// non-positive rate yields a disabled limiter (nil) so callers don't have to
// special-case it.
func NewRateLimiter(perMinute int) *RateLimiter {
	if perMinute <= 0 {
		return nil
	}

	capacity := float64(perMinute)

	return &RateLimiter{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: capacity / secondsPerMinute,
		lastRefill: time.Now(),
	}
}

// Wait blocks until one token is available or ctx is canceled. Each call
// consumes exactly one token. A nil receiver allows immediately so disabled
// limiters cost nothing.
func (r *RateLimiter) Wait(ctx context.Context) error {
	if r == nil {
		return nil
	}

	for {
		wait, ok := r.tryAcquire()
		if ok {
			return nil
		}

		if err := sleepOrCancel(ctx, wait); err != nil {
			return err
		}
	}
}

// sleepOrCancel sleeps for d or returns when ctx is canceled. Defers the
// timer Stop so cairnlint's NewTimer leak check is satisfied per iteration.
func sleepOrCancel(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %w", ErrRateLimitWaitCanceled, ctx.Err())
	case <-timer.C:
		return nil
	}
}

// tryAcquire either consumes a token (returning ok=true) or returns the
// duration until at least one full token is available. Splitting this out
// keeps the lock scope tight and makes Wait's select-loop straightforward.
func (r *RateLimiter) tryAcquire() (time.Duration, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.refillLocked()

	if r.tokens >= 1 {
		r.tokens--

		return 0, true
	}

	needed := 1 - r.tokens
	wait := time.Duration(needed / r.refillRate * float64(time.Second))

	return wait, false
}

// refillLocked tops up the bucket based on time elapsed since the last
// refill. Caller must hold r.mu.
func (r *RateLimiter) refillLocked() {
	now := time.Now()

	elapsed := now.Sub(r.lastRefill).Seconds()
	if elapsed > 0 {
		r.tokens = math.Min(r.capacity, r.tokens+elapsed*r.refillRate)
		r.lastRefill = now
	}
}
