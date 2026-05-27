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

func writeRawTestResponse(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()

	_, err := w.Write([]byte(body))
	assert.NoError(t, err, "writing response should not fail")
}

// dropConn hijacks the request's connection and closes it without writing a
// response. The client observes a transport error after it has already sent
// the request, simulating the worst case for retries: the server may have
// processed the call before the failure surfaced.
func dropConn(w http.ResponseWriter) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}

	conn, _, err := hj.Hijack()
	if err != nil {
		return
	}

	_ = conn.Close()
}

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

func TestCreateImageRouteAndRetrySafety(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact POST images route and body", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/images", r.URL.Path, "request path should be /images")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.InEpsilon(t, 123, body["disk_id"], 0, "disk_id should be sent")
			assert.Equal(t, "custom-image", body["label"], "label should be sent")
			assert.Equal(t, "test image", body["description"], "description should be sent")
			assert.Equal(t, true, body["cloud_init"], "cloud_init should be sent")
			assert.Equal(t, []any{"blue", "green"}, body["tags"], "tags should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:        "private/15",
				keyLabel:     "custom-image",
				keyStatus:    "creating",
				"created_by": "tester",
			}), "encoding image response should not fail")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		image, err := client.CreateImage(t.Context(), &linode.CreateImageRequest{
			DiskID:      123,
			Label:       "custom-image",
			Description: "test image",
			CloudInit:   true,
			Tags:        []string{"blue", "green"},
		})

		require.NoError(t, err, "CreateImage should succeed")
		require.NotNil(t, image, "created image should not be nil")
		assert.Equal(t, "private/15", image.ID, "image ID should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "CreateImage should make one request")
	})

	t.Run("transient server error is not replayed", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		image, err := client.CreateImage(t.Context(), &linode.CreateImageRequest{DiskID: 123})

		require.Error(t, err, "CreateImage should return the first transient error")
		assert.Nil(t, image, "image should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent image creation must not be retried")
	})
}

func TestCreateImageShareGroupTokenRouteAndRetrySafety(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact POST image share group tokens route and body", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/images/sharegroups/tokens", r.URL.Path, "request path should be /images/sharegroups/tokens")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, "release-token", body["label"], "label should be sent")
			assert.Equal(t, shareGroupUUIDFixture, body["valid_for_sharegroup_uuid"], "share group UUID should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"token":                     "eyJhbGciOiJIUzI1NiJ9.test.signature",
				"token_uuid":                shareGroupTokenUUIDFixture,
				keyStatus:                   oauthClientStatus,
				keyLabel:                    "release-token",
				"created":                   imageShareGroupTokenCreated,
				"updated":                   nil,
				"expiry":                    nil,
				"valid_for_sharegroup_uuid": shareGroupUUIDFixture,
				"sharegroup_uuid":           shareGroupUUIDFixture,
				"sharegroup_label":          shareGroupLabelFixture,
			}), "encoding token response should not fail")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		token, err := client.CreateImageShareGroupToken(t.Context(), &linode.CreateImageShareGroupTokenRequest{
			Label:                  "release-token",
			ValidForShareGroupUUID: shareGroupUUIDFixture,
		})

		require.NoError(t, err, "CreateImageShareGroupToken should succeed")
		require.NotNil(t, token, "created token should not be nil")
		assert.Equal(t, shareGroupTokenUUIDFixture, token.TokenUUID, "token UUID should match response")
		assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.test.signature", token.Token, "token material should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "CreateImageShareGroupToken should make one request")
	})

	t.Run("transient server error is not replayed", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		token, err := client.CreateImageShareGroupToken(t.Context(), &linode.CreateImageShareGroupTokenRequest{ValidForShareGroupUUID: shareGroupUUIDFixture})

		require.Error(t, err, "CreateImageShareGroupToken should return the first transient error")
		assert.Nil(t, token, "token should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent token creation must not be retried")
	})
}

func TestImportDomainRouteAndRetrySafety(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact POST domains import route and body", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/domains/import", r.URL.Path, "request path should be /domains/import")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, domainExample, body["domain"], "domain should be sent")
			assert.Equal(t, remoteNameserverExample, body["remote_nameserver"], "remote_nameserver should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:     111,
				keyDomain: domainExample,
				keyType:   "master",
				keyStatus: oauthClientStatus,
			}), "encoding domain response should not fail")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		domain, err := client.ImportDomain(t.Context(), &linode.ImportDomainRequest{
			Domain:           domainExample,
			RemoteNameserver: remoteNameserverExample,
		})

		require.NoError(t, err, "ImportDomain should succeed")
		require.NotNil(t, domain, "imported domain should not be nil")
		assert.Equal(t, 111, domain.ID, "domain ID should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "ImportDomain should make one request")
	})

	t.Run("transient server error is not replayed", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		domain, err := client.ImportDomain(t.Context(), &linode.ImportDomainRequest{Domain: domainExample, RemoteNameserver: remoteNameserverExample})

		require.Error(t, err, "ImportDomain should return the first transient error")
		assert.Nil(t, domain, "domain should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent domain import must not be retried")
	})
}

