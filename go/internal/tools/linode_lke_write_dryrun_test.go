package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	lkeClusterGetPath = "/lke/clusters/123"
	lkePoolGetPath    = "/lke/clusters/123/pools/10"
	lkeNodeGetPath    = "/lke/clusters/123/nodes/abc-123"
	lkeACLGetPath     = "/lke/clusters/123/control_plane_acl"
)

// These dry-run tests intentionally discard unused constructor returns and type-assertion ok values when later assertions validate the consumed data.

func TestLinodeLKEClusterCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeLKEClusterCreateTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
	}
}

func TestLinodeLKEClusterCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeLKEClusterCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:      labelTestCluster,
		keyRegion:     regionUSEast,
		keyK8sVersion: lkeVersion129,
		keyNodePools:  lkePoolSnapshot,
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

	if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/lke/clusters") {
		t.Errorf("got %v, want %v", would["path"], "/lke/clusters")
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

	if !strings.Contains(effect, labelTestCluster) {
		t.Errorf("effect does not contain %v", labelTestCluster)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Errorf("len(warnings) = %d, want %d", len(warnings), 1)
	}
}

func TestLinodeLKEClusterCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeLKEClusterCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion:     regionUSEast,
		keyK8sVersion: lkeVersion129,
		keyNodePools:  lkePoolSnapshot,
		keyDryRun:     true,
	}))
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

func TestLinodeLKEClusterUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123, Label: labelTestCluster})
		_, _, handler := tools.NewLinodeLKEClusterUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyLabel:     testRenamedLabel,
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], lkeClusterGetPath) {
			t.Errorf("got %v, want %v", would["path"], lkeClusterGetPath)
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
}

func TestLinodeLKEClusterRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterRecycleTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without recycling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123})
		_, _, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_recycle") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_recycle")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], lkeClusterGetPath+"/recycle") {
			t.Errorf("got %v, want %v", would["path"], lkeClusterGetPath+"/recycle")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeLKEClusterRegenerateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterRegenerateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview fetches cluster not the service token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123, Label: labelTestCluster})
		_, _, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		preview := dryRunResultText(t, result)
		if strings.Contains(preview, "service_token") {
			t.Errorf("preview should not contain %v", "service_token")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(preview), &body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_regenerate") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_regenerate")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], lkeClusterGetPath+"/regenerate") {
			t.Errorf("got %v, want %v", would["path"], lkeClusterGetPath+"/regenerate")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeLKEPoolCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolCreateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview fetches the cluster", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123})
		_, _, handler := tools.NewLinodeLKEPoolCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyType:      linodeTypeGetID,
			keyCount:     float64(3),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_pool_create") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_pool_create")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], lkeClusterGetPath+"/pools") {
			t.Errorf("got %v, want %v", would["path"], lkeClusterGetPath+"/pools")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates type", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEPoolCreateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyCount:     float64(3),
			keyDryRun:    true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "type is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "type is required")
		}
	})
}

func TestLinodeLKEPoolUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkePoolGetPath, linode.LKENodePool{ID: 10, ClusterID: 123})
		_, _, handler := tools.NewLinodeLKEPoolUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyPoolID:    float64(10),
			keyCount:     float64(5),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_pool_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_pool_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], lkePoolGetPath) {
			t.Errorf("got %v, want %v", would["path"], lkePoolGetPath)
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

		if !strings.Contains(effect, "5 node") {
			t.Errorf("effect does not contain %v", "5 node")
		}
	})
}

func TestLinodeLKEPoolRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolRecycleTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without recycling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkePoolGetPath, linode.LKENodePool{ID: 10, ClusterID: 123})
		_, _, handler := tools.NewLinodeLKEPoolRecycleTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyPoolID:    float64(10),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_pool_recycle") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_pool_recycle")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], lkePoolGetPath+"/recycle") {
			t.Errorf("got %v, want %v", would["path"], lkePoolGetPath+"/recycle")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates pool_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEPoolRecycleTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "pool_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "pool_id is required")
		}
	})
}

func TestLinodeLKENodeRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKENodeRecycleTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without recycling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeNodeGetPath, linode.LKENode{ID: idAbc123, InstanceID: 456})
		_, _, handler := tools.NewLinodeLKENodeRecycleTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyNodeID:    idAbc123,
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_node_recycle") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_node_recycle")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], lkeNodeGetPath+"/recycle") {
			t.Errorf("got %v, want %v", would["path"], lkeNodeGetPath+"/recycle")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})
}

func TestLinodeLKEACLUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEACLUpdateTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeACLGetPath, linode.LKEControlPlaneACL{Enabled: true})
		_, _, handler := tools.NewLinodeLKEACLUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID:  float64(123),
			statusEnabled: true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_acl_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_acl_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], lkeACLGetPath) {
			t.Errorf("got %v, want %v", would["path"], lkeACLGetPath)
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

		if !strings.Contains(effect, "only the listed") {
			t.Errorf("effect does not contain %v", "only the listed")
		}
	})
}

func TestLinodeLKEACLDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEACLDeleteTool(&config.Config{})
		if _, ok := tool.InputSchema.Properties[keyDryRun]; !ok {
			t.Errorf("tool.InputSchema.Properties missing key %v", keyDryRun)
		}
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeACLGetPath, linode.LKEControlPlaneACL{Enabled: true})
		_, _, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
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

		if !reflect.DeepEqual(body["tool"], "linode_lke_acl_delete") {
			t.Errorf("got %v, want %v", body["tool"], "linode_lke_acl_delete")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "DELETE") {
			t.Errorf("got %v, want %v", would["method"], "DELETE")
		}

		if !reflect.DeepEqual(would["path"], lkeACLGetPath) {
			t.Errorf("got %v, want %v", would["path"], lkeACLGetPath)
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}
	})

	t.Run("still validates cluster_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEACLDeleteTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		}))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "cluster_id is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "cluster_id is required")
		}
	})
}

// TestLinodeLKEClusterDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// cascade walk: each node pool is a cascade_deleted dependency and a warning
// reports the total node count that would be destroyed.
func TestLinodeLKEClusterDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/lke/clusters/55": linode.LKECluster{ID: 55, Label: "prod-cluster"},
		"/lke/clusters/55/pools": linode.PaginatedResponse[linode.LKENodePool]{
			Data: []linode.LKENodePool{
				{ID: 1, Type: linodeTypeGetID, Count: 3},
				{ID: 2, Type: linodeTypeGetID, Count: 2},
			},
		},
	})

	_, _, handler := tools.NewLinodeLKEClusterDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(55),
		keyDryRun:    true,
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

	if !reflect.DeepEqual(body["tool"], "linode_lke_cluster_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_cluster_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Errorf("len(deps) = %d, want %d", len(deps), 2)
	}

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !reflect.DeepEqual(dep[tcKind], "node_pool") {
			t.Errorf("got %v, want %v", dep[tcKind], "node_pool")
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) == 0 {
		t.Error("warnings is empty")
	}

	warning, ok := warnings[0].(string)
	if !ok {
		t.Error("ok = false, want true")
	}

	if !strings.Contains(warning, "5 node(s)") {
		t.Errorf("warning does not contain %v", "5 node(s)")
	}

	if slices.Contains(*methods, http.MethodDelete) {
		t.Errorf("*methods should not contain %v", http.MethodDelete)
	}
}

// TestLinodeLKEPoolDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: each pool node's backing Linode is cascade_deleted with the pool, and
// a warning reports the node count. The pool state comes from FetchState, so
// the preview issues only the single state-fetch GET.
func TestLinodeLKEPoolDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, lkePoolGetPath, linode.LKENodePool{
		ID:        10,
		ClusterID: 123,
		Count:     2,
		Nodes: []linode.LKENode{
			{ID: "node-aaa", InstanceID: 9001},
			{ID: "node-bbb", InstanceID: 9002},
		},
	})

	_, _, handler := tools.NewLinodeLKEPoolDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyPoolID:    float64(10),
		keyDryRun:    true,
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

	if !reflect.DeepEqual(body["tool"], "linode_lke_pool_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_pool_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 2 {
		t.Errorf("len(deps) = %d, want %d", len(deps), 2)
	}

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		if !ok {
			t.Error("ok = false, want true")
		}

		if !reflect.DeepEqual(dep[tcKind], tcInstance) {
			t.Errorf("got %v, want %v", dep[tcKind], tcInstance)
		}

		if !reflect.DeepEqual(dep[tcAction], "cascade_deleted") {
			t.Errorf("got %v, want %v", dep[tcAction], "cascade_deleted")
		}
	}

	if body["warnings"] == nil {
		t.Fatal("expected non-empty value")
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}

// TestLinodeLKENodeDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: the node's backing Linode is cascade_deleted with the node. The node
// state comes from FetchState, so the preview issues only the single GET.
func TestLinodeLKENodeDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunGetStateServer(t, lkeNodeGetPath, linode.LKENode{
		ID:         "abc-123",
		InstanceID: 9100,
		Status:     "ready",
	})

	_, _, handler := tools.NewLinodeLKENodeDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyClusterID: float64(123),
		keyNodeID:    "abc-123",
		keyDryRun:    true,
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

	if !reflect.DeepEqual(body["tool"], "linode_lke_node_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_lke_node_delete")
	}

	deps, _ := body["dependencies"].([]any)
	if len(deps) != 1 {
		t.Errorf("len(deps) = %d, want %d", len(deps), 1)
	}

	dep, ok := deps[0].(map[string]any)
	if !ok {
		t.Error("ok = false, want true")
	}

	for key, want := range map[string]any{
		tcKind:             tcInstance,
		tcAction:           "cascade_deleted",
		keySupportTicketID: float64(9100),
	} {
		if !reflect.DeepEqual(dep[key], want) {
			t.Errorf("dep[%v] = %v, want %v", key, dep[key], want)
		}
	}

	if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
		t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
	}
}
