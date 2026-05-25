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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

func TestLinodeInterfacesUpgradeTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInterfacesUpgradeTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_interfaces_upgrade", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.Description, "WARNING", "tool description should contain WARNING")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfigID, "schema should include config_id")
		assert.Contains(t, tool.InputSchema.Properties, keyDryRun, "schema should include dry_run")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be required")
	})

	confirmTests := []struct {
		name  string
		value any
		set   bool
	}{
		{name: caseMissingConfirm, set: false},
		{name: caseRequiresConfirm, value: false, set: true},
		{name: caseStringConfirmRejected, value: boolStringTrue, set: true},
		{name: caseNumericConfirmRejected, value: 1, set: true},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLinodeID: float64(123), keyConfigID: float64(4567), keyDryRun: true}
			if tt.set {
				args[keyConfirm] = tt.value
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, errConfirmEqualsTrue)
		})
	}

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1), keyConfirm: true}, wantContains: errLinodeIDMin},
		{name: caseSlashConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathSeparatorValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseQueryConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: configIDQueryValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseTraversalConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: pathTraversalValue, keyConfirm: true}, wantContains: errConfigIDMin},
		{name: "zero config id", args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(0), keyConfirm: true}, wantContains: errConfigIDMin},
		{name: caseNegativeConfigID, args: map[string]any{keyLinodeID: float64(123), keyConfigID: float64(-1), keyConfirm: true}, wantContains: errConfigIDMin},
		{name: "string dry_run", args: map[string]any{keyLinodeID: float64(123), keyDryRun: boolStringTrue, keyConfirm: true}, wantContains: "dry_run must be a boolean"},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "result should be a tool error")
			assertErrorContains(t, result, tt.wantContains)
		})
	}

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		configID := 4567

		var dryRun bool

		response := linode.UpgradeLinodeInterfacesResponse{
			ConfigID: configID,
			DryRun:   dryRun,
			Interfaces: []linode.InstanceInterface{
				{ID: 0, MACAddress: macAddressFixture},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var got linode.UpgradeLinodeInterfacesRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

			if assert.NotNil(t, got.ConfigID, "config_id should be sent") {
				assert.Equal(t, configID, *got.ConfigID, "config_id should match")
			}

			if assert.NotNil(t, got.DryRun, "dry_run should be sent") {
				assert.False(t, *got.DryRun, "dry_run should match")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(response), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123),
			keyConfigID: float64(configID),
			keyDryRun:   dryRun,
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, macAddressFixture, "response should contain MAC address")
		assert.Contains(t, textContent.Text, "4567", "response should contain config ID")
	})

	t.Run("omitted dry_run defaults to preview", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/linode/instances/123/upgrade-interfaces", r.URL.Path, "request path should match")

			var got linode.UpgradeLinodeInterfacesRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")
			assert.Nil(t, got.ConfigID, "config_id should be omitted")

			if assert.NotNil(t, got.DryRun, "dry_run should default to true") {
				assert.True(t, *got.DryRun, "dry_run default should preview")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.UpgradeLinodeInterfacesResponse{DryRun: true}), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInterfacesUpgradeTool(srvCfg)

		result, err := srvHandler(t.Context(), createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to upgrade interfaces for instance 123")
	})
}
