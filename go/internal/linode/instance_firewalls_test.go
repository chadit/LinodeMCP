package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const (
	instanceFirewallLabelFixture  = "web-firewall"
	instanceFirewallStatusEnabled = "enabled"
)

func TestClientListInstanceFirewalls(t *testing.T) {
	t.Parallel()

	firewalls := []linode.Firewall{{ID: 456, Label: instanceFirewallLabelFixture, Status: instanceFirewallStatusEnabled}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointInstanceFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceFirewallsPath)
		}

		if r.URL.Query().Get("page") != "2" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page"), "2")
		}

		if r.URL.Query().Get("page_size") != "50" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("page_size"), "50")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceFirewalls(t.Context(), 123, 2, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want %d", len(got), 1)
	}

	if got[0].Label != instanceFirewallLabelFixture {
		t.Errorf("got[0].Label = %v, want %v", got[0].Label, instanceFirewallLabelFixture)
	}
}

func TestClientListInstanceFirewallsRejectsInvalidLinodeID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.linode.com/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListInstanceFirewalls(t.Context(), 0, 0, 0)

	if !errors.Is(err, linode.ErrLinodeIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrLinodeIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListInstanceFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != endpointInstanceFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointInstanceFirewallsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyErrors: []map[string]string{{keyReason: errForbidden}},
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListInstanceFirewalls(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
