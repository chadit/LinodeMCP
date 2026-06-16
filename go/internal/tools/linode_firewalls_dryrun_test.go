package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

func TestLinodeFirewallCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeFirewallCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeFirewallCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  "fw-01",
		keyDryRun: true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/networking/firewalls") {
		t.Errorf("got %v, want %v", would["path"], "/networking/firewalls")
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Error("gotString = false, want true")
	}

	if !strings.Contains(effect, "fw-01") {
		t.Errorf("effect does not contain %v", "fw-01")
	}

	if !strings.Contains(effect, "ACCEPT") {
		t.Errorf("effect does not contain %v", "ACCEPT")
	}
}

func TestLinodeFirewallCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

func TestLinodeFirewallUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/networking/firewalls/123", linode.Firewall{ID: 123, Label: "fw"})
		_, _, handler := tools.NewLinodeFirewallUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(123),
			keyLabel:      testRenamedLabel,
			keyDryRun:     true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_firewall_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_firewall_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/networking/firewalls/123") {
			t.Errorf("got %v, want %v", would["path"], "/networking/firewalls/123")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Error("gotString = false, want true")
		}

		if !strings.Contains(effect, testRenamedLabel) {
			t.Errorf("effect does not contain %v", testRenamedLabel)
		}
	})

	t.Run("still validates firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errFirewallIDRequired) {
			t.Errorf("error text %q does not contain %q", text.Text, errFirewallIDRequired)
		}
	})
}

func TestLinodeFirewallDeviceCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeFirewallDeviceCreateToolDryRunPreviewWithoutAssigning(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(123),
		keyBetaID:     float64(456),
		keyType:       monitorAlertDefinitionToolServiceType,
		keyDryRun:     true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_device_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_device_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], tcNetworkingFirewalls123Devices) {
		t.Errorf("got %v, want %v", would["path"], tcNetworkingFirewalls123Devices)
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Errorf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Error("gotString = false, want true")
	}

	if !strings.Contains(effect, "456") {
		t.Errorf("effect does not contain %v", "456")
	}

	if !strings.Contains(effect, "firewall 123") {
		t.Errorf("effect does not contain %v", "firewall 123")
	}
}

func TestLinodeFirewallDeviceCreateToolDryRunStillValidatesFirewallId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyBetaID: float64(456),
		keyType:   monitorAlertDefinitionToolServiceType,
		keyDryRun: true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errFirewallIDPositive) {
		t.Errorf("error text %q does not contain %q", text.Text, errFirewallIDPositive)
	}
}

func TestLinodeFirewallRulesUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without replacing rules", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/networking/firewalls/123/rules",
			linode.FirewallRules{InboundPolicy: "ACCEPT", OutboundPolicy: "DROP"})
		_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(123),
			keyInbound:    databaseJSONArray,
			keyOutbound:   databaseJSONArray,
			keyDryRun:     true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_firewall_rules_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_firewall_rules_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], tcNetworkingFirewalls123Rules) {
			t.Errorf("got %v, want %v", would["path"], tcNetworkingFirewalls123Rules)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errFirewallIDPositive) {
			t.Errorf("error text %q does not contain %q", text.Text, errFirewallIDPositive)
		}
	})
}

func TestLinodeFirewallSettingsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating settings", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/networking/firewalls/settings", linode.FirewallSettings{})
		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(5)},
			keyDryRun:             true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_firewall_settings_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_firewall_settings_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], tcNetworkingFirewallsSettings) {
			t.Errorf("got %v, want %v", would["path"], tcNetworkingFirewallsSettings)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates default_firewall_ids", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "default_firewall_ids is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "default_firewall_ids is required")
		}
	})
}

const fwDeviceTypeNodeBalancer = "nodebalancer"

// TestLinodeFirewallDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: the resources a firewall protects (Linodes, NodeBalancers) are
// surfaced as removed dependencies because they lose its rules on delete.
func TestLinodeFirewallDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/networking/firewalls/789": linode.Firewall{ID: 789, Label: "prod-fw"},
		"/networking/firewalls/789/devices": linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{
				{ID: 1, Entity: linode.FirewallDeviceEntity{ID: 456, Type: keyDefaultFirewallLinode, Label: "fw-host"}},
				{ID: 2, Entity: linode.FirewallDeviceEntity{ID: 99, Type: fwDeviceTypeNodeBalancer, Label: "fw-lb"}},
			},
		},
	})

	_, _, handler := tools.NewLinodeFirewallDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyFirewallID: float64(789),
		keyDryRun:     true,
	}))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_firewall_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_firewall_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Errorf("len(deps) = %d, want %d", len(deps), 2)
	}

	kinds := make([]string, 0, len(deps))

	for _, entry := range deps {
		dep, depOK := entry.(map[string]any)
		if !depOK {
			t.Error("ok = false, want true")
		}

		kind, ok := dep[tcKind].(string)
		if !ok {
			t.Error("ok = false, want true")
		}

		kinds = append(kinds, kind)
	}

	gotElems1 := slices.Clone(kinds)
	wantElems1 := slices.Clone([]string{keyDefaultFirewallLinode, fwDeviceTypeNodeBalancer})

	slices.Sort(gotElems1)
	slices.Sort(wantElems1)

	if !slices.Equal(gotElems1, wantElems1) {
		t.Errorf("elements = %v, want %v (any order)", gotElems1, []string{keyDefaultFirewallLinode, fwDeviceTypeNodeBalancer})
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