func TestCloneDomainRouteAndRetrySafety(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact POST domains clone route and body", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/domains/111/clone", r.URL.Path, "request path should be /domains/111/clone")

			var body map[string]any
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode") {
				return
			}

			assert.Equal(t, domainExample, body["domain"], "domain should be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:     222,
				keyDomain: domainExample,
				keyType:   "master",
				keyStatus: oauthClientStatus,
			}), "encoding domain response should not fail")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		domain, err := client.CloneDomain(t.Context(), 111, &linode.CloneDomainRequest{Domain: domainExample})

		require.NoError(t, err, "CloneDomain should succeed")
		require.NotNil(t, domain, "cloned domain should not be nil")
		assert.Equal(t, 222, domain.ID, "domain ID should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "CloneDomain should make one request")
	})

	t.Run("transient server error is not replayed", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		t.Cleanup(srv.Close)

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)
		domain, err := client.CloneDomain(t.Context(), 111, &linode.CloneDomainRequest{Domain: domainExample})

		require.Error(t, err, "CloneDomain should return the first transient error")
		assert.Nil(t, domain, "domain should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "non-idempotent domain clone must not be retried")
	})
}

// Ensures the retry wrapper generator produces correct delegation for all method signatures used by the Linode client.
func TestRetryWrappersDelegationPatterns(t *testing.T) {
	t.Parallel()

	t.Run("ListRegions returns slice", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
				assert.NoError(t, err, "writing transient error response should succeed")

				return
			}

			assert.Equal(t, "/regions", r.URL.Path, "request path should be /regions")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    []map[string]string{{keyID: "us-east"}},
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
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
				_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
				assert.NoError(t, err, "writing transient error response should succeed")

				return
			}

			assert.Equal(t, "/networking/firewalls/1", r.URL.Path, "request path should include firewall ID")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:     1,
				keyLabel:  "my-fw",
				keyStatus: statusEnabledFixture,
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
				writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

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

	t.Run("CreateFirewall delegates and retries on 429", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				writeRawTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:    1,
				keyLabel: fwLabelNew,
			}), "encoding created firewall response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		firewall, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
		require.NoError(t, err, "CreateFirewall should succeed after a 429 retry")
		require.NotNil(t, firewall, "created firewall should not be nil")
		assert.Equal(t, fwLabelNew, firewall.Label, "firewall label should match the create request")
		assert.Equal(t, int32(2), requestCount.Load(),
			"a 429 is safe to replay because the request was rejected before processing")
	})

	t.Run("UpdateFirewall id and request returns pointer", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:    1,
				keyLabel: "updated-fw",
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
			writeRawTestResponse(t, w, `{"errors":[{"reason":"persistent failure"}]}`)
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
			writeRawTestResponse(t, w, `{"errors":[{"reason":"Invalid Token"}]}`)
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
				writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)

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

// TestRetryNonIdempotentDoesNotReplay verifies that a POST create is not
// retried on failures that may already have been applied server-side. A 5xx
// or a transport error (timeout, reset) on a create could mean the resource
// was made but the response was lost, so replaying it risks a duplicate.
func TestRetryNonIdempotentDoesNotReplay(t *testing.T) {
	t.Parallel()

	t.Run("no retry on 500", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusInternalServerError)
			_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
		require.Error(t, err, "CreateFirewall should fail on 500")
		assert.Equal(t, int32(1), requestCount.Load(),
			"a POST create must not retry a 5xx that may already have been applied")
	})

	t.Run("no retry on transport error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		// Hijack and drop the connection so the client sees a transport error
		// after the request was already sent: the server-side-applied case.
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)

			dropConn(w)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.CreateFirewall(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
		require.Error(t, err, "CreateFirewall should fail on a transport error")
		assert.Equal(t, int32(1), requestCount.Load(),
			"a POST create must not retry a transport error that may have been processed")
	})

	t.Run("idempotent GET still retries on transport error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)

			dropConn(w)
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.ListRegions(t.Context())
		require.Error(t, err, "ListRegions should fail when every attempt drops the connection")
		// fastRetryOpts sets MaxRetries=3: 1 initial + 3 retries = 4 attempts.
		assert.Equal(t, int32(4), requestCount.Load(),
			"a GET is idempotent, so transport errors are still retried")
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
		writeRawTestResponse(t, w, `{"errors":[{"reason":"server error"}]}`)
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
			// 429 is the one transient failure a non-idempotent POST still
			// retries, so it is what exercises body re-send on this path.
			w.WriteHeader(http.StatusTooManyRequests)
			writeRawTestResponse(t, w, `{"errors":[{"reason":"slow down"}]}`)

			return
		}

		body, _ := io.ReadAll(r.Body)
		capturedBody = body

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID:    1,
			keyLabel: "test-fw",
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

