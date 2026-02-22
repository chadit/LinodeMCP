package linode_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

func TestDefaultRetryConfig(t *testing.T) {
	t.Parallel()

	cfg := linode.DefaultRetryConfig()
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, time.Second, cfg.BaseDelay)
	assert.Equal(t, 30*time.Second, cfg.MaxDelay)
	assert.InDelta(t, 2.0, cfg.BackoffFactor, 0.001)
	assert.True(t, cfg.JitterEnabled)
}

func TestNewRetryableClient(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClient("https://api.example.com", "tok", linode.DefaultRetryConfig())
	assert.NotNil(t, retryClient.Client)
	assert.Equal(t, 3, retryClient.RetryConfigField().MaxRetries)
}

func TestNewRetryableClientWithDefaults(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClientWithDefaults("https://api.example.com", "tok")
	assert.NotNil(t, retryClient.Client)
	assert.Equal(t, linode.DefaultRetryConfig().MaxRetries, retryClient.RetryConfigField().MaxRetries)
}

func TestRetryableClient_GetProfile_SuccessNoRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "user1"}))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    3,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	profile, err := retryClient.GetProfile(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "user1", profile.Username)
	assert.Equal(t, int32(1), callCount.Load(), "should only call once on success.")
}

func TestRetryableClient_RetriesOnServerError(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Profile{Username: "recovered"}))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    3,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	profile, err := retryClient.GetProfile(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "recovered", profile.Username)
	assert.Equal(t, int32(3), callCount.Load(), "should retry twice then succeed.")
}

func TestRetryableClient_NoRetryOnAuthError(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"invalid token"}]}`))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "bad-token", linode.RetryConfig{
		MaxRetries:    3,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	_, err := retryClient.GetProfile(t.Context())
	require.Error(t, err)
	assert.Equal(t, int32(1), callCount.Load(), "should not retry auth errors.")
}

func TestRetryableClient_ExhaustsRetries(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"always failing"}]}`))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    2,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	_, err := retryClient.GetProfile(t.Context())
	require.Error(t, err)
	// 1 initial + 2 retries = 3 total calls.
	assert.Equal(t, int32(3), callCount.Load(), "should exhaust all retries.")
}

func TestRetryableClient_ContextCancelStopsRetry(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"failing"}]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(t.Context())

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    5,
		BaseDelay:     50 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := retryClient.GetProfile(ctx)
	require.Error(t, err)
	// Should have been canceled before exhausting all retries.
	assert.Less(t, callCount.Load(), int32(6), "should stop before exhausting retries.")
}

func TestRetryableClient_ListInstances_Retries(t *testing.T) {
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
		}))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    2,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	instances, err := retryClient.ListInstances(t.Context())
	require.NoError(t, err)
	assert.Len(t, instances, 1)
}

func TestRetryableClient_GetInstance_Retries(t *testing.T) {
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
		assert.NoError(t, json.NewEncoder(w).Encode(linode.Instance{ID: 99, Label: "recovered"}))
	}))
	defer srv.Close()

	retryClient := linode.NewRetryableClient(srv.URL, "token", linode.RetryConfig{
		MaxRetries:    2,
		BaseDelay:     1 * time.Millisecond,
		MaxDelay:      10 * time.Millisecond,
		BackoffFactor: 2.0,
	})

	instance, err := retryClient.GetInstance(t.Context(), 99)
	require.NoError(t, err)
	assert.Equal(t, 99, instance.ID)
}

func TestShouldRetry_ForbiddenNotRetryable(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClientWithDefaults("http://x", "t")
	assert.False(t, retryClient.ExportedShouldRetry(&linode.APIError{StatusCode: 403, Message: "forbidden"}))
}

func TestShouldRetry_NetworkErrorRetryable(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClientWithDefaults("http://x", "t")
	assert.True(t, retryClient.ExportedShouldRetry(&linode.NetworkError{Operation: "test", Err: errors.New("conn")}))
}

func TestCalculateDelay_RespectMaxDelay(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClient("http://x", "t", linode.RetryConfig{
		MaxRetries:    5,
		BaseDelay:     10 * time.Second,
		MaxDelay:      15 * time.Second,
		BackoffFactor: 10.0,
		JitterEnabled: false,
	})

	delay := retryClient.ExportedCalculateDelay(3)
	assert.LessOrEqual(t, delay, 15*time.Second, "delay should not exceed MaxDelay.")
}

func TestCalculateDelay_JitterAddsVariance(t *testing.T) {
	t.Parallel()

	retryClient := linode.NewRetryableClient("http://x", "t", linode.RetryConfig{
		MaxRetries:    3,
		BaseDelay:     100 * time.Millisecond,
		MaxDelay:      10 * time.Second,
		BackoffFactor: 2.0,
		JitterEnabled: true,
	})

	delay := retryClient.ExportedCalculateDelay(1)
	// With jitter, delay should be >= base delay.
	assert.GreaterOrEqual(t, delay, 100*time.Millisecond)
}
