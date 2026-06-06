package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// expect* helpers are fatal package-local checks from linode_assertions_test.go; check* helpers are nonfatal.

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
		checkEqual(t, "linode_nodebalancer_config_delete", tool.Name)
		checkEqual(t, profiles.CapDestroy, capability)
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyNodeBalancerID)
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfigID)
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyConfirm)
		expectContainsWithMode(t, false, tool.InputSchema.Properties, keyDryRun)
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyNodeBalancerID)
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfigID)
		expectContainsWithMode(t, false, tool.InputSchema.Required, keyConfirm)
		expectNotNil(t, handler)
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
		{name: caseMissingNodeBalancerID, args: map[string]any{keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDRequired},
		{name: caseSeparatorNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathSeparatorValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseQueryNodeBalancerID, args: map[string]any{keyNodeBalancerID: shareGroupIDQueryValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseTraversalNodeBalancerID, args: map[string]any{keyNodeBalancerID: pathTraversalValue, keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}, want: errNodeBalancerIDInteger},
		{name: caseMissingConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDRequired},
		{name: caseSeparatorConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseQueryConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: shareGroupIDQueryValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseTraversalConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDInteger},
		{name: caseNegativeConfigID, args: map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(-1), keyConfirm: true, keyConfirmedDryRun: true}, want: errConfigIDMin},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			expectNoError(t, err)
			expectNotNil(t, result)
			checkTrueWithMode(t, false, result.IsError)
			assertErrorContains(t, result, tt.want)
		})
	}

	t.Run("dry_run returns preview without deleting", func(t *testing.T) {
		t.Parallel()

		var methods []string

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methods = append(methods, r.Method)
			checkEqual(t, http.MethodGet, r.Method, "dry_run must only issue GET")
			w.Header().Set("Content-Type", "application/json")
			// The Phase 2 dependency walk also reads the config's backend nodes,
			// so the preview issues a second GET beyond the config-list fetch.
			if r.URL.Path == "/nodebalancers/123/configs/456/nodes" {
				checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{}}))

				return
			}

			checkEqual(t, "/nodebalancers/123/configs", r.URL.Path)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyData: []map[string]any{{keyID: 456, keyPort: 80}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyDryRun: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		checkFalseWithMode(t, false, result.IsError, "dry_run should not require confirm")
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok)
		expectContainsWithMode(t, false, textContent.Text, `"dry_run": true`)
		expectContainsWithMode(t, false, textContent.Text, `"method": "DELETE"`)
		expectContainsWithMode(t, false, textContent.Text, `"path": "/nodebalancers/123/configs/456"`)
		expectNotContains(t, methods, http.MethodDelete, "dry_run must not send DELETE")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			checkEqual(t, http.MethodDelete, r.Method)
			checkEqual(t, "/nodebalancers/123/configs/456", r.URL.Path)
			checkEmpty(t, r.URL.RawQuery)
			checkEqual(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		checkFalseWithMode(t, false, result.IsError)
		textContent, ok := result.Content[0].(mcp.TextContent)
		expectTrue(t, ok)
		expectContainsWithMode(t, false, textContent.Text, "removed")
		expectContainsWithMode(t, false, textContent.Text, "456")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			checkNoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeNodeBalancerConfigDeleteTool(srvCfg)
		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyNodeBalancerID: float64(123), keyConfigID: float64(456), keyConfirm: true, keyConfirmedDryRun: true}))

		expectNoError(t, err)
		expectNotNil(t, result)
		checkTrueWithMode(t, false, result.IsError)
		assertErrorContains(t, result, "Failed to delete config 456 from NodeBalancer 123")
		assertErrorContains(t, result, errForbidden)
	})
}
