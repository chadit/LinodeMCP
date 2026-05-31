package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	keyPlacementGroupID         = "group_id"
	placementGroupIDError       = "group_id must be a positive integer"
	placementGroupIDRequired    = "group_id is required"
	placementGroupLabel         = "PG_Miami_failover"
	placementGroupRegion        = "us-mia"
	placementGroupTypeLocal     = "anti_affinity:local"
	placementGroupPolicy        = "strict"
	keyPlacementIsCompliant     = "is_compliant"
	keyPlacementGroupTypeJSON   = "placement_group_type"
	keyPlacementGroupPolicyJSON = "placement_group_policy"
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
		{name: "missing group id", args: map[string]any{}, wantContains: placementGroupIDRequired},
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
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementIsCompliant: true, "members": []map[string]any{{"linode_id": 123, keyPlacementIsCompliant: true}},
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

func TestLinodePlacementGroupDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodePlacementGroupDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_placement_group_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, "CapDestroy", capability.String(), "tool should be destroy capability")
		assert.Contains(t, tool.InputSchema.Properties, keyPlacementGroupID, "schema should include group_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Required, keyPlacementGroupID, "group_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseRequiresConfirm, args: map[string]any{keyPlacementGroupID: "528"}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyPlacementGroupID: "528", keyConfirm: 1}, wantContains: errConfirmEqualsTrue},
		{name: "missing group id", args: map[string]any{keyConfirm: true}, wantContains: placementGroupIDRequired},
		{name: "non-numeric group id", args: map[string]any{keyPlacementGroupID: notANumber, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "slash group id", args: map[string]any{keyPlacementGroupID: "528/529", keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "query group id", args: map[string]any{keyPlacementGroupID: "528?x=1", keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "traversal group id", args: map[string]any{keyPlacementGroupID: pathTraversalValue, keyConfirm: true}, wantContains: placementGroupIDError},
		{name: "zero group id", args: map[string]any{keyPlacementGroupID: "0", keyConfirm: true}, wantContains: placementGroupIDError},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var called atomic.Bool

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called.Store(true)
			}))
			defer srv.Close()

			srvCfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				},
			}
			_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

			result, err := srvHandler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
			assert.False(t, called.Load(), "validation should reject before client call")
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var requestCount atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestCount.Add(1)
			assert.Equal(t, http.MethodDelete, r.Method, "request method should match")
			assert.Equal(t, "/placement/groups/528", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"), "authorization header should match")
			assert.Equal(t, http.NoBody, r.Body, "delete request should not include a body")
			w.WriteHeader(http.StatusOK)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "deleted successfully", "response should confirm deletion")
		assert.Equal(t, int32(1), requestCount.Load(), "delete should make one request")
	})

	t.Run("dry run previews without deleting", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, http.MethodGet, r.Method, "dry_run should fetch state with GET")
			assert.Equal(t, "/placement/groups/528", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyBetaID: 528, keyLabel: placementGroupLabel, keyRegion: placementGroupRegion,
				keyPlacementGroupTypeJSON: placementGroupTypeLocal, keyPlacementGroupPolicyJSON: placementGroupPolicy,
				keyPlacementIsCompliant: true, "members": []map[string]any{},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyDryRun: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "dry_run should not be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "dry_run", "response should be a dry-run preview")
		assert.Equal(t, []string{http.MethodGet}, methodsSeen, "dry_run must not issue DELETE")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errNotFound}},
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodePlacementGroupDeleteTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyPlacementGroupID: "528", keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "client failure should be a tool error")
		assertErrorContains(t, result, "linode_placement_group_delete failed")
	})
}
