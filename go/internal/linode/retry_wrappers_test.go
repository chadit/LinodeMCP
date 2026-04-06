package linode_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

// fastRetryOpts returns Option values with minimal delays for testing.
func fastRetryOpts() []linode.Option {
	return []linode.Option{
		linode.WithMaxRetries(3),
		linode.WithBaseDelay(1 * time.Millisecond),
		linode.WithMaxDelay(10 * time.Millisecond),
		linode.WithBackoffFactor(2.0),
		linode.WithJitter(false),
	}
}

// TestRetryWrappersDelegationPatterns verifies that the generated retry
// wrappers correctly delegate to the underlying client for each return-type
// pattern (slice, pointer, error-only, create-request, update-request) and
// that retries work across all patterns.
//
// Workflow:
//  1. **Setup**: Create httptest servers that fail on the first request then succeed
//  2. **Execute**: Call each wrapper method through the retryable client
//  3. **Verify**: Confirm successful results after retry and correct request counts
//
// Expected Behavior:
//   - Slice-returning methods retry on 500 and return data on success
//   - Pointer-returning methods retry on 500 and return populated structs
//   - Error-only methods retry on 500 and return nil on success
//   - Create methods with request bodies retry and return created resources
//   - Update methods with ID + request body retry and return updated resources
//   - Persistent 500s exhaust all retries and return an error
//   - 401 errors are not retried
//   - Two-ID delete methods retry correctly
//
// Purpose: Ensures the retry wrapper generator produces correct delegation
// for all method signatures used by the Linode client.
func TestRetryWrappersDelegationPatterns(t *testing.T) {
	t.Parallel()

	t.Run("ListRegions returns slice", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			assert.Equal(t, "/regions", r.URL.Path, "request path should be /regions")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    []map[string]string{{"id": "us-east"}},
				"page":    1,
				"pages":   1,
				"results": 1,
			}), "encoding regions response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		regions, err := client.ListRegions(t.Context())
		require.NoError(t, err, "ListRegions should succeed after retry")
		require.Len(t, regions, 1, "should return one region")
		assert.Equal(t, "us-east", regions[0].ID, "region ID should match the API response")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("GetFirewall returns pointer", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			assert.Equal(t, "/networking/firewalls/1", r.URL.Path, "request path should include firewall ID")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"id":     1,
				"label":  "my-fw",
				"status": "enabled",
			}), "encoding firewall response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		firewall, err := client.GetFirewall(t.Context(), 1)
		require.NoError(t, err, "GetFirewall should succeed after retry")
		require.NotNil(t, firewall, "firewall should not be nil")
		assert.Equal(t, 1, firewall.ID, "firewall ID should match the request")
		assert.Equal(t, "my-fw", firewall.Label, "firewall label should match the API response")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("DeleteDomain returns error only", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			assert.Equal(t, "/domains/1", r.URL.Path, "request path should include domain ID")

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		err := client.DeleteDomain(t.Context(), 1)
		require.NoError(t, err, "DeleteDomain should succeed after retry")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("CreateFirewall request returns pointer", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"id":    1,
				"label": "new-fw",
			}), "encoding created firewall response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		firewall, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{
			Label: "new-fw",
		})
		require.NoError(t, err, "CreateFirewall should succeed after retry")
		require.NotNil(t, firewall, "created firewall should not be nil")
		assert.Equal(t, "new-fw", firewall.Label, "firewall label should match the create request")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("UpdateFirewall id and request returns pointer", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"id":    1,
				"label": "updated-fw",
			}), "encoding updated firewall response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		firewall, err := client.UpdateFirewall(t.Context(), 1, linode.UpdateFirewallRequest{
			Label: "updated-fw",
		})
		require.NoError(t, err, "UpdateFirewall should succeed after retry")
		require.NotNil(t, firewall, "updated firewall should not be nil")
		assert.Equal(t, "updated-fw", firewall.Label, "firewall label should match the update request")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("ListRegions exhausts retries on persistent 500", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"persistent failure"}]}`))
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.ListRegions(t.Context())
		require.Error(t, err, "ListRegions should fail after exhausting retries")
		require.ErrorContains(t, err, "persistent failure", "error should contain the server's reason")
		// fastRetryOpts sets MaxRetries=3: 1 initial attempt + 3 retries = 4 total requests.
		assert.Equal(t, int32(4), requestCount.Load(), "should exhaust all retries (1 initial + 3 retries)")
	})

	t.Run("GetFirewall no retry on 401", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"Invalid Token"}]}`))
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.GetFirewall(t.Context(), 1)
		require.Error(t, err, "GetFirewall should fail on 401")
		require.ErrorContains(t, err, "Invalid Token", "error should contain the auth failure reason")
		assert.Equal(t, int32(1), requestCount.Load(), "should not retry on 401 authentication error")
	})

	t.Run("DeleteDomainRecord two ids returns error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

				return
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		err := client.DeleteDomainRecord(t.Context(), 1, 2)
		require.NoError(t, err, "DeleteDomainRecord should succeed after retry")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})
}

// TestRetryWrappersContextCancellationStopsRetry verifies that canceling
// the context during a retry sequence stops further attempts and returns
// a context-canceled error.
func TestRetryWrappersContextCancellationStopsRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	ctx, cancel := context.WithCancel(t.Context())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		cancel()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	_, err := client.ListRegions(ctx)
	require.Error(t, err, "ListRegions should fail when context is canceled")
	assert.ErrorContains(t, err, "context canceled", "error should indicate context cancellation")
}

// TestRetryWrappersBodyForwardedOnRetry verifies that request bodies are
// correctly re-sent on retried POST requests, so the server receives the
// full payload even after an initial failure.
func TestRetryWrappersBodyForwardedOnRetry(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))

			return
		}

		body, _ := io.ReadAll(r.Body)
		capturedBody = body

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"id":    1,
			"label": "test-fw",
		}), "encoding firewall response should not fail")
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

	firewall, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{
		Label: "test-fw",
	})
	require.NoError(t, err, "CreateFirewall should succeed after retry")
	require.NotNil(t, firewall, "created firewall should not be nil")
	assert.Equal(t, "test-fw", firewall.Label, "firewall label should match the create request")
	assert.Contains(t, string(capturedBody), `"label"`, "retried request body should contain the label field")
	assert.Contains(t, string(capturedBody), `"test-fw"`, "retried request body should contain the label value")
}
