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

func TestClientUpgradeLinodeInterfacesSuccess(t *testing.T) {
	t.Parallel()

	configID := 4567

	var dryRun bool

	response := linode.UpgradeLinodeInterfacesResponse{
		ConfigID: configID,
		DryRun:   dryRun,
		Interfaces: []linode.InstanceInterface{
			{ID: 1234, MACAddress: macAddressFixture},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")
		assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.UpgradeLinodeInterfacesRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

		if assert.NotNil(t, got.ConfigID, "config_id should be sent") {
			assert.Equal(t, configID, *got.ConfigID, "config_id should match")
		}

		if assert.NotNil(t, got.DryRun, "dry_run should be sent") {
			assert.False(t, *got.DryRun, "dry_run should match")
		}

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{
		ConfigID: &configID,
		DryRun:   &dryRun,
	})

	require.NoError(t, err, "UpgradeLinodeInterfaces should succeed on 200 response")
	require.NotNil(t, got)
	assert.Equal(t, configID, got.ConfigID)
	assert.False(t, got.DryRun)
	require.Len(t, got.Interfaces, 1)
	assert.Equal(t, macAddressFixture, got.Interfaces[0].MACAddress)
}

func TestClientUpgradeLinodeInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		assert.Equal(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})

	require.Error(t, err, "API error should be returned")

	var apiErr *linode.APIError
	assert.ErrorAs(t, err, &apiErr, "error should expose APIError")
}

func TestClientUpgradeLinodeInterfacesRejectsInvalidID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpgradeLinodeInterfaces(t.Context(), 0, &linode.UpgradeLinodeInterfacesRequest{})

	require.ErrorIs(t, err, linode.ErrLinodeIDPositive)
	assert.False(t, called.Load(), "invalid ID should not reach upstream server")
}

func TestClientUpgradeLinodeInterfacesDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))
	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})

	require.Error(t, err, "server error should be returned")
	assert.Equal(t, int32(1), calls.Load(), "POST upgrade call should not be replayed after transient server error")
}
