package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestLinodeLKEClusterCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterCreateTool(&config.Config{})
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEClusterCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:      labelTestCluster,
			keyRegion:     regionUSEast,
			keyK8sVersion: lkeVersion129,
			keyNodePools:  lkePoolSnapshot,
			keyDryRun:     true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_lke_cluster_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "POST", would["method"])
		checkEqual(t, "/lke/clusters", would["path"])
		expectNil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "create surfaces the new-cluster side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContainsWithMode(t, false, effect, labelTestCluster, "side effect should name the new cluster")

		warnings, _ := body["warnings"].([]any)
		expectLen(t, warnings, 1, "create warns that node-pool billing starts immediately")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEClusterCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:     regionUSEast,
			keyK8sVersion: lkeVersion129,
			keyNodePools:  lkePoolSnapshot,
			keyDryRun:     true,
		}))
		expectNoError(t, err)
		checkTrueWithMode(t, false, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeLKEClusterUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_cluster_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, lkeClusterGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the label change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, testRenamedLabel, "side effect names the new label")
	})
}

func TestLinodeLKEClusterRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterRecycleTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without recycling", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123})
		_, _, handler := tools.NewLinodeLKEClusterRecycleTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_cluster_recycle", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, lkeClusterGetPath+"/recycle", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeLKEClusterRegenerateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEClusterRegenerateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview fetches cluster not the service token", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeClusterGetPath, linode.LKECluster{ID: 123, Label: labelTestCluster})
		_, _, handler := tools.NewLinodeLKEClusterRegenerateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		preview := dryRunResultText(t, result)
		assert.NotContains(t, preview, "service_token", "dry_run must not surface the rotated token credential")

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(preview), &body))
		assert.Equal(t, "linode_lke_cluster_regenerate", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, lkeClusterGetPath+"/regenerate", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeLKEPoolCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_pool_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, lkeClusterGetPath+"/pools", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates type", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEPoolCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyCount:     float64(3),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "type is required")
	})
}

func TestLinodeLKEPoolUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_pool_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, lkePoolGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the node-count change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "5 node", "side effect names the new node count")
	})
}

func TestLinodeLKEPoolRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEPoolRecycleTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_pool_recycle", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, lkePoolGetPath+"/recycle", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates pool_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEPoolRecycleTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "pool_id is required")
	})
}

func TestLinodeLKENodeRecycleToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKENodeRecycleTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_node_recycle", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, lkeNodeGetPath+"/recycle", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeLKEACLUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEACLUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_acl_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, lkeACLGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1)

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "only the listed", "enabled ACL gates API access")
	})
}

func TestLinodeLKEACLDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeLKEACLDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, lkeACLGetPath, linode.LKEControlPlaneACL{Enabled: true})
		_, _, handler := tools.NewLinodeLKEACLDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyClusterID: float64(123),
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_lke_acl_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, lkeACLGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates cluster_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeLKEACLDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "cluster_id is required")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_lke_cluster_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each node pool is a cascade-deleted dependency")

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "node_pool", dep["kind"])
		assert.Equal(t, "cascade_deleted", dep["action"])
	}

	warnings, _ := body["warnings"].([]any)
	require.NotEmpty(t, warnings)

	warning, ok := warnings[0].(string)
	require.True(t, ok)
	assert.Contains(t, warning, "5 node(s)")

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_lke_pool_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each pool node's backing Linode is a cascade dependency")

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "instance", dep["kind"])
		assert.Equal(t, "cascade_deleted", dep["action"])
	}

	assert.NotEmpty(t, body["warnings"])
	assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_lke_node_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 1, "the node's backing Linode is the dependency")

	dep, ok := deps[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "instance", dep["kind"])
	assert.Equal(t, "cascade_deleted", dep["action"])
	assert.InDelta(t, 9100, dep["id"], 0)

	assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
}
