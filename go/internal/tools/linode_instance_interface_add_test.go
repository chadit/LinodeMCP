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
	toolLinodeInstanceInterfaceAdd = "linode_instance_interface_add"
	publicInterfaceAddJSON         = `{"public":{"ipv4":{"addresses":[{"address":"auto","primary":true}]}},"default_route":{"ipv4":true}}`
	vpcInterfaceAddJSON            = `{"vpc":{"subnet_id":456,"ipv4":{"addresses":[{"address":"auto","primary":true,"nat_1_1_address":"auto"}],"ranges":[{"range":"/28"}]}},"default_route":{"ipv4":true},"firewall_id":321}`
	vlanInterfaceAddJSON           = `{"vlan":{"vlan_label":"backend","ipam_address":"10.0.0.1/24"}}`
)

func TestLinodeInstanceInterfaceAddTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceAddTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, toolLinodeInstanceInterfaceAdd, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "capability should be write")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "tool description should warn about mutation")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyInterface, "schema should include interface")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []instanceConfigCreateValidationCase{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterface: publicInterfaceAddJSON, keyConfirm: true}, wantContains: errLinodeIDInteger},
		{name: caseMissingInterface, args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, wantContains: errInterfaceRequired},
		{name: caseNonStringInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: map[string]any{keyIPv4: keyAddress}, keyConfirm: true}, wantContains: errInterfaceString},
		{name: caseInvalidInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: `{`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: caseNullInterface, args: map[string]any{keyLinodeID: float64(123), keyInterface: databaseJSONNull, keyConfirm: true}, wantContains: errInterfaceJSONObject},
		{name: "unknown interface field", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"public":{},"typo":true}`, keyConfirm: true}, wantContains: errInvalidInterfaceJSON},
		{name: "missing interface type", args: map[string]any{keyLinodeID: float64(123), keyInterface: jsonObjectEmpty, keyConfirm: true}, wantContains: "interface must define exactly one of public, vpc, or vlan"},
		{name: "multiple interface types", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"public":{},"vlan":{"vlan_label":"backend"}}`, keyConfirm: true}, wantContains: "interface must define exactly one of public, vpc, or vlan"},
		{name: "invalid vpc subnet", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"vpc":{"subnet_id":0}}`, keyConfirm: true}, wantContains: "interface.vpc.subnet_id must be a positive integer"},
		{name: "blank vlan label", args: map[string]any{keyLinodeID: float64(123), keyInterface: `{"vlan":{"vlan_label":"  "}}`, keyConfirm: true}, wantContains: "interface.vlan.vlan_label is required"},
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

	t.Run("successful public interface creation", func(t *testing.T) {
		t.Parallel()

		created := linode.InstanceInterface{ID: 1234, Public: &linode.InterfacePublicConfig{}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")

			var got linode.AddInstanceInterfaceRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

			if assert.NotNil(t, got.Public, "public interface should be sent") && assert.NotNil(t, got.Public.IPv4, "public IPv4 should be sent") {
				assert.Equal(t, "auto", got.Public.IPv4.Addresses[0].Address, "IPv4 address should match")
			}

			if assert.NotNil(t, got.DefaultRoute, "default route should be sent") && assert.NotNil(t, got.DefaultRoute.IPv4, "IPv4 default route should be sent") {
				assert.True(t, *got.DefaultRoute.IPv4, "default route should match")
			}

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(created), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceAddTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterface: publicInterfaceAddJSON, keyConfirm: true})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "1234", "response should contain interface ID")
		assert.Contains(t, textContent.Text, "Interface added to instance 123", "response should contain message")
	})

	t.Run("accepts vpc and vlan interface bodies", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assert.Equal(t, "/linode/instances/123/interfaces", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.InstanceInterface{ID: int(calls.Load())}), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceAddTool(srvCfg)

		for _, body := range []string{vpcInterfaceAddJSON, vlanInterfaceAddJSON} {
			req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterface: body, keyConfirm: true})
			result, err := srvHandler(t.Context(), req)
			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.False(t, result.IsError, "result should not be a tool error")
		}

		assert.Equal(t, int32(2), calls.Load(), "both interface body shapes should call upstream")
	})
}
