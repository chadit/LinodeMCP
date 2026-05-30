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
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeNodeBalancerConfigDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeNodeBalancerConfigDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_nodebalancer_config_delete", tool.Name)
		assert.Equal(t, profiles.CapDestroy, capability)
		assert.Contains(t, tool.InputSchema.Properties, keyNodeBalancerID)
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID)
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm)
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun)
		assert.Contains(t, tool.InputSchema.Required, keyNodeBalancerID)
		assert.Contains(t, tool.InputSchema.Required, keyConfigID)
		assert.Contains(t, tool.InputSchema.Required, keyConfirm)
		require.NotNil(t, handler)
	})

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456)}, want: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: false}, want: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: boolStringTrue}, want: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: float64(1)}, want: errConfirmEqualsTrue},
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyConfirm: true}, want: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorValue, keyConfigID: float64(456), keyConfirm: true}, want: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyConfirm: true}, want: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyConfirm: true}, want: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true}, want: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true}, want: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyConfirm: true}, want: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true}, want: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(-1), keyConfirm: true}, want: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			assertErrorContains(t, result, tt.want)
		})
	}

	t.Run("dry_run returns preview without deleting", func(t *testing.T) {
		t.Parallel()

		var methods []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methods = append(methods, r.Method)
			assert.Equal(t, http.MethodGet, r.Method, "dry_run must only issue GET")
			assert.Equal(t, "/nodebalancers/123/configs", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456, keyPort: 80}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyDryRun: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError, "dry_run should not require confirm")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, `"dry_run": true`)
		assert.Contains(t, textContent.Text, `"method": "DELETE"`)
		assert.Contains(t, textContent.Text, `"path": "/nodebalancers/123/configs/456"`)
		assert.Equal(t, []string{http.MethodGet}, methods, "dry_run must not send DELETE")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			assert.Equal(t, "/nodebalancers/123/configs/456", r.URL.Path)
			assert.Empty(t, r.URL.RawQuery)
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError)
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok)
		assert.Contains(t, textContent.Text, "removed")
		assert.Contains(t, textContent.Text, "456")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true}))

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assertErrorContains(t, result, "Failed to delete config 456 from NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}
