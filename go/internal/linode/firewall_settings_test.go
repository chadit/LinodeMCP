package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	endpointFirewallSettings  = "/networking/firewalls/settings"
	firewallSettingsKeyLinode = "linode"
)

func TestClientListFirewallSettingsSuccess(t *testing.T) {
	t.Parallel()

	settings := linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{
		Linode: 100, NodeBalancer: 101, PublicInterface: 200, VPCInterface: 201,
	}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallSettings)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		if len(r.URL.Query()["unexpected"]) != 0 {
			t.Errorf("value = %v, want empty", r.URL.Query()["unexpected"])
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(settings); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallSettings(t.Context(), 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.DefaultFirewallIDs.Linode != 100 {
		t.Errorf("result.DefaultFirewallIDs.Linode = %v, want %v", result.DefaultFirewallIDs.Linode, 100)
	}

	if result.DefaultFirewallIDs.NodeBalancer != 101 {
		t.Errorf("result.DefaultFirewallIDs.NodeBalancer = %v, want %v", result.DefaultFirewallIDs.NodeBalancer, 101)
	}

	if result.DefaultFirewallIDs.PublicInterface != 200 {
		t.Errorf("result.DefaultFirewallIDs.PublicInterface = %v, want %v", result.DefaultFirewallIDs.PublicInterface, 200)
	}

	if result.DefaultFirewallIDs.VPCInterface != 201 {
		t.Errorf("result.DefaultFirewallIDs.VPCInterface = %v, want %v", result.DefaultFirewallIDs.VPCInterface, 201)
	}
}

func TestClientListFirewallSettingsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallSettings)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallSettings(t.Context(), 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error = %v, want %v", err, &apiErr)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}
}

func TestClientListFirewallSettingsRetriesTransientFailure(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := requestCount.Add(1)
		if count == 1 {
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Error("response writer should support hijacking")

				return
			}

			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Errorf("unexpected error: %v", err)

				return
			}

			if err := conn.Close(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			return
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallSettings {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallSettings)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.FirewallSettings{DefaultFirewallIDs: linode.FirewallDefaultIDs{Linode: 100}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallSettings(t.Context(), 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.DefaultFirewallIDs.Linode != 100 {
		t.Errorf("result.DefaultFirewallIDs.Linode = %v, want %v", result.DefaultFirewallIDs.Linode, 100)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
