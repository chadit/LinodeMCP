package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const instanceGetPath = "/linode/instances/123"

func TestLinodeInstanceResizeToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := gentools.NewLinodeInstanceResizeTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("tool.RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeInstanceResizeToolDryRunPreviewWithoutResizing(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123, Type: "g6-nanode-1"})
	_, _, handler := gentools.NewLinodeInstanceResizeTool(cfg)

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

	if !reflect.DeepEqual(body["tool"], toolInstanceResize) {
		t.Errorf("got %v, want %v", body["tool"], toolInstanceResize)
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

	_, _, handler := gentools.NewLinodeInstanceResizeTool(&config.Config{})

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
