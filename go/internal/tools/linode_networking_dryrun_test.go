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

const networkingAssignmentJSON = `[{"address":"192.0.2.1","linode_id":123}]`

func TestLinodeNodeBalancerCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeNodeBalancerCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeNodeBalancerCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeNodeBalancerCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyIPv4:   reservedIPv4Fixture,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/nodebalancers") {
		t.Errorf("got %v, want %v", would["path"], "/nodebalancers")
	}

	wouldBody, ok := would["body"].(map[string]any)
	if !ok {
		t.Fatalf("would_execute.body is not an object: %v", would["body"])
	}

	wantBody := map[string]any{keyRegion: regionUSEast, keyIPv4: reservedIPv4Fixture}
	if !reflect.DeepEqual(wouldBody, wantBody) {
		t.Errorf("would_execute.body = %v, want %v", wouldBody, wantBody)
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, regionUSEast) {
		t.Errorf("effect does not contain %v", regionUSEast)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want %d", len(warnings), 1)
	}
}

func TestLinodeNodeBalancerCreateToolDryRunOmitsUnselectedIPv4(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeNodeBalancerCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	would, wouldOK := body["would_execute"].(map[string]any)
	if !wouldOK {
		t.Fatalf("would_execute is not an object: %v", body["would_execute"])
	}

	wouldBody, bodyOK := would["body"].(map[string]any)
	if !bodyOK {
		t.Fatalf("would_execute.body is not an object: %v", would["body"])
	}

	if _, present := wouldBody[keyIPv4]; present {
		t.Errorf("would_execute.body contains omitted %v: %v", keyIPv4, wouldBody)
	}
}

func TestLinodeNodeBalancerCreateToolDryRunStillValidatesRegion(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeNodeBalancerCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "region is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "region is required")
	}
}

func TestLinodeNodeBalancerCreateToolDryRunRejectsInvalidIPv4(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeNodeBalancerCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyIPv4:   "2001:db8::1",
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "ipv4 must be a valid IPv4 address") {
		t.Errorf("error text %q does not contain IPv4 validation message", text.Text)
	}
}

func TestLinodeNodeBalancerUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNodeBalancerUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/nodebalancers/123", linode.NodeBalancer{ID: 123, Label: "nb"})
		_, _, handler := tools.NewLinodeNodeBalancerUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(123),
			keyLabel:          testRenamedLabel,
			keyDryRun:         true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/nodebalancers/123") {
			t.Errorf("got %v, want %v", would["path"], "/nodebalancers/123")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, testRenamedLabel) {
			t.Errorf("effect does not contain %v", testRenamedLabel)
		}
	})

	t.Run("still validates nodebalancer_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "nodebalancer_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "nodebalancer_id is required")
		}
	})
}

func TestLinodeNodeBalancerFirewallUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNodeBalancerFirewallUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/nodebalancers/123/firewalls", map[string]any{
			keyData: []map[string]any{{keyID: 456, keyLabel: nodeBalancerFirewallLabel, keyStatus: statusEnabled}},
			keyPage: 1, keyPages: 1, keyResults: 1,
		})
		_, _, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(123),
			keyFirewallIDs:    []any{float64(456)},
			keyDryRun:         true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_firewall_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_firewall_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], tcNodebalancers123Firewalls) {
			t.Errorf("got %v, want %v", would["path"], tcNodebalancers123Firewalls)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates firewall IDs", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(123),
			keyDryRun:         true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "firewall_ids is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "firewall_ids is required")
		}
	})
}

func TestLinodeNetworkingIPUpdateRDNSToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPUpdateRDNSTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/networking/ips/"+testPublicIPv4, linode.IPAddress{Address: testPublicIPv4})
		_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyAddress: testPublicIPv4,
			keyRDNS:    rdnsHostFixture,
			keyDryRun:  true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_networking_ip_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_networking_ip_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/networking/ips/"+testPublicIPv4) {
			t.Errorf("got %v, want %v", would["path"], "/networking/ips/"+testPublicIPv4)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, rdnsHostFixture) {
			t.Errorf("effect does not contain %v", rdnsHostFixture)
		}
	})

	t.Run("still validates address", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRDNS:   "host.example.com",
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "address must be a non-empty string") {
			t.Errorf("error text %q does not contain %q", text.Text, "address must be a non-empty string")
		}
	})
}

func TestLinodeNetworkingIPAllocateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without allocating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:   float64(123),
			keyType:       keyIPv4,
			purposePublic: true,
			keyDryRun:     true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_networking_ip_allocate") {
			t.Errorf("got %v, want %v", body["tool"], "linode_networking_ip_allocate")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/networking/ips") {
			t.Errorf("got %v, want %v", would["path"], "/networking/ips")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyType:       keyIPv4,
			purposePublic: true,
			keyDryRun:     true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_id must be a positive integer") {
			t.Errorf("error text %q does not contain %q", text.Text, "linode_id must be a positive integer")
		}
	})
}

func TestLinodeNetworkingIPAssignToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without assigning", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_networking_ip_assign") {
			t.Errorf("got %v, want %v", body["tool"], "linode_networking_ip_assign")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/networking/ips/assign") {
			t.Errorf("got %v, want %v", would["path"], "/networking/ips/assign")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "region must be a non-empty string") {
			t.Errorf("error text %q does not contain %q", text.Text, "region must be a non-empty string")
		}
	})
}

func TestLinodeNetworkingIPv4AssignToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without assigning", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_networking_ipv4_assign") {
			t.Errorf("got %v, want %v", body["tool"], "linode_networking_ipv4_assign")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/networking/ipv4/assign") {
			t.Errorf("got %v, want %v", would["path"], "/networking/ipv4/assign")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "region must be a non-empty string") {
			t.Errorf("error text %q does not contain %q", text.Text, "region must be a non-empty string")
		}
	})
}

func TestLinodeNetworkingIPShareToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPv4ShareTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without sharing", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4ShareTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyIPs:      databaseJSONArray,
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_networking_ipv4_share") {
			t.Errorf("got %v, want %v", body["tool"], "linode_networking_ipv4_share")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], "/networking/ipv4/share") {
			t.Errorf("got %v, want %v", would["path"], "/networking/ipv4/share")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4ShareTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyIPs:    databaseJSONArray,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_id must be a positive integer") {
			t.Errorf("error text %q does not contain %q", text.Text, "linode_id must be a positive integer")
		}
	})
}

func TestLinodeIPv6RangeCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPrefixLength: float64(64),
			keyLinodeID:     float64(12345),
			keyDryRun:       true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_ipv6_range_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_ipv6_range_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], tcNetworkingIpv6Ranges) {
			t.Errorf("got %v, want %v", would["path"], tcNetworkingIpv6Ranges)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, "/64") {
			t.Errorf("effect does not contain %v", "/64")
		}
	})

	t.Run("still validates prefix_length", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "prefix_length must be an integer between 1 and 128") {
			t.Errorf("error text %q does not contain %q", text.Text, "prefix_length must be an integer between 1 and 128")
		}
	})
}

// TestLinodeNodeBalancerDeleteToolDryRunDependencies exercises the Phase 2
// Tier A walk: each config is destroyed with the NodeBalancer, so configs are
// surfaced as cascade_deleted dependencies.
func TestLinodeNodeBalancerDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/nodebalancers/888": linode.NodeBalancer{ID: 888, Label: "prod-lb"},
		"/nodebalancers/888/configs": linode.PaginatedResponse[linode.NodeBalancerConfig]{
			Data: []linode.NodeBalancerConfig{
				{ID: 10, Port: 80, Protocol: "http"},
				{ID: 11, Port: 443, Protocol: "https"},
			},
		},
	})

	_, _, handler := tools.NewLinodeNodeBalancerDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(888),
		keyDryRun:         true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 2)
	}

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		if !gotMap {
			t.Fatal("gotMap = false, want true")
		}

		if !reflect.DeepEqual(dep[tcKind], "nodebalancer_config") {
			t.Errorf("got %v, want %v", dep[tcKind], "nodebalancer_config")
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}

// TestLinodeNodeBalancerConfigDeleteToolDryRunDependencies exercises the Phase
// 2 Tier A walk: deleting a config destroys its backend node list, so each
// node is surfaced as a cascade_deleted dependency.
func TestLinodeNodeBalancerConfigDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/nodebalancers/888/configs": linode.PaginatedResponse[linode.NodeBalancerConfig]{
			Data: []linode.NodeBalancerConfig{{ID: 10, Port: 80, Protocol: "http"}},
		},
		"/nodebalancers/888/configs/10/nodes": linode.PaginatedResponse[linode.NodeBalancerConfigNode]{
			Data: []linode.NodeBalancerConfigNode{
				{ID: 501, Label: "web-backend-1", Address: nodeBalancerNodeAddress, Mode: "accept"},
				{ID: 502, Label: "web-backend-2", Address: "192.0.2.11:80", Mode: "accept"},
			},
		},
	})

	_, _, handler := tools.NewLinodeNodeBalancerConfigDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyNodeBalancerID: float64(888),
		keyConfigID:       float64(10),
		keyDryRun:         true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_nodebalancer_config_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_nodebalancer_config_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Fatalf("len(deps) = %d, want %d", len(deps), 2)
	}

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		if !gotMap {
			t.Fatal("gotMap = false, want true")
		}

		if !reflect.DeepEqual(dep[tcKind], "nodebalancer_node") {
			t.Errorf("got %v, want %v", dep[tcKind], "nodebalancer_node")
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}
