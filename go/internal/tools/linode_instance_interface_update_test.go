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
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const (
	toolLinodeInstanceInterfaceUpdate = "linode_instance_interface_update"
	publicInterfaceUpdateJSON         = `{"public":{"ipv4":{"addresses":[{"address":"auto","primary":true}]}},"default_route":{"ipv4":true}}`
	vpcInterfaceUpdateJSON            = `{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"auto","primary":true,"nat_1_1_address":"auto"}],"ranges":[{"range":"/28"}]}},"default_route":{"ipv4":true}}`
	vlanInterfaceUpdateJSON           = `{"vlan":{"vlan_label":"backend","ipam_address":"10.0.0.1/24"}}`
)

func TestLinodeInstanceInterfaceUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}}}}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, toolLinodeInstanceInterfaceUpdate, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyInterfaceID, "schema should include interface_id")
		assert.Contains(t, props, keyInterface, "schema should include interface")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: "separator interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "query interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "traversal interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue, keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "missing interface id", args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: "zero interface id", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(0), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true}, wantContains: errInterfaceIDPositive},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: map[string]any{keyIPv4: keyAddress}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: "unknown interface field", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"public":{},"typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: "missing interface type", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: jsonObjectEmpty, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "multiple interface types", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"public":{},"vlan":{"vlan_label":"backend"}}`, keyConfirm: true}, wantContains: errInterfaceTypeExactlyOne},
		{name: "invalid vpc subnet", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"vpc":{"subnet_id":0}}`, keyConfirm: true}, wantContains: "interface.vpc.subnet_id must be a positive integer"},
		{name: "blank vlan label", args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: `{"vlan":{"vlan_label":"  "}}`, keyConfirm: true}, wantContains: "interface.vlan.vlan_label is required"},
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

	t.Run("successful vpc interface update", func(t *testing.T) {
		t.Parallel()

		updated := linode.InstanceInterface{ID: 456, VPC: &linode.InterfaceVPCConfig{SubnetID: 789}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var got linode.UpdateInstanceInterfaceRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

			if assert.NotNil(t, got.VPC, "vpc interface should be sent") {
				assert.Equal(t, 456, got.VPC.SubnetID, "subnet id should match request JSON")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(updated), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: vpcInterfaceUpdateJSON, keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "456", "response should contain interface ID")
		assert.Contains(t, textContent.Text, "Interface 456 updated on instance 123", "response should contain message")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}), "encoding error response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: publicInterfaceUpdateJSON, keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update interface 456 on instance 123")
	})

	t.Run("accepts public and vlan interface bodies", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)

			assert.Equal(t, "/linode/instances/123/interfaces/456", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceInterface{ID: 456}), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceUpdateTool(srvCfg)

		for _, body := range []string{publicInterfaceUpdateJSON, vlanInterfaceUpdateJSON} {
			req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456), keyInterface: body, keyConfirm: true})
			result, err := srvHandler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.False(t, result.IsError, "result should not be a tool error")
		}

		assert.Equal(t, int32(2), calls.Load(), "both interface body shapes should call upstream")
	})
}
