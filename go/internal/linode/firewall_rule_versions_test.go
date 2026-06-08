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

const endpointFirewallRuleVersions = "/networking/firewalls/123/history"

func TestClientListFirewallRuleVersionsSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{
		ID:     123,
		Label:  "web-firewall",
		Status: "enabled",
		Rules: linode.FirewallRules{
			Version:        2,
			Fingerprint:    firewallRulesFingerprint,
			InboundPolicy:  policyDrop,
			OutboundPolicy: policyAccept,
		},
	}

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

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if result.Rules.Version != 2 {
		t.Errorf("result.Rules.Version = %v, want %v", result.Rules.Version, 2)
	}

	if result.Rules.Fingerprint != firewallRulesFingerprint {
		t.Errorf("result.Rules.Fingerprint = %v, want %v", result.Rules.Fingerprint, firewallRulesFingerprint)
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

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)
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

	result, err := client.ListFirewallRuleVersions(t.Context(), 0)

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

		if err := json.NewEncoder(w).Encode(linode.Firewall{ID: 123}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.ListFirewallRuleVersions(t.Context(), 123)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.ID != 123 {
		t.Errorf("result.ID = %v, want %v", result.ID, 123)
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}

func TestClientGetFirewallRuleVersionSuccess(t *testing.T) {
	t.Parallel()

	firewall := linode.Firewall{ID: 123, Label: "web-firewall", Rules: linode.FirewallRules{Version: 2, Fingerprint: firewallRulesFingerprint, Inbound: []linode.FirewallRule{{Action: policyAccept, Protocol: protocolTCP, Ports: "443", Label: "allow-https"}}}}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallHistoryRule2 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallHistoryRule2)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Header.Get("Authorization") != managedContactAuthHeader {
			t.Errorf("got %v, want %v", r.Header.Get("Authorization"), managedContactAuthHeader)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(firewall); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Label != instanceFirewallLabelFixture {
		t.Errorf("result.Label = %v, want %v", result.Label, "web-firewall")
	}

	if result.Rules.Version != 2 {
		t.Errorf("result.Rules.Version = %v, want %v", result.Rules.Version, 2)
	}

	if len(result.Rules.Inbound) != 1 {
		t.Fatalf("len(result.Rules.Inbound) = %d, want %d", len(result.Rules.Inbound), 1)
	}

	if result.Rules.Inbound[0].Label != "allow-https" {
		t.Errorf("result.Rules.Inbound[0].Label = %v, want %v", result.Rules.Inbound[0].Label, "allow-https")
	}
}

func TestClientGetFirewallRuleVersionRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		firewallID int
		version    int
		wantErr    error
	}{
		{name: "zero firewall id", firewallID: 0, version: 2, wantErr: linode.ErrFirewallIDPositive},
		{name: "zero version", firewallID: 123, version: 0, wantErr: linode.ErrFirewallRuleVersionPositive},
		{name: "negative version", firewallID: 123, version: -1, wantErr: linode.ErrFirewallRuleVersionPositive},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

			result, err := client.GetFirewallRuleVersion(t.Context(), testCase.firewallID, testCase.version)

			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}

			if result != nil {
				t.Errorf("result = %v, want nil", result)
			}

			if called.Load() {
				t.Error("called.Load() = true, want false")
			}
		})
	}
}

func TestClientGetFirewallRuleVersionHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != endpointFirewallHistoryRule2 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallHistoryRule2)
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

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if result != nil {
		t.Errorf("result = %v, want nil", result)
	}
}

func TestClientGetFirewallRuleVersionRetriesTransientFailure(t *testing.T) {
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

		if r.URL.Path != endpointFirewallHistoryRule2 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, endpointFirewallHistoryRule2)
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if err := json.NewEncoder(w).Encode(linode.Firewall{ID: 123, Label: "retried"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, fastRetryOpts()...)

	result, err := client.GetFirewallRuleVersion(t.Context(), 123, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Label != "retried" {
		t.Errorf("result.Label = %v, want %v", result.Label, "retried")
	}

	if requestCount.Load() != int32(2) {
		t.Errorf("requestCount.Load() = %v, want %v", requestCount.Load(), int32(2))
	}
}
