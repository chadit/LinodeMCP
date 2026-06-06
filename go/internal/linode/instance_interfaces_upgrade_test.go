package linode_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")
		checkEmpty(t, r.URL.RawQuery, "request should not include query parameters")
		checkEqual(t, "Bearer token", r.Header.Get("Authorization"), "authorization header should use bearer token")

		var got linode.UpgradeLinodeInterfacesRequest
		if !checkNoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode") {
			return
		}

		if got.ConfigID == nil {
			t.Error("config_id should be sent")

			return
		}

		checkEqual(t, configID, *got.ConfigID, "config_id should match")

		if got.DryRun == nil {
			t.Error("dry_run should be sent")

			return
		}

		if *got.DryRun {
			t.Error("dry_run should match")
		}

		w.Header().Set("Content-Type", "application/json")
		checkNoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	got, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{
		ConfigID: &configID,
		DryRun:   &dryRun,
	})

	requireNoError(t, err, "UpgradeLinodeInterfaces should succeed on 200 response")
	requireNotNil(t, got)
	checkEqual(t, configID, got.ConfigID)

	if got.DryRun {
		t.Error("dry_run should match")
	}

	requireLenOne(t, got.Interfaces)
	checkEqual(t, macAddressFixture, got.Interfaces[0].MACAddress)
}

func TestClientUpgradeLinodeInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		checkEqual(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")
		w.WriteHeader(http.StatusForbidden)
		checkNoError(t, json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}), "encoding error response should not fail")
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))
	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})

	requireError(t, err, "API error should be returned")
	apiErr := requireAPIError(t, err, "error should expose APIError")
	checkEqual(t, http.StatusForbidden, apiErr.StatusCode)
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

	requireErrorIs(t, err, linode.ErrLinodeIDPositive)

	if called.Load() {
		t.Error("invalid ID should not reach upstream server")
	}
}

func TestClientUpgradeLinodeInterfacesDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		checkEqual(t, http.MethodPost, r.Method, "request method should be POST")
		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))
	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})

	requireError(t, err, "server error should be returned")
	checkEqual(t, int32(1), calls.Load(), "POST upgrade call should not be replayed after transient server error")
}
