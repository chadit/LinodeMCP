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
	keyFirewallIDs                    = "firewall_ids"
	labelAssignedInstanceFirewall     = "assigned-instance-firewall"
	errInstanceFirewallsPageSizeRange = "page_size must be an integer from 25 through 500"
	errInstanceFirewallsArray         = "firewall_ids must be an array"
	errInstanceFirewallsEntries       = "firewall_ids entries must be positive integers"
	caseInvalidInstanceFirewallsPage  = "invalid page"
	errInstanceFirewallsPageMin       = "page must be an integer greater than or equal to 1"
)

func TestLinodeInstanceFirewallListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceFirewallListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_firewall_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, "page", "schema should include page")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only schema should not include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue}, wantContains: errLinodeIDInteger},
		{name: caseNegativeLinodeID, args: map[string]any{keyLinodeID: float64(-1)}, wantContains: "linode_id must be an integer greater than or equal to 1"},
		{name: caseFractionalLinodeID, args: map[string]any{keyLinodeID: float64(123.9)}, wantContains: errLinodeIDInteger},
		{name: caseInvalidInstanceFirewallsPage, args: map[string]any{keyLinodeID: float64(123), keyPage: float64(0)}, wantContains: errInstanceFirewallsPageMin},
		{name: "invalid page size low", args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(10)}, wantContains: errPageSizeRange},
		{name: "invalid page size high", args: map[string]any{keyLinodeID: float64(123), keyPageSize: float64(501)}, wantContains: errPageSizeRange},
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

		firewalls := []linode.Firewall{{ID: 456, Label: labelWebFirewall, Status: "enabled"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get("page"), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: firewalls, keyPage: 2, keyPages: 3, keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceFirewallListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelWebFirewall, "response should contain firewall label")
		assert.Contains(t, textContent.Text, "456", "response should contain firewall ID")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: errForbidden}},
			}), "encoding error response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceFirewallListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list firewalls for instance 123")
	})
}

func TestLinodeInstanceInterfaceFirewallsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceInterfaceFirewallsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_interface_firewalls_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyInterfaceID, "schema should include interface_id")
		assert.NotContains(t, tool.InputSchema.Properties, keyConfirm, "read-only schema should not include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingLinodeID, args: map[string]any{keyInterfaceID: float64(456)}, wantContains: errLinodeIDRequired},
		{name: caseSeparatorLinodeID, args: map[string]any{keyLinodeID: pathSeparatorLinodeID, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyInterfaceID: float64(456)}, wantContains: errLinodeIDInteger},
		{name: caseMissingInterfaceID, args: map[string]any{keyLinodeID: float64(123)}, wantContains: "interface_id is required"},
		{name: caseSlashInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathSeparatorValue}, wantContains: errInterfaceIDInteger},
		{name: caseQueryInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: shareGroupIDQueryValue}, wantContains: errInterfaceIDInteger},
		{name: caseTraversalInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: pathTraversalValue}, wantContains: errInterfaceIDInteger},
		{name: caseNegativeInterfaceID, args: map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(-1)}, wantContains: "interface_id must be an integer greater than or equal to 1"},
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

		firewalls := []linode.Firewall{{ID: 789, Label: labelAssignedInstanceFirewall, Status: "enabled"}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/linode/instances/123/interfaces/456/firewalls", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
				Data:    firewalls,
				Page:    1,
				Pages:   1,
				Results: 1,
			}), "encoding response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceFirewallsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelAssignedInstanceFirewall, "response should contain firewall label")
		assert.Contains(t, textContent.Text, "789", "response should contain firewall ID")
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
		_, _, srvHandler := tools.NewLinodeInstanceInterfaceFirewallsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyLinodeID: float64(123), keyInterfaceID: float64(456)})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to list firewalls for interface 456 on instance 123")
	})
}

func TestLinodeInstanceFirewallsUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}
	tool, capability, handler := tools.NewLinodeInstanceFirewallsUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_instance_firewalls_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyFirewallIDs, "schema should include firewall_ids")
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.Contains(t, props, keyConfirm, "schema should require confirm")
	})

	confirmTests := []struct {
		name    string
		confirm any
	}{
		{name: "missing confirm"},
		{name: caseFalseConfirm, confirm: false},
		{name: caseStringConfirm, confirm: boolStringTrue},
		{name: caseNumericConfirm, confirm: float64(1)},
	}
	for _, tt := range confirmTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			args := map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}}
			if tt.confirm != nil {
				args[keyConfirm] = tt.confirm
			}

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(srv.Close)

			srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, srvHandler := tools.NewLinodeInstanceFirewallsUpdateTool(srvCfg)

			result, err := srvHandler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "confirm failure should return a tool error")
			assertErrorContains(t, result, "Set confirm=true")
			assert.Equal(t, int32(0), calls.Load(), "confirm failure must happen before client call")
		})
	}

	validationTests := []struct {
		name string
		args map[string]any
		want string
	}{
		{name: "missing linode id", args: map[string]any{keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDRequired},
		{name: "separator instance id", args: map[string]any{keyLinodeID: pathSeparatorValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "query linode id", args: map[string]any{keyLinodeID: "123?query", keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "traversal instance id", args: map[string]any{keyLinodeID: pathTraversalValue, keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "fractional instance id", args: map[string]any{keyLinodeID: float64(123.9), keyFirewallIDs: []any{float64(456)}, keyConfirm: true}, want: errLinodeIDInteger},
		{name: "missing firewall ids", args: map[string]any{keyLinodeID: float64(123), keyConfirm: true}, want: "firewall_ids is required"},
		{name: "invalid firewall ids", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: "456", keyConfirm: true}, want: errInstanceFirewallsArray},
		{name: "invalid firewall id entry", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(0)}, keyConfirm: true}, want: errInstanceFirewallsEntries},
		{name: caseInvalidInstanceFirewallsPage, args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}, keyPage: float64(0), keyConfirm: true}, want: errInstanceFirewallsPageMin},
		{name: "invalid page size", args: map[string]any{keyLinodeID: float64(123), keyFirewallIDs: []any{float64(456)}, keyPageSize: float64(501), keyConfirm: true}, want: errInstanceFirewallsPageSizeRange},
	}
	for _, tt := range validationTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := handler(t.Context(), createRequestWithArgs(t, tt.args))

			require.NoError(t, err, "handler should not return Go error")
			require.NotNil(t, result, "handler should return a result")
			assert.True(t, result.IsError, "validation failure should return a tool error")
			assertErrorContains(t, result, tt.want)
		})
	}

	t.Run("successful update", func(t *testing.T) {
		t.Parallel()

		firewalls := []linode.Firewall{{ID: 456, Label: labelAssignedInstanceFirewall, Status: statusEnabled}}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/firewalls", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")

			var body linode.UpdateInstanceFirewallsRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, []int{456, 789}, body.FirewallIDs, "request body should include firewall IDs")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.PaginatedResponse[linode.Firewall]{
				Data:    firewalls,
				Page:    2,
				Pages:   4,
				Results: 1,
			}), "encoding response should succeed")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, srvHandler := tools.NewLinodeInstanceFirewallsUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID:    float64(123),
			keyFirewallIDs: []any{float64(456), float64(789)},
			keyPage:        float64(2),
			keyPageSize:    float64(25),
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent type")
		assert.Contains(t, textContent.Text, labelAssignedInstanceFirewall, "response should include firewall data")
	})
}
