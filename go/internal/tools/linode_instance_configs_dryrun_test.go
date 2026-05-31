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
	configGetPath       = "/linode/instances/123/configs/456"
	configIfaceGetPath  = "/linode/instances/123/configs/456/interfaces/789"
	configDevicesJSON   = `{"sda":{"disk_id":123}}`
	configIfaceAddJSON  = `{"purpose":"public"}`
	configIfaceEditJSON = `{"primary":true}`
)

func TestLinodeInstanceConfigCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, instanceGetPath, linode.Instance{ID: 123})
		_, _, handler := tools.NewLinodeInstanceConfigCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyLabel:    labelBootConfig,
			keyDevices:  configDevicesJSON,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, instanceGetPath+"/configs", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-config side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, labelBootConfig, "side effect should name the new config profile")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceConfigCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDevices:  configDevicesJSON,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinodeInstanceConfigUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configGetPath, linode.InstanceConfig{ID: 456})
		_, _, handler := tools.NewLinodeInstanceConfigUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(456),
			keyLabel:    testRenamedLabel,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, configGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceConfigDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configGetPath, linode.InstanceConfig{ID: 456})
		_, _, handler := tools.NewLinodeInstanceConfigDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(456),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, configGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates config_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeInstanceConfigDeleteTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
	})
}

func TestLinodeInstanceConfigInterfaceAddToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigInterfaceAddTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without adding", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configGetPath, linode.InstanceConfig{ID: 456})
		_, _, handler := tools.NewLinodeInstanceConfigInterfaceAddTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:  float64(123),
			keyConfigID:  float64(456),
			keyInterface: configIfaceAddJSON,
			keyDryRun:    true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_interface_add", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, configGetPath+"/interfaces", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceConfigInterfaceUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigInterfaceUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configIfaceGetPath, linode.ConfigInterfaceResponse{ID: 789})
		_, _, handler := tools.NewLinodeInstanceConfigInterfaceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(456),
			keyInterfaceID: float64(789),
			keyInterface:   configIfaceEditJSON,
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_interface_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, configIfaceGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceConfigInterfaceDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigInterfaceDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without deleting", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configIfaceGetPath, linode.ConfigInterfaceResponse{ID: 789})
		_, _, handler := tools.NewLinodeInstanceConfigInterfaceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyConfigID:    float64(456),
			keyInterfaceID: float64(789),
			keyDryRun:      true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_interface_delete", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, configIfaceGetPath, would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}

func TestLinodeInstanceConfigInterfacesReorderToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeInstanceConfigInterfacesReorderTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without reordering", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, configGetPath, linode.InstanceConfig{ID: 456})
		_, _, handler := tools.NewLinodeInstanceConfigInterfacesReorderTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(456),
			keyIDs:      `[101,102,103]`,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_instance_config_interfaces_reorder", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, configGetPath+"/interfaces/order", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})
}