func TestGetSSHKeyRetries(t *testing.T) {
	t.Parallel()

	t.Run("happy path retries on transient error", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
				assert.NoError(t, err, "writing error response should succeed")

				return
			}

			assert.Equal(t, "/profile/sshkeys/42", r.URL.Path, "request path should include SSH key ID")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:     42,
				keyLabel:  "my-key",
				keySSHKey: "ssh-rsa AAAA test@example.com",
				"created": "2024-01-01T00:00:00Z",
			}), "encoding SSH key response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		sshKey, err := client.GetSSHKey(t.Context(), 42)
		require.NoError(t, err, "GetSSHKey should succeed after retry")
		require.NotNil(t, sshKey, "sshKey should not be nil")
		assert.Equal(t, 42, sshKey.ID, "SSH key ID should match")
		assert.Equal(t, "my-key", sshKey.Label, "SSH key label should match")
		assert.Equal(t, int32(2), requestCount.Load(), "should retry once then succeed")
	})

	t.Run("no retry on 401", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, err := w.Write([]byte(`{"errors":[{"reason":"Invalid Token"}]}`))
			assert.NoError(t, err, "writing error response should succeed")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		_, err := client.GetSSHKey(t.Context(), 42)
		require.Error(t, err, "GetSSHKey should fail on 401")
		require.ErrorContains(t, err, "Invalid Token", "error should contain the auth failure reason")
		assert.Equal(t, int32(1), requestCount.Load(), "should not retry on 401")
	})
}

const domainZoneTTL = "$TTL 864000"

func TestGetDomainZoneFileRoute(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact GET domain zone file route", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/domains/123/zone-file", r.URL.Path, "request path should include domain ID and zone-file suffix")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err, "request body should read")
			assert.Empty(t, body, "GET request should not include a body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"zone_file": []string{
					"; example.com [123]",
					domainZoneTTL,
				},
			}), "encoding domain zone file response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
		require.NoError(t, err, "GetDomainZoneFile should succeed")
		require.NotNil(t, zoneFile, "zoneFile should not be nil")
		assert.Equal(t, []string{"; example.com [123]", domainZoneTTL}, zoneFile.ZoneFile, "zone file lines should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "GetDomainZoneFile should make one request")
	})

	t.Run("transient server error retries read-only request", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
				assert.NoError(t, err, "writing transient error response should succeed")

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"zone_file": []string{domainZoneTTL},
			}), "encoding domain zone file response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
		require.NoError(t, err, "GetDomainZoneFile should succeed after retry")
		require.NotNil(t, zoneFile, "zoneFile should not be nil")
		assert.Equal(t, []string{"$TTL 864000"}, zoneFile.ZoneFile, "zone file lines should match response")
		assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
	})

	t.Run("permanent API error is not retried", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(`{"errors":[{"reason":"domain not found"}]}`))
			assert.NoError(t, err, "writing not found response should succeed")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		zoneFile, err := client.GetDomainZoneFile(t.Context(), 123)
		require.Error(t, err, "GetDomainZoneFile should return permanent API errors")
		assert.Nil(t, zoneFile, "zoneFile should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "permanent API errors should not be retried")
	})
}

func TestGetDomainRecordRoute(t *testing.T) {
	t.Parallel()

	t.Run("happy path uses exact GET domain record route", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/domains/123/records/456", r.URL.Path, "request path should include domain and record IDs")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err, "request body should read")
			assert.Empty(t, body, "GET request should not include a body")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID:     456,
				"type":    "A",
				"name":    "www",
				"target":  "192.0.2.10",
				"ttl_sec": 300,
			}), "encoding domain record response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		record, err := client.GetDomainRecord(t.Context(), 123, 456)
		require.NoError(t, err, "GetDomainRecord should succeed")
		require.NotNil(t, record, "record should not be nil")
		assert.Equal(t, 456, record.ID, "record ID should match response")
		assert.Equal(t, "A", record.Type, "record type should match response")
		assert.Equal(t, "www", record.Name, "record name should match response")
		assert.Equal(t, "192.0.2.10", record.Target, "record target should match response")
		assert.Equal(t, int32(1), requestCount.Load(), "GetDomainRecord should make one request")
	})

	t.Run("transient server error retries read-only request", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			count := requestCount.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusInternalServerError)
				_, err := w.Write([]byte(`{"errors":[{"reason":"server error"}]}`))
				assert.NoError(t, err, "writing transient error response should succeed")

				return
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyID: 456,
			}), "encoding domain record response should not fail")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		record, err := client.GetDomainRecord(t.Context(), 123, 456)
		require.NoError(t, err, "GetDomainRecord should succeed after retry")
		require.NotNil(t, record, "record should not be nil")
		assert.Equal(t, 456, record.ID, "record ID should match response")
		assert.Equal(t, int32(2), requestCount.Load(), "read-only GET should retry once then succeed")
	})

	t.Run("permanent API error is not retried", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			requestCount.Add(1)
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(`{"errors":[{"reason":"record not found"}]}`))
			assert.NoError(t, err, "writing not found response should succeed")
		}))
		defer srv.Close()

		client := linode.NewClient(srv.URL, "test-token", nil, fastRetryOpts()...)

		record, err := client.GetDomainRecord(t.Context(), 123, 456)
		require.Error(t, err, "GetDomainRecord should return permanent API errors")
		assert.Nil(t, record, "record should be nil on error")
		assert.Equal(t, int32(1), requestCount.Load(), "permanent API errors should not be retried")
	})
}
