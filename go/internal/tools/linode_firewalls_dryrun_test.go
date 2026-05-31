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

func TestLinodeFirewallCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "fw-01",
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_firewall_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/firewalls", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-firewall side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "fw-01", "side effect should name the new firewall")
		assert.Contains(t, effect, "ACCEPT", "side effect should state the default policies")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeFirewallUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_firewall_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/networking/firewalls/123", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the label change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, testRenamedLabel, "side effect names the new label")
	})

	t.Run("still validates firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errFirewallIDRequired)
	})
}

func TestLinodeFirewallDeviceCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without assigning", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyFirewallID: float64(123),
			keyBetaID:     float64(456),
			keyType:       monitorAlertDefinitionToolServiceType,
			keyDryRun:     true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_firewall_device_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/networking/firewalls/123/devices", would["path"])
		assert.Nil(t, body["current_state"], "device assignment has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the device-attach side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "456", "side effect should name the attached device")
		assert.Contains(t, effect, "firewall 123", "side effect should name the firewall")
	})

	t.Run("still validates firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallDeviceCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyBetaID: float64(456),
			keyType:   monitorAlertDefinitionToolServiceType,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errFirewallIDPositive)
	})
}

func TestLinodeFirewallRulesUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
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
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_firewall_rules_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/networking/firewalls/123/rules", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallRulesUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, errFirewallIDPositive)
	})
}

func TestLinodeFirewallSettingsUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating settings", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/networking/firewalls/settings", linode.FirewallSettings{})
		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyDefaultFirewallIDs: map[string]any{keyDefaultFirewallLinode: float64(5)},
			keyDryRun:             true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_firewall_settings_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/networking/firewalls/settings", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates default_firewall_ids", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallSettingsUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "default_firewall_ids is required")
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
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_firewall_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each attached device is a dependency")

	kinds := make([]string, 0, len(deps))

	for _, entry := range deps {
		dep, ok := entry.(map[string]any)
		require.True(t, ok)

		kind, ok := dep["kind"].(string)
		require.True(t, ok)

		kinds = append(kinds, kind)
	}

	assert.ElementsMatch(t, []string{keyDefaultFirewallLinode, fwDeviceTypeNodeBalancer}, kinds)

	warnings, _ := body["warnings"].([]any)
	assert.NotEmpty(t, warnings)

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}
