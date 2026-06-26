package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	instanceIfaceGetPath = "/linode/instances/123/interfaces/55"
	rdnsHostFixture      = "host.example.com"
	ifacePublicJSON      = `{"public":{}}`
)

func TestLinodeInstanceFirewallsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceFirewallsUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath+"/firewalls", linode.PaginatedResponse[linode.Firewall]{})
		_, _, handler := tools.NewLinodeInstanceFirewallsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			"firewall_ids": []any{},
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_firewall_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_firewall_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/firewalls") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/firewalls")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceIPAllocateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceIPAllocateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without allocating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceIPAllocateTool(dryRunNoCallServer(t))

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

		if !reflect.DeepEqual(body["tool"], "linode_instance_ip_allocate") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_ip_allocate")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/ips") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/ips")
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}
	})

	t.Run("still validates type", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceIPAllocateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "type is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "type is required")
		}
	})
}

func TestLinodeInstanceIPUpdateRDNSToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceIPUpdateRDNSTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath+"/ips/"+testPublicIPv4,
			linode.IPAddress{Address: testPublicIPv4})
		_, _, handler := tools.NewLinodeInstanceIPUpdateRDNSTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyAddress:  testPublicIPv4,
			keyRDNS:     rdnsHostFixture,
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_ip_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_ip_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/ips/"+testPublicIPv4) {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/ips/"+testPublicIPv4)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates rdns", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceIPUpdateRDNSTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyAddress:  testPublicIPv4,
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "rdns is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "rdns is required")
		}
	})
}

func TestLinodeInstanceInterfaceAddToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceInterfaceAddTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without adding", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceInterfaceAddTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:  float64(123),
			keyInterface: ifacePublicJSON,
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_interface_add") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_interface_add")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/interfaces") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/interfaces")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates interface", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceInterfaceAddTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeInstanceInterfaceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceInterfaceUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceIfaceGetPath, linode.InstanceInterface{ID: 55})
		_, _, handler := tools.NewLinodeInstanceInterfaceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:        float64(123),
			keyInterfaceID:     float64(55),
			keyInterfacePublic: map[string]any{},
			keyDryRun:          true,
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_interface_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_interface_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], instanceIfaceGetPath) {
			t.Errorf("got %v, want %v", would["path"], instanceIfaceGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates interface_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceInterfaceUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:        float64(123),
			keyInterfacePublic: map[string]any{},
			keyDryRun:          true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

func TestLinodeInstanceInterfaceSettingsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceInterfaceSettingsUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath+"/interfaces/settings",
			linode.InstanceInterfaceSettings{})
		_, _, handler := tools.NewLinodeInstanceInterfaceSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:      float64(123),
			"network_helper": true,
			keyDryRun:        true,
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_interface_settings_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_interface_settings_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/interfaces/settings") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/interfaces/settings")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceInterfaceDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceInterfaceDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceIfaceGetPath, linode.InstanceInterface{ID: 55})
		_, _, handler := tools.NewLinodeInstanceInterfaceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyInterfaceID: float64(55),
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_interface_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_interface_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], instanceIfaceGetPath) {
			t.Errorf("got %v, want %v", would["path"], instanceIfaceGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
