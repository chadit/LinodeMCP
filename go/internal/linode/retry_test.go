package linode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func writeRetryTestResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	_, err := w.Write([]byte(body))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestRetryableClientSuccessNoRetry verifies that a successful first
// attempt returns immediately without any retries.
func TestRetryableClientSuccessNoRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Profile{Username: "user1"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(
		srv.URL, "token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	profile, err := readProfile(t.Context(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Username != "user1" {
		t.Errorf("profile.Username = %v, want %v", profile.Username, "user1")
	}

	if callCount.Load() != int32(1) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(1))
	}
}

// TestRetryableClientRetriesOnServerError verifies that the retry client
// retries on 500 errors and eventually succeeds when the server recovers.
func TestRetryableClientRetriesOnServerError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			writeRetryTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Profile{Username: "recovered"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(
		srv.URL, "token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	profile, err := readProfile(t.Context(), client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Username != tcRecovered {
		t.Errorf("profile.Username = %v, want %v", profile.Username, tcRecovered)
	}

	if callCount.Load() != int32(3) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(3))
	}
}

// TestRetryableClientNoRetryOnAuthError verifies that authentication errors
// (401) are not retried, since retrying with the same bad token is pointless.
func TestRetryableClientNoRetryOnAuthError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		writeRetryTestResponse(t, w, `{"errors":[{"reason":"invalid token"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(
		srv.URL, "bad-token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	_, err := readProfile(t.Context(), client)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if callCount.Load() != int32(1) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(1))
	}
}

// TestRetryableClientExhaustsRetries verifies that the retry client gives
// up after exhausting all configured retries and returns the last error.
func TestRetryableClientExhaustsRetries(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		writeRetryTestResponse(t, w, `{"errors":[{"reason":"always failing"}]}`)
	}))
	defer srv.Close()

	client := linode.NewClient(
		srv.URL, "token", nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	_, err := readProfile(t.Context(), client)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	// 1 initial + 2 retries = 3 total calls.
	if callCount.Load() != int32(3) {
		t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(3))
	}
}

// TestRetryableClientContextCancelStopsRetry verifies that canceling the
// context stops the retry loop before all retries are exhausted.
//
// Driven on synctest's virtual clock. The old shape raced a 10ms timer against
// the retry loop and could only assert "fewer than 6 attempts happened", which
// a cancel landing anywhere in a wide window would satisfy. Here each attempt
// is stepped deliberately: synctest.Wait parks the loop on its backoff, the
// clock advances exactly one backoff to release the next attempt, and the
// cancel lands at a known point. That turns a range check into an exact count.
//
// The server closes each connection so the transport does not park a read loop
// on a live socket inside the bubble; see TestRetryClampsRetryAfterToMaxDelay
// for the full reason.
func TestRetryableClientContextCancelStopsRetry(t *testing.T) {
	t.Parallel()

	// First backoff is BaseDelay * BackoffFactor^0.
	const firstBackoff = 50 * time.Millisecond

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Connection", "close")
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		writeRetryTestResponse(t, w, `{"errors":[{"reason":"failing"}]}`)
	}))
	defer srv.Close()

	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		client := linode.NewClient(
			srv.URL, "token", nil,
			linode.WithMaxRetries(5),
			linode.WithBaseDelay(firstBackoff),
			linode.WithMaxDelay(100*time.Millisecond),
			linode.WithBackoffFactor(2.0),
			linode.WithJitter(false),
		)

		result := make(chan error, 1)

		go func() {
			_, err := readProfile(ctx, client)
			result <- err
		}()

		// The first attempt has run and the loop is parked on its backoff.
		synctest.Wait()

		if got := callCount.Load(); got != int32(1) {
			t.Fatalf("callCount after the first attempt = %v, want %v", got, int32(1))
		}

		// Releasing exactly one backoff admits exactly one more attempt.
		synctest.Sleep(firstBackoff)

		if got := callCount.Load(); got != int32(2) {
			t.Fatalf("callCount after one backoff = %v, want %v", got, int32(2))
		}

		// Cancel while the loop sits on the second backoff. It has budget for
		// five retries, so without the cancellation check it would keep going.
		cancelledAt := time.Now()

		cancel()
		synctest.Wait()

		err := <-result
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		// The backoff select has to wake on ctx.Done rather than ride the timer
		// out, so no virtual time may pass between the cancel and the return.
		// Counting server hits alone would not prove this: once the context is
		// dead the per-request context also aborts each attempt, so the loop
		// could spin through its whole retry budget without the server ever
		// seeing another call. Elapsed virtual time is what separates "stopped"
		// from "kept waiting and failed fast".
		if waited := time.Since(cancelledAt); waited != 0 {
			t.Errorf("retry loop waited %v after cancel, want it to return without further backoff", waited)
		}

		// No attempt may follow the cancel. The old bound allowed anything
		// under six; this admits only the two attempts that were actually
		// released.
		if got := callCount.Load(); got != int32(2) {
			t.Errorf("callCount after cancel = %v, want %v", got, int32(2))
		}
	})
}

// TestRetryHonorsRetryAfterHint verifies that when the API returns 429 with
// a Retry-After hint, the retry loop waits that long instead of running its
// own exponential backoff. The hint is set well above BaseDelay so a retry
// that ran the default backoff lands nowhere near the expected wait.
//
// Measured on synctest's virtual clock for the reasons spelled out on
// TestRetryClampsRetryAfterToMaxDelay, including why the server closes each
// connection. Virtual time turns the old ">= 900ms, allowing for slop" bound
// into the exact hint, and costs no real second of suite runtime to observe it.
func TestRetryHonorsRetryAfterHint(t *testing.T) {
	t.Parallel()

	const retryAfterHint = time.Second

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Connection", "close")

		count := callCount.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			writeRetryTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Profile{Username: "ok"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	synctest.Test(t, func(t *testing.T) {
		client := linode.NewClient(
			srv.URL, "token", nil,
			linode.WithMaxRetries(2),
			linode.WithBaseDelay(1*time.Millisecond),
			linode.WithMaxDelay(5*time.Second),
			linode.WithBackoffFactor(2.0),
			linode.WithJitter(false),
		)

		start := time.Now()
		profile, err := readProfile(t.Context(), client)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if profile.Username != managedServiceStatus {
			t.Errorf("profile.Username = %v, want %v", profile.Username, managedServiceStatus)
		}

		if callCount.Load() != int32(2) {
			t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(2))
		}

		// The hint sits well under MaxDelay, so it is honored verbatim. The
		// exponential path with BaseDelay=1ms would land at 1ms instead, and
		// on virtual time that difference is exact rather than approximate.
		if elapsed != retryAfterHint {
			t.Errorf("elapsed = %v, want exactly %v", elapsed, retryAfterHint)
		}
	})
}

// TestRetryClampsRetryAfterToMaxDelay verifies that an absurdly large
// Retry-After hint is clamped to MaxDelay so a hostile or buggy server
// can't make us wait forever.
//
// The wait is measured on synctest's virtual clock rather than the wall clock.
// The retry loop waits on time.After, which the bubble virtualizes, and the two
// real HTTP round trips cost no virtual time at all. That turns the clamp into
// an exact equality instead of a "fast enough" ceiling that a loaded machine
// can miss for reasons that have nothing to do with the clamp.
//
// Two details make the bubble work. The server is started outside it, so the
// server's own goroutines stay out. And the server closes each connection,
// because the client transport otherwise parks a persistConn read loop on a
// live socket inside the bubble: a goroutine blocked on real network I/O is not
// durably blocked, so the virtual clock would never advance and the retry wait
// would hang instead of completing. Connection teardown leaves every bubble
// goroutine either running or durably blocked, which is what lets time move.
func TestRetryClampsRetryAfterToMaxDelay(t *testing.T) {
	t.Parallel()

	const maxDelay = 50 * time.Millisecond

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Connection", "close")

		count := callCount.Add(1)
		if count == 1 {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			writeRetryTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

			return
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Profile{Username: "ok"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	synctest.Test(t, func(t *testing.T) {
		client := linode.NewClient(
			srv.URL, "token", nil,
			linode.WithMaxRetries(1),
			linode.WithBaseDelay(1*time.Millisecond),
			linode.WithMaxDelay(maxDelay),
			linode.WithBackoffFactor(2.0),
			linode.WithJitter(false),
		)

		start := time.Now()
		_, err := readProfile(t.Context(), client)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if callCount.Load() != int32(2) {
			t.Errorf("callCount.Load() = %v, want %v", callCount.Load(), int32(2))
		}

		// The 3600s hint has to come back as exactly MaxDelay. On virtual time
		// that is an equality, not a ceiling: an unclamped wait, a clamp to some
		// other bound, or the exponential-backoff path running instead of the
		// hint path all produce a different number and all fail here.
		if elapsed != maxDelay {
			t.Errorf("elapsed = %v, want exactly %v", elapsed, maxDelay)
		}
	})
}
