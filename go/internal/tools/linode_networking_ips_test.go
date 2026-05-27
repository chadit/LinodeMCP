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
	networkingIPAddressFixture   = "198.51.100.5"
	networkingIPv6AddressFixture = "2001:db8::1"
	networkingScopedIPv6Fixture  = "fe80::1%eth0"
	networkingZoneTraversalValue = "fe80::1%../../x?y=1"
	networkingIPAssignmentJSON   = `[{"address":"198.51.100.5","linode_id":123}]`
	networkingIPShareJSON        = `["198.51.100.5"]`
	keyIPs                       = "ips"
)

func TestLinodeNetworkingIPsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

		assert.Equal(t, "linode_networking_ips_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		assert.Contains(t, tool.InputSchema.Properties, "skip_ipv6_rdns", "tool should declare skip_ipv6_rdns")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		ips := linode.PaginatedResponse[linode.IPAddress]{
			Data: []linode.IPAddress{{
				Address: networkingIPAddressFixture,
				Type:    keyIPv4,
				Public:  true,
				Region:  regionUSEast,
			}},
			Page:    1,
			Pages:   1,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/ips", r.URL.Path, "request path should match")
			assert.Equal(t, "true", r.URL.Query().Get("skip_ipv6_rdns"), "skip_ipv6_rdns query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(ips))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, networkingIPAddressFixture, "response should include IP address")
		assert.Contains(t, textContent.Text, regionUSEast, "response should include region")
	})

	t.Run("invalid skip_ipv6_rdns", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeNetworkingIPListTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{"skip_ipv6_rdns": boolStringTrue}))

		require.NoError(t, err, "handler should return MCP error result, not Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "invalid skip_ipv6_rdns should be a tool error")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "skip_ipv6_rdns must be a boolean")
	})
}

func TestLinodeNetworkingIPGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPGetTool(&config.Config{})

		assert.Equal(t, "linode_networking_ip_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		assert.Contains(t, tool.InputSchema.Properties, keyAddress, "tool should declare address")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/ips/"+networkingIPAddressFixture, r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{
				Address: networkingIPAddressFixture,
				Type:    keyIPv4,
				Public:  true,
				Region:  regionUSEast,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPAddressFixture}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, networkingIPAddressFixture, "response should include IP address")
		assert.Contains(t, textContent.Text, regionUSEast, "response should include region")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"not found"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPAddressFixture}))

		require.NoError(t, err, "handler should return MCP error result, not Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve networking IP")
		assertErrorContains(t, result, "not found")
	})

	for name, address := range map[string]any{
		caseMissingAddress:        nil,
		"non-string address":      123,
		"slash address":           "198.51.100.5/24",
		"query separator address": "198.51.100.5?bad=1",
		"traversal address":       pathTraversalValue,
		"scoped IPv6 address":     networkingScopedIPv6Fixture,
		"zone traversal address":  networkingZoneTraversalValue,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

			args := map[string]any{}
			if address != nil {
				args[keyAddress] = address
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid address should be a tool error")
			assert.Equal(t, int32(0), calls.Load(), "address rejection must happen before client call")
		})
	}

	t.Run("IPv6 success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/networking/ips/2001:db8::1", r.URL.EscapedPath(), "request path should preserve valid IPv6 segment")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{Address: networkingIPv6AddressFixture}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyAddress: networkingIPv6AddressFixture}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "IPv6 lookup should be valid")
	})
}

func TestLinodeNetworkingIPAllocateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})

		assert.Equal(t, "linode_networking_ip_allocate", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.Contains(t, tool.InputSchema.Properties, "linode_id", "tool should declare linode_id")
		assert.Contains(t, tool.InputSchema.Properties, "type", "tool should declare type")
		assert.Contains(t, tool.InputSchema.Properties, "public", "tool should declare public")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "tool should declare confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/networking/ips", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body linode.AllocateNetworkingIPRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assert.Equal(t, 123, body.LinodeID)
			assert.True(t, body.Public)
			assert.Equal(t, keyIPv4, body.Type)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.IPAddress{
				Address:  networkingIPAddressFixture,
				Type:     keyIPv4,
				Public:   true,
				Region:   regionUSEast,
				LinodeID: 123,
			}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID:   123,
			keyType:       keyIPv4,
			purposePublic: true,
			keyConfirm:    true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, networkingIPAddressFixture, "response should include IP address")
		assert.Contains(t, textContent.Text, "allocated", "response should describe allocation")
	})

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(cfg)

			args := map[string]any{keyLinodeID: 123, keyType: keyIPv4, purposePublic: true}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid confirm should be a tool error")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for name, args := range map[string]map[string]any{
		"missing linode_id": {keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"zero linode_id":    {keyLinodeID: 0, keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"decimal linode_id": {keyLinodeID: 12.5, keyType: keyIPv4, purposePublic: true, keyConfirm: true},
		"missing type":      {keyLinodeID: 123, purposePublic: true, keyConfirm: true},
		"blank type":        {keyLinodeID: 123, keyType: blankString, purposePublic: true, keyConfirm: true},
		"missing public":    {keyLinodeID: 123, keyType: keyIPv4, keyConfirm: true},
		"invalid public":    {keyLinodeID: 123, keyType: keyIPv4, purposePublic: boolStringTrue, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPAllocateTool(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid arguments should be a tool error")
		})
	}
}

func TestLinodeNetworkingIPAssignTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})

		assert.Equal(t, "linode_networking_ips_assign", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.Contains(t, tool.InputSchema.Properties, keyRegion, "tool should declare region")
		assert.Contains(t, tool.InputSchema.Properties, keyAssignments, "tool should declare assignments")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "tool should declare confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/networking/ips/assign", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body linode.AssignNetworkingIPsRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assert.Equal(t, regionUSEast, body.Region)
			assert.Len(t, body.Assignments, 1)
			assert.Equal(t, networkingIPAddressFixture, body.Assignments[0].Address)
			assert.Equal(t, 123, body.Assignments[0].LinodeID)

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingIPAssignmentJSON,
			keyConfirm:     true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "updated", "response should describe assignment update")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast,
			keyAssignments: networkingIPAssignmentJSON,
			keyConfirm:     true,
		}))

		require.NoError(t, err, "handler should return MCP error result, not Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be a tool error")
		assertErrorContains(t, result, "Failed to assign networking IPs")
		assertErrorContains(t, result, "forbidden")
	})

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeNetworkingIPAssignTool(cfg)

			args := map[string]any{keyRegion: regionUSEast, keyAssignments: networkingIPAssignmentJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid confirm should be a tool error")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for name, args := range map[string]map[string]any{
		caseMissingRegion:     {keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		caseBlankRegion:       {keyRegion: blankString, keyAssignments: networkingIPAssignmentJSON, keyConfirm: true},
		"missing assignments": {keyRegion: regionUSEast, keyConfirm: true},
		"invalid assignments": {keyRegion: regionUSEast, keyAssignments: "not-json", keyConfirm: true},
		"empty assignments":   {keyRegion: regionUSEast, keyAssignments: `[]`, keyConfirm: true},
		"missing address":     {keyRegion: regionUSEast, keyAssignments: `[{"linode_id":123}]`, keyConfirm: true},
		"invalid linode_id":   {keyRegion: regionUSEast, keyAssignments: `[{"address":"198.51.100.5","linode_id":0}]`, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPAssignTool(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid arguments should be a tool error")
		})
	}
}

func TestLinodeNetworkingIPShareTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeNetworkingIPShareTool(&config.Config{})

		assert.Equal(t, "linode_networking_ips_share", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		assert.Contains(t, tool.InputSchema.Properties, keyLinodeID, "tool should declare linode_id")
		assert.Contains(t, tool.InputSchema.Properties, keyIPs, "tool should declare ips")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "tool should declare confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm should be required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/networking/ips/share", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body linode.ShareNetworkingIPsRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assert.Equal(t, 123, body.LinodeID)

			if !assert.Len(t, body.IPs, 1) {
				return
			}

			assert.Equal(t, networkingIPAddressFixture, body.IPs[0])

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: 123,
			keyIPs:      networkingIPShareJSON,
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "updated", "response should describe sharing update")
	})

	t.Run("empty ips array", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/networking/ips/share", r.URL.Path, "request path should match")

			var body linode.ShareNetworkingIPsRequest
			if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&body)) {
				return
			}

			assert.Equal(t, 123, body.LinodeID)
			assert.Empty(t, body.IPs, "empty ips array should pass through")

			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: 123,
			keyIPs:      `[]`,
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "empty ips array should be accepted")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLinodeID: 123,
			keyIPs:      networkingIPShareJSON,
			keyConfirm:  true,
		}))

		require.NoError(t, err, "handler should return MCP error result, not Go error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be a tool error")
		assertErrorContains(t, result, "Failed to share networking IPs")
		assertErrorContains(t, result, "forbidden")
	})

	for name, confirm := range map[string]any{
		caseRequiresConfirm:        nil,
		caseFalseConfirmRejected:   false,
		caseStringConfirmRejected:  boolStringTrue,
		caseNumericConfirmRejected: 1,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32

			srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				calls.Add(1)
			}))
			t.Cleanup(srv.Close)

			cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			}}
			_, _, handler := tools.NewLinodeNetworkingIPShareTool(cfg)

			args := map[string]any{keyLinodeID: 123, keyIPs: networkingIPShareJSON}
			if confirm != nil {
				args[keyConfirm] = confirm
			}

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid confirm should be a tool error")
			assert.Equal(t, int32(0), calls.Load(), "confirm rejection must happen before client call")
		})
	}

	for name, args := range map[string]map[string]any{
		"missing linode_id": {keyIPs: networkingIPShareJSON, keyConfirm: true},
		"zero linode_id":    {keyLinodeID: 0, keyIPs: networkingIPShareJSON, keyConfirm: true},
		"decimal linode_id": {keyLinodeID: 12.5, keyIPs: networkingIPShareJSON, keyConfirm: true},
		"missing ips":       {keyLinodeID: 123, keyConfirm: true},
		"invalid ips":       {keyLinodeID: 123, keyIPs: "not-json", keyConfirm: true},
		"null ips":          {keyLinodeID: 123, keyIPs: databaseJSONNull, keyConfirm: true},
		"blank ip":          {keyLinodeID: 123, keyIPs: `[""]`, keyConfirm: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, handler := tools.NewLinodeNetworkingIPShareTool(&config.Config{})

			result, err := handler(t.Context(), createRequestWithArgs(t, args))

			require.NoError(t, err, "handler should return MCP error result, not Go error")
			require.NotNil(t, result, "result should not be nil")
			assert.True(t, result.IsError, "invalid arguments should be a tool error")
		})
	}
}
