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

const endpointFirewallRules = "/networking/firewalls/123/rules"

func TestClientListFirewallRulesSuccess(t *testing.T) {
	t.Parallel()

	rules := linode.FirewallRules{
		InboundPolicy:  policyDrop,
		OutboundPolicy: policyAccept,
		Inbound: []linode.FirewallRule{{
			Action:   policyAccept,
			Protocol: protocolTCP,
			Ports:    "443",
			Label:    firewallRuleLabelAllowHTTPS,
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallRules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRules)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(rules); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.InboundPolicy != policyDrop {
		t.Errorf("result.InboundPolicy = %v, want %v", result.InboundPolicy, policyDrop)
	}

	if result.OutboundPolicy != policyAccept {
		t.Errorf("result.OutboundPolicy = %v, want %v", result.OutboundPolicy, policyAccept)
	}

	if len(result.Inbound) != 1 {
		t.Fatalf("len(result.Inbound) = %d, want %d", len(result.Inbound), 1)
	}

	if result.Inbound[0].Label != firewallRuleLabelAllowHTTPS {
		t.Errorf("result.Inbound[0].Label = %v, want %v", result.Inbound[0].Label, firewallRuleLabelAllowHTTPS)
	}
}

func TestClientListFirewallRulesRejectsInvalidFirewallID(t *testing.T) {
	t.Parallel()

	var called atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRules(t.Context(), 0)

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

func TestClientListFirewallRulesHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallRules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRules)
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

	result, err := client.ListFirewallRules(t.Context(), 123)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientListFirewallRulesRetriesTransientFailure(t *testing.T) {
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

		if r.URL.Path != endpointFirewallRules {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallRules)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.FirewallRules{InboundPolicy: policyDrop}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRules(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.InboundPolicy != policyDrop {
		t.Errorf("result.InboundPolicy = %v, want %v", result.InboundPolicy, policyDrop)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
