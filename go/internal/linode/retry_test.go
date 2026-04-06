package linode_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// TestRetryableClientGetProfileSuccessNoRetry verifies that a successful
// first attempt returns immediately without any retries.
func TestRetryableClientGetProfileSuccessNoRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "user1"}), "encoding profile response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	profile, err := client.GetProfile(t.Context())
	require.NoError(t, err, "GetProfile should succeed on first attempt")
	assert.Equal(t, "user1", profile.Username, "username should match the API response")
	assert.Equal(t, int32(1), callCount.Load(), "should only call the API once on success")
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
			_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "recovered"}), "encoding recovered profile should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	profile, err := client.GetProfile(t.Context())
	require.NoError(t, err, "GetProfile should succeed after retries")
	assert.Equal(t, "recovered", profile.Username, "username should match the recovered response")
	assert.Equal(t, int32(3), callCount.Load(), "should retry twice then succeed on third attempt")
}

// TestRetryableClientNoRetryOnAuthError verifies that authentication errors
// (401) are not retried, since retrying with the same bad token is pointless.
func TestRetryableClientNoRetryOnAuthError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"invalid token"}]}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "bad-token", nil,
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	_, err := client.GetProfile(t.Context())
	require.Error(t, err, "GetProfile should fail on auth error")
	assert.Equal(t, int32(1), callCount.Load(), "should not retry authentication errors")
}

// TestRetryableClientExhaustsRetries verifies that the retry client gives
// up after exhausting all configured retries and returns the last error.
func TestRetryableClientExhaustsRetries(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"always failing"}]}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	_, err := client.GetProfile(t.Context())
	require.Error(t, err, "GetProfile should fail after exhausting retries")
	// 1 initial + 2 retries = 3 total calls.
	assert.Equal(t, int32(3), callCount.Load(), "should exhaust all retries (1 initial + 2 retries)")
}

// TestRetryableClientContextCancelStopsRetry verifies that canceling the
// context stops the retry loop before all retries are exhausted.
func TestRetryableClientContextCancelStopsRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"failing"}]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(5),
		linode.WithBaseDelay(50*time.Millisecond),
		linode.WithMaxDelay(100*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	done := make(chan struct{})

	go func() {
		defer close(done)

		select {
		case <-time.After(10 * time.Millisecond):
			cancel()
		case <-ctx.Done():
		}
	}()

	_, err := client.GetProfile(ctx)
	require.Error(t, err, "GetProfile should fail when context is canceled")
	// Should have been canceled before exhausting all retries.
	assert.Less(t, callCount.Load(), int32(6), "should stop before exhausting all retries")
	<-done
}

// TestRetryableClientListInstancesRetries verifies that ListInstances
// retries on a 429 rate-limit response and succeeds on the second attempt.
func TestRetryableClientListInstancesRetries(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"rate limited"}]}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    []linode.Instance{{ID: 1, Label: "srv-1"}},
			"page":    1,
			"pages":   1,
			"results": 1,
		}), "encoding instances response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	instances, err := client.ListInstances(t.Context())
	require.NoError(t, err, "ListInstances should succeed after retry")
	assert.Len(t, instances, 1, "should return one instance after retry")
}

// TestRetryableClientGetInstanceRetries verifies that GetInstance retries
// on a 500 server error and succeeds on the second attempt.
func TestRetryableClientGetInstanceRetries(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := callCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"temporary"}]}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Instance{ID: 99, Label: "recovered"}), "encoding instance response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "token", nil,
		linode.WithMaxRetries(2),
		linode.WithBaseDelay(1*time.Millisecond),
		linode.WithMaxDelay(10*time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	)

	instance, err := client.GetInstance(t.Context(), 99)
	require.NoError(t, err, "GetInstance should succeed after retry")
	assert.Equal(t, 99, instance.ID, "instance ID should match the request")
}
