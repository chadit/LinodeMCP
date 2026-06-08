package linode_test

import (
	"encoding/json"
	"errors"
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
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/linode/instances/123/upgrade-interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/upgrade-interfaces")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		var got linode.UpgradeLinodeInterfacesRequest
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("request body should decode: %v", err)

			return
		}

		if got.ConfigID == nil {
			t.Error("config_id should be sent")

			return
		}

		if *got.ConfigID != configID {
			t.Errorf("*got.ConfigID = %v, want %v", *got.ConfigID, configID)
		}

		if got.DryRun == nil {
			t.Error("dry_run should be sent")

			return
		}

		if *got.DryRun {
			t.Error("dry_run should match")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{
		ConfigID: &configID,
		DryRun:   &dryRun,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ConfigID != configID {
		t.Errorf("got.ConfigID = %v, want %v", got.ConfigID, configID)
	}

	if got.DryRun {
		t.Error("dry_run should match")
	}

	if len(got.Interfaces) != 1 {
		t.Fatalf("len(got.Interfaces) = %d, want 1", len(got.Interfaces))
	}

	if got.Interfaces[0].MACAddress != macAddressFixture {
		t.Errorf("got.Interfaces[0].MACAddress = %v, want %v", got.Interfaces[0].MACAddress, macAddressFixture)
	}
}

func TestClientUpgradeLinodeInterfacesAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/linode/instances/123/upgrade-interfaces" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/linode/instances/123/upgrade-interfaces")
		}

		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
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

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if called.Load() {
		t.Error("invalid ID should not reach upstream server")
	}
}

func TestClientUpgradeLinodeInterfacesDoesNotReplayTransientPost(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		http.Error(w, `{"errors":[{"reason":"temporary failure"}]}`, http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(3))

	_, err := client.UpgradeLinodeInterfaces(t.Context(), 123, &linode.UpgradeLinodeInterfacesRequest{})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if calls.Load() != int32(1) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(1))
	}
}
