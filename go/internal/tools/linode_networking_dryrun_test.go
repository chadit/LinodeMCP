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

const networkingAssignmentJSON = `[{"address":"192.0.2.1","linode_id":123}]`

func TestLinodeNodeBalancerCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNodeBalancerCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_nodebalancer_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/nodebalancers", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-nodebalancer side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, regionUSEast, "side effect should name the target region")

		warnings, _ := body["warnings"].([]any)
		require.Len(t, warnings, 1, "create warns that billing starts immediately")
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "region is required")
	})
}

func TestLinodeNodeBalancerUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNodeBalancerUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_nodebalancer_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/nodebalancers/123", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the label change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, testRenamedLabel, "side effect names the new label")
	})

	t.Run("still validates nodebalancer_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "nodebalancer_id is required")
	})
}

func TestLinodeNodeBalancerFirewallUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNodeBalancerFirewallUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_nodebalancer_firewall_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/nodebalancers/123/firewalls", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read current firewall assignments")
	})

	t.Run("still validates firewall IDs", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNodeBalancerFirewallUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyNodeBalancerID: float64(123),
			keyDryRun:         true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "firewall_ids is required")
	})
}

func TestLinodeNetworkingIPUpdateRDNSToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPUpdateRDNSTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_networking_ip_update_rdns", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/networking/ips/"+testPublicIPv4, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the rDNS change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, rdnsHostFixture, "side effect names the new rDNS")
	})

	t.Run("still validates address", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPUpdateRDNSTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRDNS:   "host.example.com",
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "address must be a non-empty string")
	})
}

func TestLinodeNetworkingIPAllocateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_networking_ip_allocate", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/ips", would["path"])
		assert.Nil(t, body["current_state"], "allocate has no existing resource to preview")
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyType:       keyIPv4,
			purposePublic: true,
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "linode_id must be a positive integer")
	})
}

func TestLinodeNetworkingIPAssignToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without assigning", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_networking_ips_assign", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/ips/assign", would["path"])
		assert.Nil(t, body["current_state"], "bulk assign has no single resource to preview")
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "region must be a non-empty string")
	})
}

func TestLinodeNetworkingIPv4AssignToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without assigning", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_networking_ipv4_assign", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/ipv4/assign", would["path"])
		assert.Nil(t, body["current_state"], "bulk assign has no single resource to preview")
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPv4AssignTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyAssignments: networkingAssignmentJSON,
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "region must be a non-empty string")
	})
}

func TestLinodeNetworkingIPShareToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeNetworkingIPShareTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without sharing", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPShareTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyIPs:      databaseJSONArray,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_networking_ips_share", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/ipv4/share", would["path"])
		assert.Nil(t, body["current_state"], "ip share has no single resource to preview")
	})

	t.Run("still validates linode_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPShareTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyIPs:    databaseJSONArray,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "linode_id must be a positive integer")
	})
}

func TestLinodeIPv6RangeCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyPrefixLength: float64(64),
			keyDryRun:       true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_ipv6_range_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/ipv6/ranges", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-range side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "/64", "side effect should state the prefix length")
	})

	t.Run("still validates prefix_length", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeIPv6RangeCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "prefix_length must be an integer between 1 and 128")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_nodebalancer_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each config is a cascade dependency")

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		require.True(t, gotMap)
		assert.Equal(t, "nodebalancer_config", dep["kind"])
		assert.Equal(t, "cascade_deleted", dep["action"])
	}

	assert.NotEmpty(t, body["warnings"])
	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_nodebalancer_config_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each backend node is a cascade dependency")

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		require.True(t, gotMap)
		assert.Equal(t, "nodebalancer_node", dep["kind"])
		assert.Equal(t, "cascade_deleted", dep["action"])
	}

	assert.NotEmpty(t, body["warnings"])
	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}
