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

func TestLinodeFirewallDevicesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallDevicesListTool(&config.Config{})

		assert.Equal(t, "linode_firewall_devices_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		assert.Contains(t, tool.InputSchema.Properties, keyPage, "schema should include page property")
		assert.Contains(t, tool.InputSchema.Properties, keyPageSize, "schema should include page_size property")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		devices := linode.PaginatedResponse[linode.FirewallDevice]{
			Data: []linode.FirewallDevice{{
				ID: 456,
				Entity: linode.FirewallDeviceEntity{
					ID:    123,
					Label: firewallDeviceLabelFixture,
					Type:  monitorAlertDefinitionToolServiceType,
					URL:   "/v4/linode/instances/123",
				},
			}},
			Page:    2,
			Pages:   3,
			Results: 1,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			assert.Equal(t, "2", r.URL.Query().Get(keyPage), "page query should match")
			assert.Equal(t, "50", r.URL.Query().Get(keyPageSize), "page_size query should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(devices))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyPage: float64(2), keyPageSize: float64(50)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, firewallDeviceLabelFixture, "response should include device entity label")
		assert.Contains(t, textContent.Text, monitorAlertDefinitionToolServiceType, "response should include entity type")
	})

	t.Run("rejects invalid firewall id before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]any{
			caseMissingFirewallPathID:   nil,
			caseZeroFirewallPathID:      float64(0),
			caseSlashFirewallPathID:     paymentMethodIDSlash,
			caseQueryFirewallPathID:     databaseInvalidInstanceIDQuery,
			caseTraversalFirewallPathID: pathTraversalValue,
		}

		for name, rawID := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called.Store(true)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

				args := map[string]any{}
				if rawID != nil {
					args[keyFirewallID] = rawID
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid firewall_id should be rejected")
				assertErrorContains(t, result, errFirewallIDPositive)
				assert.False(t, called.Load(), "client should not be called for invalid firewall_id")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDevicesListTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_devices_list")
	})
}

func TestLinodeFirewallDeviceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallDeviceGetTool(&config.Config{})

		assert.Equal(t, "linode_firewall_device_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallDeviceID, "schema should include device_id property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallDeviceID, "schema should require device_id")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		device := linode.FirewallDevice{
			ID: 456,
			Entity: linode.FirewallDeviceEntity{
				ID:    123,
				Label: firewallDeviceLabelFixture,
				Type:  monitorAlertDefinitionToolServiceType,
				URL:   "/v4/linode/instances/123",
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices/456", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(device))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, firewallDeviceLabelFixture, "response should include device entity label")
		assert.Contains(t, textContent.Text, monitorAlertDefinitionToolServiceType, "response should include entity type")
	})

	t.Run("rejects invalid ids before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissingFirewallPathID, args: map[string]any{keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
			{name: caseZeroFirewallPathID, args: map[string]any{keyFirewallID: float64(0), keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
			{name: caseSlashFirewallPathID, args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
			{name: caseQueryFirewallPathID, args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
			{name: caseTraversalFirewallPathID, args: map[string]any{keyFirewallID: pathTraversalValue, keyFirewallDeviceID: float64(456)}, want: errFirewallIDPositive},
			{name: caseMissingFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123)}, want: errFirewallDeviceIDPositive},
			{name: caseZeroFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(0)}, want: errFirewallDeviceIDPositive},
			{name: caseSlashFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: paymentMethodIDSlash}, want: errFirewallDeviceIDPositive},
			{name: caseQueryFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: databaseInvalidInstanceIDQuery}, want: errFirewallDeviceIDPositive},
			{name: caseTraversalFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: pathTraversalValue}, want: errFirewallDeviceIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called.Store(true)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid IDs should be rejected")
				assertErrorContains(t, result, testCase.want)
				assert.False(t, called.Load(), "client should not be called for invalid IDs")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/networking/firewalls/123/devices/456", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceGetTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to retrieve linode_firewall_device_get")
	})
}

func TestLinodeFirewallDeviceDeleteTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		tool, capability, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})

		assert.Equal(t, "linode_firewall_device_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapDestroy, capability, "tool should be destroy capability")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallID, "schema should include firewall_id property")
		assert.Contains(t, tool.InputSchema.Properties, keyFirewallDeviceID, "schema should include device_id property")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm property")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallID, "schema should require firewall_id")
		assert.Contains(t, tool.InputSchema.Required, keyFirewallDeviceID, "schema should require device_id")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "schema should require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/networking/firewalls/123/devices/456", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request should not include query params")
			w.Header().Set("Content-Type", "application/json")
			_, writeErr := w.Write([]byte(`{}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "delete should succeed")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Firewall device removed successfully", "response should include message")
		assert.Contains(t, textContent.Text, keyFirewallID, "response should include firewall ID field")
		assert.Contains(t, textContent.Text, keyFirewallDeviceID, "response should include device ID field")
	})

	t.Run("rejects invalid confirm before client call", func(t *testing.T) {
		t.Parallel()

		cases := map[string]any{
			caseMissingConfirm:         nil,
			caseConfirmFalse:           false,
			caseStringConfirmRejected:  boolStringTrue,
			caseNumericConfirmRejected: float64(1),
		}

		for name, rawConfirm := range cases {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called.Store(true)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

				args := map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456)}
				if rawConfirm != nil {
					args[keyConfirm] = rawConfirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid confirm should be rejected")
				assertErrorContains(t, result, "confirm=true")
				assert.False(t, called.Load(), "client should not be called without confirm")
			})
		}
	})

	t.Run("rejects invalid ids before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: caseMissingFirewallPathID, args: map[string]any{keyFirewallDeviceID: float64(456), keyConfirm: true}, want: errFirewallIDPositive},
			{name: caseZeroFirewallPathID, args: map[string]any{keyFirewallID: float64(0), keyFirewallDeviceID: float64(456), keyConfirm: true}, want: errFirewallIDPositive},
			{name: caseSlashFirewallPathID, args: map[string]any{keyFirewallID: paymentMethodIDSlash, keyFirewallDeviceID: float64(456), keyConfirm: true}, want: errFirewallIDPositive},
			{name: caseQueryFirewallPathID, args: map[string]any{keyFirewallID: databaseInvalidInstanceIDQuery, keyFirewallDeviceID: float64(456), keyConfirm: true}, want: errFirewallIDPositive},
			{name: caseTraversalFirewallPathID, args: map[string]any{keyFirewallID: pathTraversalValue, keyFirewallDeviceID: float64(456), keyConfirm: true}, want: errFirewallIDPositive},
			{name: caseMissingFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyConfirm: true}, want: errFirewallDeviceIDPositive},
			{name: caseZeroFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(0), keyConfirm: true}, want: errFirewallDeviceIDPositive},
			{name: caseSlashFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: paymentMethodIDSlash, keyConfirm: true}, want: errFirewallDeviceIDPositive},
			{name: caseQueryFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: databaseInvalidInstanceIDQuery, keyConfirm: true}, want: errFirewallDeviceIDPositive},
			{name: caseTraversalFirewallDeviceID, args: map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: pathTraversalValue, keyConfirm: true}, want: errFirewallDeviceIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var called atomic.Bool

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					called.Store(true)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				}}
				_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should not return Go error")
				require.NotNil(t, result, "handler should return a result")
				assert.True(t, result.IsError, "invalid IDs should be rejected")
				assertErrorContains(t, result, testCase.want)
				assert.False(t, called.Load(), "client should not be called for invalid IDs")
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/networking/firewalls/123/devices/456", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"forbidden"}]}`))
			assert.NoError(t, writeErr)
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyFirewallID: float64(123), keyFirewallDeviceID: float64(456), keyConfirm: true}))

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "linode_firewall_device_delete failed")
	})
}

// Dry-run coverage for firewall device delete. Kept in a sibling
// function so the main test's subtest count stays under maintidx's
// threshold.
func TestLinodeFirewallDeviceDeleteToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
		assert.Contains(t, tool.InputSchema.Properties, "dry_run",
			"schema must advertise the dry_run boolean to the model")
	})

	t.Run("preview without mutating", func(t *testing.T) {
		t.Parallel()

		var methodsSeen []string

		deviceBody := `{"id":456,"created":"2024-01-01T00:00:00","updated":"2024-01-01T00:00:00","entity":{"id":789,"type":"linode","label":"web-01","url":"/linode/instances/789"}}`

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			methodsSeen = append(methodsSeen, r.Method)
			assert.Equal(t, "/networking/firewalls/123/devices/456", r.URL.Path)

			if r.Method == http.MethodGet {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(deviceBody))

				return
			}

			t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID:       float64(123),
			keyFirewallDeviceID: float64(456),
			keyDryRun:           true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		textContent, isText := result.Content[0].(mcp.TextContent)
		require.True(t, isText)

		var body map[string]any
		require.NoError(t, json.Unmarshal([]byte(textContent.Text), &body))
		assert.Equal(t, true, body[keyDryRun])
		assert.Equal(t, "linode_firewall_device_delete", body["tool"])

		would, isWouldObject := body["would_execute"].(map[string]any)
		require.True(t, isWouldObject)
		assert.Equal(t, "DELETE", would["method"])
		assert.Equal(t, "/networking/firewalls/123/devices/456", would["path"])

		assert.Equal(t, []string{http.MethodGet}, methodsSeen,
			"dry_run must only issue a single GET, never DELETE")
	})

	t.Run("does not require confirm", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":456,"created":"2024-01-01T00:00:00","updated":"2024-01-01T00:00:00","entity":{"id":789,"type":"linode","label":"web-01","url":"/linode/instances/789"}}`))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		}}
		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID:       float64(123),
			keyFirewallDeviceID: float64(456),
			keyDryRun:           true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.False(t, result.IsError,
			"dry_run without confirm must succeed; confirm only gates real execution")
	})

	t.Run("dry_run still rejects non-positive firewall_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID:       float64(-1),
			keyFirewallDeviceID: float64(456),
			keyDryRun:           true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError,
			"dry_run with non-positive firewall_id must error the same way the real call would")
		assertErrorContains(t, result, "firewall_id must be a positive integer")
	})

	t.Run("dry_run still rejects non-positive device_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeFirewallDeviceDeleteTool(&config.Config{})
		req := createRequestWithArgs(t, map[string]any{
			keyFirewallID:       float64(123),
			keyFirewallDeviceID: float64(-1),
			keyDryRun:           true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.IsError,
			"dry_run with non-positive device_id must error the same way the real call would")
		assertErrorContains(t, result, "device id must be a positive integer")
	})
}
