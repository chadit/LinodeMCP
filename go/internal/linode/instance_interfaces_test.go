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

// TestClientListInstanceInterfacesProtoUsesDocumentedRoute pins the wire call
// the list method makes. The endpoint is assembled at runtime from a base
// constant and a format verb, so nothing outside a live request proves the
// client reaches the path the API documents, and the route carries no query
// parameters: this collection is not paginated.
func TestClientListInstanceInterfacesProtoUsesDocumentedRoute(t *testing.T) {
	t.Parallel()

	const vlanLabel = "vlan-a"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			"interfaces": []map[string]any{
				{"id": 456, "mac_address": tcABCDEF02, "version": 1},
				{"id": 457, "vlan": map[string]any{"vlan_label": vlanLabel}},
			},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceInterfacesProto(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %v, want %v", len(got), 2)
	}

	if got[0].GetId() != 456 || got[0].GetMacAddress() != tcABCDEF02 {
		t.Errorf("got[0] = %v, want id 456 with MAC %v", got[0], tcABCDEF02)
	}

	// The API lists interfaces in creation order and the second element carries
	// a config arm, so this checks both that the order survives and that the
	// keyed decode reaches nested messages.
	if got[1].GetId() != 457 || got[1].GetVlan().GetVlanLabel() != vlanLabel {
		t.Errorf("got[1] = %v, want id 457 with VLAN label %v", got[1], vlanLabel)
	}
}

// TestClientListInstanceInterfacesProtoAPIError pins that an upstream failure
// still surfaces as *linode.APIError. The list method wraps request failures in
// a NetworkError, so callers reach the status only through the unwrap chain.
func TestClientListInstanceInterfacesProtoAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Interfaces {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces)
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

	_, err := client.ListInstanceInterfacesProto(t.Context(), 123)
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

func TestClientListInstanceInterfacesProtoRejectsInvalidID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.ListInstanceInterfacesProto(t.Context(), 0)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if called.Load() {
		t.Error("invalid IDs should not reach upstream server")
	}
}

func TestClientGetInstanceInterfaceSuccess(t *testing.T) {
	t.Parallel()

	want := linode.InstanceInterface{ID: 456, MACAddress: "22:00:AB:CD:EF:02", Version: 1}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedIssueAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedIssueAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	got, err := client.GetInstanceInterface(t.Context(), 123, 456)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("got is nil")
	}

	if got.ID != 456 {
		t.Errorf("got.ID = %v, want %v", got.ID, 456)
	}

	if got.MACAddress != tcABCDEF02 {
		t.Errorf("got.MACAddress = %v, want %v", got.MACAddress, tcABCDEF02)
	}
}

func TestClientGetInstanceInterfaceAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcLinodeInstances123Interfaces456 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcLinodeInstances123Interfaces456)
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

	_, err := client.GetInstanceInterface(t.Context(), 123, 456)
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

func TestClientGetInstanceInterfaceRejectsInvalidIDs(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called.Store(true)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "token", nil, linode.WithMaxRetries(0))

	_, err := client.GetInstanceInterface(t.Context(), 0, 456)
	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	_, err = client.GetInstanceInterface(t.Context(), 123, 0)
	if !errors.Is(err, linode.ErrInterfaceIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrInterfaceIDPositive)
	}

	if called.Load() {
		t.Error("invalid IDs should not reach upstream server")
	}
}
