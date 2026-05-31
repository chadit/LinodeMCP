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

func TestLinodeVPCCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "vpc-01",
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_vpc_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/vpcs", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-VPC side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "vpc-01", "side effect should name the new VPC")
		assert.Contains(t, effect, regionUSEast, "side effect should name the target region")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeVPCUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/vpcs/123", linode.VPC{ID: 123, Label: "vpc"})
		_, _, handler := tools.NewLinodeVPCUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_vpc_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/vpcs/123", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "update surfaces the label change")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, testRenamedLabel, "side effect names the new label")
	})

	t.Run("still validates vpc_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "vpc_id is required")
	})
}

func TestLinodeVPCSubnetCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCSubnetCreateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyLabel:  "subnet-01",
			keyIPv4:   cidrV4,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_vpc_subnet_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "POST", would["method"])
		assert.Equal(t, "/vpcs/123/subnets", would["path"])
		assert.Nil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		require.Len(t, sideEffects, 1, "create surfaces the new-subnet side effect")

		effect, gotString := sideEffects[0].(string)
		require.True(t, gotString)
		assert.Contains(t, effect, "subnet-01", "side effect should name the new subnet")
		assert.Contains(t, effect, cidrV4, "side effect should name the IPv4 range")
	})

	t.Run("still validates vpc_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "subnet-01",
			keyIPv4:   cidrV4,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "vpc_id is required")
	})
}

func TestLinodeVPCSubnetUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeVPCSubnetUpdateTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/vpcs/123/subnets/456", linode.VPCSubnet{ID: 456, Label: "subnet"})
		_, _, handler := tools.NewLinodeVPCSubnetUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:    float64(123),
			keySubnetID: float64(456),
			keyLabel:    testRenamedLabel,
			keyDryRun:   true,
		}))
		require.NoError(t, err)
		require.False(t, result.IsError)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		assert.Equal(t, "linode_vpc_subnet_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		assert.Equal(t, "PUT", would["method"])
		assert.Equal(t, "/vpcs/123/subnets/456", would["path"])
		assert.Equal(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates subnet_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeVPCSubnetUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyVPCID:  float64(123),
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		require.NoError(t, err)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "subnet_id is required")
	})
}

// TestLinodeVPCDeleteToolDryRunDependencies exercises the Phase 2 Tier A walk:
// each subnet is destroyed with the VPC (and its Linode interfaces detached),
// so subnets are surfaced as cascade_deleted dependencies.
func TestLinodeVPCDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/vpcs/888": linode.VPC{ID: 888, Label: "prod-vpc"},
		"/vpcs/888/subnets": linode.PaginatedResponse[linode.VPCSubnet]{
			Data: []linode.VPCSubnet{
				{ID: 1, Label: "subnet-a", Linodes: []linode.VPCSubnetLinode{{ID: 456}}},
				{ID: 2, Label: "subnet-b"},
			},
		},
	})

	_, _, handler := tools.NewLinodeVPCDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVPCID:  float64(888),
		keyDryRun: true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_vpc_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 2, "each subnet is a cascade dependency")

	for _, entry := range deps {
		dep, gotMap := entry.(map[string]any)
		require.True(t, gotMap)
		assert.Equal(t, "vpc_subnet", dep["kind"])
		assert.Equal(t, "cascade_deleted", dep["action"])
	}

	warnings, _ := body["warnings"].([]any)
	require.NotEmpty(t, warnings)

	warning, gotString := warnings[0].(string)
	require.True(t, gotString)
	assert.Contains(t, warning, "1 Linode interface(s)")

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}

// TestLinodeVPCSubnetDeleteToolDryRunDependencies exercises the Phase 2 Tier A
// walk: Linodes with interfaces in the subnet are surfaced as detached
// dependencies, read from the subnet state (the parent VPC is fetched once).
func TestLinodeVPCSubnetDeleteToolDryRunDependencies(t *testing.T) {
	t.Parallel()

	cfg, methods := dryRunRouteServer(t, map[string]any{
		"/vpcs/888/subnets/777": linode.VPCSubnet{
			ID:      777,
			Label:   "app-subnet",
			Linodes: []linode.VPCSubnetLinode{{ID: 456}},
		},
		"/vpcs/888": linode.VPC{ID: 888, Label: "app-vpc"},
	})

	_, _, handler := tools.NewLinodeVPCSubnetDeleteTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyVPCID:    float64(888),
		keySubnetID: float64(777),
		keyDryRun:   true,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError)

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
	assert.Equal(t, "linode_vpc_subnet_delete", body["tool"])

	deps, _ := body["dependencies"].([]any)
	require.Len(t, deps, 1, "the attached Linode is the dependency")

	dep, gotMap := deps[0].(map[string]any)
	require.True(t, gotMap)
	assert.Equal(t, "instance", dep["kind"])
	assert.Equal(t, "detached", dep["action"])
	assert.InDelta(t, 456, dep["id"], 0)

	assert.NotContains(t, *methods, http.MethodDelete, "dry_run must not issue a DELETE")
}
