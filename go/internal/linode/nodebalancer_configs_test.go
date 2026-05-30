package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/linode"
)

const nodeBalancerConfigsPath = "/nodebalancers/123/configs"

func TestClientListNodeBalancerConfigsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{
				keyID: 456, "port": 443, "protocol": "https", "algorithm": "roundrobin",
				"stickiness": "http_cookie", "check": "http", "check_interval": 10,
				"check_timeout": 5, "check_attempts": 3, "check_path": "/health",
				"check_body": "healthy", "check_passive": true, "cipher_suite": "recommended",
				"ssl_commonname": "example.com", "ssl_fingerprint": "fp", "nodebalancer_id": 123,
				"nodes_status": map[string]int{"up": 2, "down": 1},
			}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, 456, got[0].ID)
	assert.Equal(t, 443, got[0].Port)
	assert.Equal(t, "https", got[0].Protocol)
	assert.Equal(t, 123, got[0].NodeBalancerID)
	assert.Equal(t, 2, got[0].NodesStatus.Up)
	assert.Equal(t, 1, got[0].NodesStatus.Down)
}

func TestClientListNodeBalancerConfigsAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientListNodeBalancerConfigsRetriesTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")

		if calls.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)

			return
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(1))
	got, err := client.ListNodeBalancerConfigs(t.Context(), 123)

	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load(), "read route should retry once after transient failure")
	require.Len(t, got, 1)
	assert.Equal(t, 456, got[0].ID)
}

func TestClientCreateNodeBalancerConfigSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var body map[string]any
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		port, ok := body["port"].(float64)
		assert.True(t, ok, "request body port should be numeric")
		assert.Equal(t, 80, int(port), "request body should include port")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyID: 456, "port": 80, "protocol": "http", "algorithm": "roundrobin", "nodebalancer_id": 123,
		}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 456, got.ID)
	assert.Equal(t, 80, got.Port)
	assert.Equal(t, "http", got.Protocol)
	assert.Equal(t, 123, got.NodeBalancerID)
}

func TestClientCreateNodeBalancerConfigAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	require.Error(t, err)
	assert.Nil(t, got)

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
	assert.Equal(t, errForbidden, apiErr.Message)
}

func TestClientCreateNodeBalancerConfigDoesNotRetryTransientError(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, nodeBalancerConfigsPath, r.URL.Path, "request path should match")
		calls.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(2))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	require.Error(t, err)
	assert.Nil(t, got)
	assert.Equal(t, int32(1), calls.Load(), "POST create route must not be retried")
}

func TestClientCreateNodeBalancerConfigRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for invalid nodebalancer_id")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 0, &linode.CreateNodeBalancerConfigRequest{Port: 80})

	require.ErrorIs(t, err, linode.ErrNodeBalancerIDPositive)
	assert.Nil(t, got)
}

func TestClientCreateNodeBalancerConfigRejectsNilRequest(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("request should not be sent for nil create config request")
		w.WriteHeader(http.StatusTeapot)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.CreateNodeBalancerConfig(t.Context(), 123, nil)

	require.ErrorIs(t, err, linode.ErrCreateConfigRequestRequired)
	assert.Nil(t, got)
}
