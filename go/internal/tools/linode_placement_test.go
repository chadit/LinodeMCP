package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyPlacementGroupID     = "group_id"
	placementGroupIDError   = "group_id must be a positive integer"
	placementGroupLabel     = "PG_Miami_failover"
	placementGroupRegion    = "us-mia"
	placementGroupTypeLocal = "anti_affinity:local"
	placementGroupPolicy    = "strict"
)

func TestLinodePlacementGroupGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodePlacementGroupGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_placement_group_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, "CapRead", capability.String(), "tool should be read-only")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: "missing group id", args: map[string]any{}, wantContains: "group_id is required"},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber}, wantContains: placementGroupIDError},
		{name: "slash group id", args: map[string]any{keyPlacementGroupID: "528/529"}, wantContains: placementGroupIDError},
		{name: "query group id", args: map[string]any{keyPlacementGroupID: "528?x=1"}, wantContains: placementGroupIDError},
		{name: "traversal group id", args: map[string]any{keyPlacementGroupID: pathTraversalValue}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0"}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := createRequestWithArgs(t, tt.args)
			result, err := handler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should match")
			assert.Equal(t, "/placement/groups/528", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				"placement_group_type": placementGroupTypeLocal, "placement_group_policy": placementGroupPolicy,
				"is_compliant": true, "members": []map[string]any{{"linode_id": 123, "is_compliant": true}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, placementGroupLabel, "response should contain placement group label")
		assert.Contains(t, textContent.Text, placementGroupRegion, "response should contain placement group region")
	})
}
