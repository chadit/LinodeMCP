package linode_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const nodeBalancerFirewallsPath = "/nodebalancers/123/firewalls"

func TestClientListNodeBalancerFirewallsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerFirewallsPath)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != authHeaderTestToken {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), authHeaderTestToken)
		}

		if !reflect.DeepEqual(r.Body, http.NoBody) {
			t.Errorf("r.Body = %v, want %v", r.Body, http.NoBody)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: "nb-firewall", keyStatus: statusEnabledFixture}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}

	if got[0].ID != 456 {
		t.Errorf("got[0].ID = %v, want %v", got[0].ID, 456)
	}

	if got[0].Label != "nb-firewall" {
		t.Errorf("got[0].Label = %v, want %v", got[0].Label, "nb-firewall")
	}

	if got[0].Status != statusEnabledFixture {
		t.Errorf("got[0].Status = %v, want %v", got[0].Status, statusEnabledFixture)
	}
}

func TestClientListNodeBalancerFirewallsForwardsPagination(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != nodeBalancerFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerFirewallsPath)
		}

		if r.URL.Query().Get(keyPage) != "2" {
			t.Errorf("r.URL.Query().Get(keyPage) = %v, want %v", r.URL.Query().Get(keyPage), "2")
		}

		if r.URL.Query().Get(keyPageSize) != "25" {
			t.Errorf("r.URL.Query().Get(keyPageSize) = %v, want %v", r.URL.Query().Get(keyPageSize), "25")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123, 2, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
}

func TestClientListNodeBalancerFirewallsRejectsInvalidNodeBalancerID(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("https://api.example.test/v4", "test-token", nil, linode.WithMaxRetries(0))
	got, err := client.ListNodeBalancerFirewalls(t.Context(), 0, 0, 0)

	if !errors.Is(err, linode.ErrNodeBalancerIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrNodeBalancerIDPositive)
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}

func TestClientListNodeBalancerFirewallsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != nodeBalancerFirewallsPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, nodeBalancerFirewallsPath)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if err := json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.ListNodeBalancerFirewalls(t.Context(), 123, 0, 0)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}

	apiErr, ok := errors.AsType[*linode.APIError](err)
	if !ok {
		t.Fatalf("error %v is not *linode.APIError", err)
	}

	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("apiErr.StatusCode = %v, want %v", apiErr.StatusCode, http.StatusForbidden)
	}

	if apiErr.Message != errForbidden {
		t.Errorf("apiErr.Message = %v, want %v", apiErr.Message, errForbidden)
	}
}

func TestClientUpdateNodeBalancerFirewallsProtoRejectsNullResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte("null")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "test-token", nil, linode.WithMaxRetries(0))

	got, err := client.UpdateNodeBalancerFirewallsProto(
		t.Context(),
		123,
		0,
		0,
		&linode.UpdateNodeBalancerFirewallsRequest{FirewallIDs: []int{456}},
	)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if got != nil {
		t.Errorf("got = %v, want nil", got)
	}
}
