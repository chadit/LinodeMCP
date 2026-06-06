package linode_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// Tests drive the breaker through linode.NewCircuitBreaker and the public
// Client API. Time-dependent transitions run under testing/synctest so
// cooldown progression is deterministic, not wall-clock.

func TestCircuitBreakerDisabledWhenThresholdZero(t *testing.T) {
	t.Parallel()

	breaker := linode.NewCircuitBreaker(0, time.Second)

	for range 100 {
		breaker.RecordFailure()
	}

	mustNoError(t, breaker.Allow(), "threshold 0 disables the breaker entirely")
}

func TestCircuitBreakerTripsAtThreshold(t *testing.T) {
	t.Parallel()

	breaker := linode.NewCircuitBreaker(3, time.Minute)

	breaker.RecordFailure()
	breaker.RecordFailure()
	mustNoError(t, breaker.Allow(), "two failures (below threshold) should not trip")

	breaker.RecordFailure()
	err := breaker.Allow()
	mustError(t, err, "third failure should trip the breaker")
	mustErrorIs(t, err, linode.ErrCircuitOpen, "trip rejects with ErrCircuitOpen")
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 50*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()
		mustErrorIs(t, breaker.Allow(), linode.ErrCircuitOpen, "should be open after threshold")

		time.Sleep(70 * time.Millisecond)

		mustNoError(t, breaker.Allow(), "after cooldown one probe is admitted (half-open)")
		mustErrorIs(t, breaker.Allow(), linode.ErrCircuitOpen, "concurrent probe attempts rejected")
	})
}

func TestCircuitBreakerClosesOnSuccessfulProbe(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 20*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()
		time.Sleep(30 * time.Millisecond)
		mustNoError(t, breaker.Allow(), "half-open probe admitted")

		breaker.RecordSuccess()

		mustNoError(t, breaker.Allow(), "successful probe closes the breaker")
		mustNoError(t, breaker.Allow(), "closed state lets every request through")
	})
}

func TestCircuitBreakerReopensOnFailedProbe(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 20*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()
		time.Sleep(30 * time.Millisecond)
		mustNoError(t, breaker.Allow(), "half-open probe admitted")

		breaker.RecordFailure()

		mustErrorIs(t, breaker.Allow(), linode.ErrCircuitOpen, "failed probe must re-open")
	})
}

func TestCircuitBreakerSuccessResetsFailures(t *testing.T) {
	t.Parallel()

	breaker := linode.NewCircuitBreaker(3, time.Minute)

	breaker.RecordFailure()
	breaker.RecordFailure()
	breaker.RecordSuccess()

	breaker.RecordFailure()
	breaker.RecordFailure()
	mustNoError(t, breaker.Allow(), "success resets failure count")
}

func TestCircuitBreakerNilSafe(t *testing.T) {
	t.Parallel()

	var breaker *linode.CircuitBreaker

	mustNoError(t, breaker.Allow(), "nil breaker allows")
	mustNotPanics(t, breaker.RecordSuccess, "nil breaker recordSuccess is a no-op")
	mustNotPanics(t, breaker.RecordFailure, "nil breaker recordFailure is a no-op")
}

// TestExecuteWithRetryTripsCircuitOnExhaustion drives the integration through
// the public NewClient/httptest path. After threshold exhaustions, subsequent
// calls must short-circuit with ErrCircuitOpen and not touch the upstream.
func TestExecuteWithRetryTripsCircuitOnExhaustion(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"down"}]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Resilience: config.ResilienceConfig{
			MaxRetries:              1,
			BaseRetryDelay:          time.Millisecond,
			MaxRetryDelay:           time.Millisecond,
			CircuitBreakerThreshold: 2,
			CircuitBreakerTimeout:   time.Hour,
		},
	}
	client := linode.NewClient(srv.URL, "token", cfg, linode.WithJitter(false))

	// First exhaustion: 1 initial + 1 retry = 2 upstream calls.
	_, err := client.GetProfile(t.Context())
	mustError(t, err, "first attempt should fail after retry exhaustion")
	checkEqual(t, int32(2), calls.Load(), "first exhaustion runs the full retry budget")

	// Second exhaustion: another 2 calls, breaker trips after.
	_, err = client.GetProfile(t.Context())
	mustError(t, err, "second attempt should also fail")
	checkEqual(t, int32(4), calls.Load(), "second exhaustion runs the budget again")

	// Third call: breaker open, must NOT touch upstream.
	_, err = client.GetProfile(t.Context())
	mustError(t, err)
	mustErrorIs(t, err, linode.ErrCircuitOpen, "open breaker rejects without calling upstream")
	checkEqual(t, int32(4), calls.Load(), "open breaker must not invoke upstream")
}
