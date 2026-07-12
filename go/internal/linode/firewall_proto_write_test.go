package linode_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/linode"
)

func TestClientCreateFirewallProtoSuccess(t *testing.T) {
	t.Parallel()

	const body = `{"id":7,"label":"new-fw","status":"enabled","tags":[],` +
		`"created":"2025-01-01T00:00:00","updated":"2025-01-01T00:00:00",` +
		`"rules":{"inbound_policy":"ACCEPT","outbound_policy":"ACCEPT"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.Path != "/networking/firewalls" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	firewall, err := client.CreateFirewallProto(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall.GetId() != 7 {
		t.Errorf("firewall.Id = %v, want %v", firewall.GetId(), 7)
	}

	if firewall.GetLabel() != fwLabelNew {
		t.Errorf("firewall.Label = %v, want %v", firewall.GetLabel(), fwLabelNew)
	}

	if firewall.GetRules().GetInboundPolicy() != policyAccept {
		t.Errorf("firewall.Rules.InboundPolicy = %v, want %v", firewall.GetRules().GetInboundPolicy(), policyAccept)
	}
}

func TestClientCreateFirewallProtoHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", tcApplicationJSON)
		w.WriteHeader(http.StatusForbidden)

		if _, err := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	firewall, err := client.CreateFirewallProto(t.Context(), linode.CreateFirewallRequest{Label: fwLabelNew})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if firewall != nil {
		t.Errorf("firewall = %v, want nil", firewall)
	}
}

func TestClientUpdateFirewallProtoSuccess(t *testing.T) {
	t.Parallel()

	const body = `{"id":1,"label":"updated-fw","status":"disabled","tags":[],` +
		`"created":"2025-01-01T00:00:00","updated":"2025-01-02T00:00:00",` +
		`"rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/networking/firewalls/1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/1")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	firewall, err := client.UpdateFirewallProto(t.Context(), 1, linode.UpdateFirewallRequest{Label: tcUpdatedFw})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if firewall.GetLabel() != tcUpdatedFw {
		t.Errorf("firewall.Label = %v, want %v", firewall.GetLabel(), tcUpdatedFw)
	}

	if firewall.GetStatus() != "disabled" {
		t.Errorf("firewall.Status = %v, want %v", firewall.GetStatus(), "disabled")
	}
}

func TestClientUpdateFirewallRulesProtoSuccess(t *testing.T) {
	t.Parallel()

	const body = `{"inbound_policy":"DROP","outbound_policy":"ACCEPT",` +
		`"inbound":[{"action":"ACCEPT","protocol":"TCP","ports":"443","label":"allow-https"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		if r.URL.Path != "/networking/firewalls/123/rules" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/rules")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	rules, err := client.UpdateFirewallRulesProto(t.Context(), 123, &linode.FirewallRulesReplaceRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rules.GetInboundPolicy() != policyDrop {
		t.Errorf("rules.InboundPolicy = %v, want %v", rules.GetInboundPolicy(), policyDrop)
	}

	if len(rules.GetInbound()) != 1 {
		t.Fatalf("len(rules.Inbound) = %d, want %d", len(rules.GetInbound()), 1)
	}

	if rules.GetInbound()[0].GetLabel() != firewallRuleLabelAllowHTTPS {
		t.Errorf("rules.Inbound[0].Label = %v, want %v", rules.GetInbound()[0].GetLabel(), firewallRuleLabelAllowHTTPS)
	}
}

func TestClientUpdateFirewallRulesProtoRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.UpdateFirewallRulesProto(t.Context(), 0, &linode.FirewallRulesReplaceRequest{}); err == nil {
		t.Error("expected an error for zero firewall id, got nil")
	}

	if _, err := client.UpdateFirewallRulesProto(t.Context(), 123, nil); err == nil {
		t.Error("expected an error for nil request, got nil")
	}
}

func TestClientGetFirewallRuleVersionProtoSuccess(t *testing.T) {
	t.Parallel()

	const body = `{"id":123,"label":"web-firewall","status":"enabled","version":2,` +
		`"created":"2025-01-01T00:00:00","updated":"2025-01-02T00:00:00","tags":[],` +
		`"rules":{"inbound_policy":"ACCEPT","outbound_policy":"ACCEPT",` +
		`"inbound":[{"action":"ACCEPT","protocol":"TCP","ports":"443","label":"allow-https"}]}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/networking/firewalls/123/history/rules/2" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/123/history/rules/2")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	ruleVersion, err := client.GetFirewallRuleVersionProto(t.Context(), 123, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ruleVersion.GetVersion() != 2 {
		t.Errorf("ruleVersion.Version = %v, want %v", ruleVersion.GetVersion(), 2)
	}

	if ruleVersion.GetLabel() != "web-firewall" {
		t.Errorf("ruleVersion.Label = %v, want %v", ruleVersion.GetLabel(), "web-firewall")
	}

	if len(ruleVersion.GetRules().GetInbound()) != 1 {
		t.Fatalf("len(ruleVersion.Rules.Inbound) = %d, want %d", len(ruleVersion.GetRules().GetInbound()), 1)
	}
}

func TestClientGetFirewallTemplateProtoSuccess(t *testing.T) {
	t.Parallel()

	const body = `{"slug":"public","rules":{"inbound_policy":"DROP","outbound_policy":"ACCEPT"}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/networking/firewalls/templates/public" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/networking/firewalls/templates/public")
		}

		w.Header().Set("Content-Type", tcApplicationJSON)

		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	client := linode.NewClient(srv.URL, "my-token", nil, linode.WithMaxRetries(0))

	template, err := client.GetFirewallTemplateProto(t.Context(), "public", 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if template.GetSlug() != "public" {
		t.Errorf("template.Slug = %v, want %v", template.GetSlug(), "public")
	}

	if template.GetRules().GetInboundPolicy() != policyDrop {
		t.Errorf("template.Rules.InboundPolicy = %v, want %v", template.GetRules().GetInboundPolicy(), policyDrop)
	}
}

func TestClientGetFirewallTemplateProtoRejectsInvalidSlug(t *testing.T) {
	t.Parallel()

	client := linode.NewClient("http://example.invalid", "my-token", nil, linode.WithMaxRetries(0))

	if _, err := client.GetFirewallTemplateProto(t.Context(), "internal", 0, 0); err == nil {
		t.Error("expected an error for invalid slug, got nil")
	}
}
