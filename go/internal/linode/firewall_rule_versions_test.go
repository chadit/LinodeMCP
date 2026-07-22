package linode_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const endpointFirewallRuleVersions = "/networking/firewalls/123/history"

// firewallRuleVersionsObjectJSON is the documented history body: one
// firewall-shaped object whose rules.version carries the rule version. There
// is no {data:[...]} page and no top-level version on this route.
const firewallRuleVersionsObjectJSON = `{"id":123,"label":"web-firewall","status":"enabled",` +
	`"created":"2025-01-01T00:00:00","updated":"2025-01-02T00:00:00","tags":[],` +
	`"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT","version":2}}`

func TestClientListFirewallRuleVersionsSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallRuleVersions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRuleVersions)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, writeErr := w.Write([]byte(firewallRuleVersionsObjectJSON)); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersionsProto(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 1)
	}

	if result[0].GetId() != 123 || result[0].GetLabel() != "web-firewall" {
		t.Errorf("snapshot = (%d, %q), want (123, \"web-firewall\")", result[0].GetId(), result[0].GetLabel())
	}

	// The top-level version must be lifted out of rules.version, since the
	// documented object carries it nowhere else.
	if result[0].GetVersion() != 2 {
		t.Errorf("result[0].Version = %v, want %v", result[0].GetVersion(), 2)
	}

	if result[0].GetRules().GetInboundPolicy() != policyDrop {
		t.Errorf("result[0].Rules.InboundPolicy = %v, want %v", result[0].GetRules().GetInboundPolicy(), policyDrop)
	}
}

func TestClientListFirewallRuleVersionsRejectsMistypedObject(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		// id must be numeric in the proto message; a string makes the proto
		// decode itself fail rather than the shape check.
		if _, writeErr := w.Write([]byte(`{"id":"not-a-number","rules":{}}`)); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.ListFirewallRuleVersionsProto(t.Context(), 123); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientListFirewallRuleVersionsRejectsMistypedRuleVersion(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		// The proto decode discards the unknown rules.version, so only the
		// version probe sees the mistyped value; its failure must surface.
		if _, writeErr := w.Write([]byte(`{"id":123,"rules":{"version":"two"}}`)); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.ListFirewallRuleVersionsProto(t.Context(), 123); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestClientListFirewallRuleVersionsRejectsPageEnvelope(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, writeErr := w.Write([]byte(`{"data":[` + firewallRuleVersionsObjectJSON + `],"page":1,"pages":1,"results":1}`)); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	_, err := client.ListFirewallRuleVersionsProto(t.Context(), 123)
	if !errors.Is(err, linode.ErrFirewallHistoryNotObject) {
		t.Fatalf("err = %v, want %v", err, linode.ErrFirewallHistoryNotObject)
	}
}

func TestClientListFirewallRuleVersionsHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallRuleVersions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRuleVersions)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
		if writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersionsProto(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListFirewallRuleVersionsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersionsProto(t.Context(), 0)

	if !errors.Is(err, linode.ErrFirewallIDPositive) {
		t.Fatalf("error = %v, want %v", err, linode.ErrFirewallIDPositive)
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}

	if called.Load() {
		t.Error("called.Load() = true, want false")
	}
}

func TestClientListFirewallRuleVersionsRetriesTransientFailure(t *testing.T) {
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

		if r.URL.Path != endpointFirewallRuleVersions {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRuleVersions)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, writeErr := w.Write([]byte(firewallRuleVersionsObjectJSON)); writeErr != nil {
			t.Errorf("unexpected error: %v", writeErr)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRuleVersionsProto(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want %d", len(result), 2)
	}

	if result[0].GetId() != 123 {
		t.Errorf("result[0].Id = %v, want %v", result[0].GetId(), 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
