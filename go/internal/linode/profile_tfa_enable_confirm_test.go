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

const (
	keyTFAConfirmExpiry = "expiry"
	tfaConfirmExpiry    = "2026-01-01T00:00:00"
)

func TestClientConfirmProfileTFAEnableSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		assert.Empty(t, r.URL.RawQuery, "request query should be empty")
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		var body map[string]string
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, map[string]string{"tfa_code": "123456"}, body)

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"scratch": "setup-token", keyTFAConfirmExpiry: tfaConfirmExpiry}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	require.NoError(t, err, "ConfirmProfileTFAEnable should succeed on 200 response")
	assert.Equal(t, "setup-token", result["scratch"])
	assert.Equal(t, tfaConfirmExpiry, result[keyTFAConfirmExpiry])
}

func TestClientConfirmProfileTFAEnableAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	require.Error(t, err, "ConfirmProfileTFAEnable should fail on 403 response")

	var apiErr *linode.APIError
	require.ErrorAs(t, err, &apiErr, "ConfirmProfileTFAEnable should return APIError")
}

func TestClientConfirmProfileTFAEnableNoRetryOnTransientFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/profile/tfa-enable-confirm", r.URL.Path, "request path should be /profile/tfa-enable-confirm")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: serverErrorReason}}}))
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(3))

	_, err := client.ConfirmProfileTFAEnable(t.Context(), &linode.ProfileTFAEnableConfirmRequest{TFACode: "123456"})

	require.Error(t, err, "ConfirmProfileTFAEnable should return the transient error")
	assert.Equal(t, int32(1), calls.Load(), "security-state-changing POST must not be retried")
}
