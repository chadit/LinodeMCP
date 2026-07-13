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

const instanceGetPath = "/linode/instances/123"

func TestLinodeInstanceCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeInstanceCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:     regionUSEast,
		keyType:       typeG6Nanode1,
		keyFirewallID: float64(789),
		keyRootPass:   rootPassStrong,
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/linode/instances") {
		t.Errorf("got %v, want %v", would["path"], "/linode/instances")
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

	if !strings.Contains(effect, typeG6Nanode1) {
		t.Errorf("effect does not contain %v", typeG6Nanode1)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want %d", len(warnings), 1)
	}
}

func TestLinodeInstanceCreateToolDryRunStillValidatesFirewallId(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast,
		keyType:   typeG6Nanode1,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "firewall_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "firewall_id is required")
	}
}

func TestLinodeInstanceBootToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceBootTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without booting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceBootTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], toolInstanceBoot) {
			t.Errorf("got %v, want %v", body["tool"], toolInstanceBoot)
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/boot") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/boot")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates instance_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceBootTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "instance_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "instance_id is required")
		}
	})
}

func TestLinodeInstanceRebootToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceRebootTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without rebooting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceRebootTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_reboot") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_reboot")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/reboot") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/reboot")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceShutdownToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceShutdownTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without shutting down", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceShutdownTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyInstanceID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], tcLinodeInstanceShutdown) {
			t.Errorf("got %v, want %v", body["tool"], tcLinodeInstanceShutdown)
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/shutdown") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/shutdown")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeInstanceResizeToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeInstanceResizeTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeInstanceResizeToolDryRunPreviewWithoutResizing(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123, Type: "g6-nanode-1"})
	_, _, handler := tools.NewLinodeInstanceResizeTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyType:       typeG6Standard1,
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

	if !reflect.DeepEqual(body["tool"], "linode_instance_resize") {
		t.Errorf("got %v, want %v", body["tool"], "linode_instance_resize")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], instanceGetPath+"/resize") {
		t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/resize")
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

	if !strings.Contains(effect, "g6-nanode-1") {
		t.Errorf("effect does not contain %v", "g6-nanode-1")
	}

	if !strings.Contains(effect, typeG6Standard1) {
		t.Errorf("effect does not contain %v", typeG6Standard1)
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}
}

func TestLinodeInstanceResizeToolDryRunStillValidatesType(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeInstanceResizeTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyInstanceID: float64(123),
		keyDryRun:     true,
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
}

func TestLinodeInstanceCloneToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceCloneTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without cloning", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceCloneTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_clone") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_clone")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/clone") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/clone")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceCloneTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "linode_id is required")
		}
	})
}

func TestLinodeInstanceMigrateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceMigrateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without migrating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123, Region: regionUSEast})
		_, _, handler := tools.NewLinodeInstanceMigrateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyRegion:   "us-west",
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_migrate") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_migrate")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/migrate") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/migrate")
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

		if !strings.Contains(effect, regionUSEast) {
			t.Errorf("effect does not contain %v", regionUSEast)
		}

		if !strings.Contains(effect, "us-west") {
			t.Errorf("effect does not contain %v", "us-west")
		}
	})
}

func TestLinodeInstanceMutateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceMutateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123, Type: "g6-nanode-1"})
		_, _, handler := tools.NewLinodeInstanceMutateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_mutate") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_mutate")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/mutate") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/mutate")
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

		if !strings.Contains(effect, "g6-nanode-1") {
			t.Errorf("effect does not contain %v", "g6-nanode-1")
		}
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceMutateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "linode_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "linode_id is required")
		}
	})
}

func TestLinodeInstanceRescueToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceRescueTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without rescuing", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceRescueTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
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

		if !reflect.DeepEqual(body["tool"], "linode_instance_rescue") {
			t.Errorf("got %v, want %v", body["tool"], "linode_instance_rescue")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], instanceGetPath+"/rescue") {
			t.Errorf("got %v, want %v", would["path"], instanceGetPath+"/rescue")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}
