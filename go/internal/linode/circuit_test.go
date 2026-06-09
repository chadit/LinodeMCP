package linode_test

import (
	"errors"
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
// cooldown progression is reproducible, not wall-clock.

func TestCircuitBreakerDisabledWhenThresholdZero(t *testing.T) {
	t.Parallel()

	breaker := linode.NewCircuitBreaker(0, time.Second)

	for range 100 {
		breaker.RecordFailure()
	}

	if err := breaker.Allow(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCircuitBreakerTripsAtThreshold(t *testing.T) {
	t.Parallel()

	breaker := linode.NewCircuitBreaker(3, time.Minute)

	breaker.RecordFailure()
	breaker.RecordFailure()

	if err := breaker.Allow(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	breaker.RecordFailure()

	err := breaker.Allow()
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
	}
}

func TestCircuitBreakerHalfOpenAfterTimeout(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 50*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()

		if err := breaker.Allow(); !errors.Is(err, linode.ErrCircuitOpen) {
			t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
		}

		time.Sleep(70 * time.Millisecond)

		if err := breaker.Allow(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := breaker.Allow(); !errors.Is(err, linode.ErrCircuitOpen) {
			t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
		}
	})
}

func TestCircuitBreakerClosesOnSuccessfulProbe(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 20*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()
		time.Sleep(30 * time.Millisecond)

		if err := breaker.Allow(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		breaker.RecordSuccess()

		if err := breaker.Allow(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := breaker.Allow(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestCircuitBreakerReopensOnFailedProbe(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		breaker := linode.NewCircuitBreaker(2, 20*time.Millisecond)

		breaker.RecordFailure()
		breaker.RecordFailure()
		time.Sleep(30 * time.Millisecond)

		if err := breaker.Allow(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		breaker.RecordFailure()

		if err := breaker.Allow(); !errors.Is(err, linode.ErrCircuitOpen) {
			t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
		}
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

	if err := breaker.Allow(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCircuitBreakerNilSafe(t *testing.T) {
	t.Parallel()

	var breaker *linode.CircuitBreaker

	if err := breaker.Allow(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	breaker.RecordSuccess()
	breaker.RecordFailure()
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
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(2) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(2))
	}

	// Second exhaustion: another 2 calls, breaker trips after.
	_, err = client.GetProfile(t.Context())
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(4) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(4))
	}

	// Third call: breaker open, must NOT touch upstream.
	_, err = client.GetProfile(t.Context())
	if !errors.Is(err, linode.ErrCircuitOpen) {
		t.Fatalf("error = %v, want %v", err, linode.ErrCircuitOpen)
	}

	if calls.Load() != int32(4) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(4))
	}
}
