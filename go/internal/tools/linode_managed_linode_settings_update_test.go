package tools_test

import (
	"encoding/json"
	"math"
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
	managedLinodeSettingsUpdateToolName  = "linode_managed_linode_settings_update"
	managedLinodeSettingsUpdateIDKey     = "linode_id"
	managedLinodeSettingsUpdateAccessKey = "ssh_access"
	managedLinodeSettingsUpdateIPKey     = "ssh_ip"
	managedLinodeSettingsUpdatePortKey   = "ssh_port"
	managedLinodeSettingsUpdateUserKey   = "ssh_user"
	managedLinodeSettingsUpdatePortError = "ssh_port must be an integer between 1 and 65535"
)

func TestLinodeManagedLinodeSettingsUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

		assert.Equal(t, managedLinodeSettingsUpdateToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "tool should require admin capability")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.Contains(t, tool.InputSchema.Required, managedLinodeSettingsUpdateIDKey, "linode_id must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32

		port := 2222
		user := keyGrantLinode
		settings := linode.ManagedLinodeSettings{
			ID:    123,
			Label: managedLinodeSettingsLabel,
			Group: managedLinodeSettingsGroup,
			SSH:   linode.ManagedLinodeSettingsSSH{Access: true, IP: ip203_0_113_1, Port: &port, User: &user},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedLinodeSettingsPath+"/123", r.URL.Path, "request path should include Linode ID")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var got map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got), "request body should decode")

			ssh, ok := got["ssh"].(map[string]any)
			if assert.True(t, ok, "ssh should be an object") {
				assert.Equal(t, true, ssh["access"])
				assert.Equal(t, ip203_0_113_1, ssh["ip"])
				assert.InDelta(t, float64(port), ssh["port"], 0)
				assert.Equal(t, user, ssh["user"])
			}

			assert.NotContains(t, got, "group", "read-only group must not be sent")
			assert.NotContains(t, got, "id", "read-only id must not be sent")
			assert.NotContains(t, got, "label", "read-only label must not be sent")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(settings))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{
			managedLinodeSettingsUpdateIDKey:     123,
			managedLinodeSettingsUpdateAccessKey: true,
			managedLinodeSettingsUpdateIPKey:     ip203_0_113_1,
			managedLinodeSettingsUpdatePortKey:   port,
			managedLinodeSettingsUpdateUserKey:   user,
			keyConfirm:                           true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		assert.Equal(t, int32(1), calls.Load(), "client should be called once")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, managedLinodeSettingsLabel, "response should contain Linode label")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			confirm    any
			setConfirm bool
		}{
			{name: caseMissingConfirm},
			{name: caseFalseConfirm, confirm: false, setConfirm: true},
			{name: caseStringConfirm, confirm: boolStringTrue, setConfirm: true},
			{name: caseNumericConfirm, confirm: 1, setConfirm: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

				args := map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateAccessKey: true}
				if testCase.setConfirm {
					args[keyConfirm] = testCase.confirm
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("invalid input rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingLinodeID, args: map[string]any{managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: "zero linode id", args: map[string]any{managedLinodeSettingsUpdateIDKey: 0, managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: caseSlashLinodeID, args: map[string]any{managedLinodeSettingsUpdateIDKey: pathSeparatorValue, managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: caseQueryLinodeID, args: map[string]any{managedLinodeSettingsUpdateIDKey: "123?x=1", managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: caseTraversalLinodeID, args: map[string]any{managedLinodeSettingsUpdateIDKey: pathTraversalValue, managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}, wantMessage: errLinodeIDPositive},
			{name: managedContactUpdateEmptyCase, args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, keyConfirm: true}, wantMessage: "at least one mutable SSH setting is required"},
			{name: "numeric ssh ip", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateIPKey: 123, keyConfirm: true}, wantMessage: "ssh_ip must be a string"},
			{name: "string ssh access", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateAccessKey: boolStringTrue, keyConfirm: true}, wantMessage: "ssh_access must be a boolean"},
			{name: "zero ssh port", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdatePortKey: 0, keyConfirm: true}, wantMessage: managedLinodeSettingsUpdatePortError},
			{name: "large ssh port", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdatePortKey: 65536, keyConfirm: true}, wantMessage: managedLinodeSettingsUpdatePortError},
			{name: "fractional ssh port", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdatePortKey: 22.5, keyConfirm: true}, wantMessage: managedLinodeSettingsUpdatePortError},
			{name: "infinite ssh port", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdatePortKey: math.Inf(1), keyConfirm: true}, wantMessage: managedLinodeSettingsUpdatePortError},
			{name: "nan ssh port", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdatePortKey: math.NaN(), keyConfirm: true}, wantMessage: managedLinodeSettingsUpdatePortError},
			{name: "numeric ssh user", args: map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateUserKey: 123, keyConfirm: true}, wantMessage: "ssh_user must be a string"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				var calls atomic.Int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					calls.Add(1)
					w.WriteHeader(http.StatusOK)
				}))
				t.Cleanup(srv.Close)

				cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
				_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)
				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				assertErrorContains(t, result, testCase.wantMessage)
				assert.Equal(t, int32(0), calls.Load(), "request validation must fail before client call")
			})
		}
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, managedLinodeSettingsPath+"/123", r.URL.Path, "request path should include Linode ID")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeManagedLinodeSettingsUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{managedLinodeSettingsUpdateIDKey: 123, managedLinodeSettingsUpdateAccessKey: true, keyConfirm: true}))

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to update linode_managed_linode_settings_update")
		assertErrorContains(t, result, errForbidden)
	})
}
