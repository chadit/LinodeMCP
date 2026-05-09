package linode_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// Tests cover the limiter as a primitive (NewRateLimiter + Wait) and as
// wired into Client through makeRequest. Time-dependent behavior runs under
// testing/synctest so refill progression is deterministic.

// rateLimitBurstTest is the per-minute rate used by the burst test. Sized
// small so synctest cycles fast.
const rateLimitBurstTest = 60

func TestRateLimiterDisabledWhenRateZero(t *testing.T) {
	t.Parallel()

	limiter := linode.NewRateLimiter(0)
	require.Nil(t, limiter, "non-positive rate yields nil (disabled)")
	require.NoError(t, limiter.Wait(t.Context()), "nil receiver allows immediately")
}

func TestRateLimiterAllowsBurstUpToCapacity(t *testing.T) {
	t.Parallel()

	limiter := linode.NewRateLimiter(rateLimitBurstTest)
	ctx := t.Context()

	for range rateLimitBurstTest {
		require.NoError(t, limiter.Wait(ctx), "burst within capacity should not block")
	}
}

func TestRateLimiterBlocksBeyondBurst(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// 60/min => 1 token per second refill, 60 capacity. Drain it then
		// time how long the next acquire takes; should be ~1s.
		limiter := linode.NewRateLimiter(60)
		ctx := t.Context()

		for range 60 {
			require.NoError(t, limiter.Wait(ctx))
		}

		start := time.Now()

		require.NoError(t, limiter.Wait(ctx), "after refill the next token should be granted")

		elapsed := time.Since(start)

		// Refill is 1 token/sec at 60/min; first post-burst token arrives
		// after ~1s of synthetic time.
		require.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "limiter should have blocked for ~1s of synthetic time")
	})
}

func TestRateLimiterCanceledByContext(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// 6/min: 0.1 token/sec refill. Drain then attempt one more with a
		// short-deadline context.
		limiter := linode.NewRateLimiter(6)
		for range 6 {
			require.NoError(t, limiter.Wait(t.Context()))
		}

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		err := limiter.Wait(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, linode.ErrRateLimitWaitCanceled, "ctx cancel surfaces as ErrRateLimitWaitCanceled")
		require.ErrorIs(t, err, context.DeadlineExceeded, "wraps the underlying ctx error")
	})
}

func TestRateLimiterRefillCapsAtCapacity(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		// 60/min capacity; idle 5 minutes; bucket should still cap at 60,
		// not 60*5.
		limiter := linode.NewRateLimiter(60)

		time.Sleep(5 * time.Minute)

		ctx := t.Context()
		for range 60 {
			require.NoError(t, limiter.Wait(ctx), "capped capacity should still allow 60 in burst")
		}

		// 61st call must block; ensure it does by using a tight deadline.
		short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		err := limiter.Wait(short)
		require.Error(t, err, "bucket must not over-fill past capacity")
	})
}

// TestClientHonorsRateLimit drives the limiter through the public NewClient
// + httptest path. Sets rate=1/min so the bucket empties after one call, then
// confirms the second call is blocked by the limiter (not by the upstream)
// by surfacing ErrRateLimitWaitCanceled when the caller's context expires.
//
// httptest.NewServer cannot run inside testing/synctest (its goroutines live
// outside the bubble), so this test uses real time with a deadline tight
// enough to keep the test fast.
func TestClientHonorsRateLimit(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"username":"u","email":"e"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:         0,
			RateLimitPerMinute: 1,
		},
	}
	client := linode.NewClient(srv.URL, "token", cfg, linode.WithJitter(false))

	// First call drains the single-token bucket.
	_, err := client.GetProfile(t.Context())
	require.NoError(t, err)
	assert.Equal(t, int32(1), calls.Load(), "first call should hit upstream")

	// Second call: bucket empty, refill is 1/60 token per second so the next
	// token is ~60s away. Tight ctx deadline ensures the limiter cancels
	// without waiting that long.
	//
	// GetProfile wraps makeRequest errors in a NetworkError (which is
	// retryable), so the surfaced error chain is shaped by the retry loop,
	// not the limiter directly. The signal that matters here is that the
	// upstream was never hit a second time, proving the limiter
	// short-circuited before any HTTP attempt.
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_, err = client.GetProfile(ctx)
	require.Error(t, err, "second call must fail (ctx expires before refill)")
	assert.Equal(t, int32(1), calls.Load(), "limiter must block the second call from reaching upstream")
}
