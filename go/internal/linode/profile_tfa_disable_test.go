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

func TestClientDisableProfileTFASuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		assert.Equal(t, int64(0), r.ContentLength, "POST request should not send a body")

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())

	require.NoError(t, err, "DisableProfileTFA should succeed on 200 response")
}

func TestClientDisableProfileTFAAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	err := client.DisableProfileTFA(t.Context())

	require.Error(t, err, "DisableProfileTFA should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "DisableProfileTFA should return APIError")
}

func TestClientDisableProfileTFANoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-disable", r.URL.Path, "request path should be /profile/tfa-disable")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	err := client.DisableProfileTFA(t.Context())

	require.Error(t, err, "DisableProfileTFA should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "security-state-changing POST must not be retried")
}
