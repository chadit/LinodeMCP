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

const keyDiskPassword = "password"

// TestLinodeInstanceDiskPasswordResetTool verifies the instance disk password reset tool
// registers correctly, validates confirm, and resets disk root passwords.
func TestLinodeInstanceDiskPasswordResetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceDiskPasswordResetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "linode_instance_disk_password_reset", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapWrite, capability, "tool should require write capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLinodeID, "schema should include linode_id")
		assert.Contains(t, props, keyDiskID, "schema should include disk_id")
		assert.Contains(t, props, keyDiskPassword, "schema should include password")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirmRejected, args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: caseMissingLinodeID, args: map[string]any{keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: pathSeparatorValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: pathQueryValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "missing disk identifier", args: map[string]any{keyLinodeID: float64(123), keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseSlash, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathSeparatorValue, keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseQuery, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathQueryValue, keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: caseDotTraversal, args: map[string]any{keyLinodeID: float64(123), keyDiskID: pathTraversalValue, keyDiskPassword: rootPassStrong, keyConfirm: true}, wantContains: tools.ErrDiskIDRequired.Error()},
		{name: "missing password", args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyConfirm: true}, wantContains: "password is required"},
		{name: "weak password", args: map[string]any{keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: "weak", keyConfirm: true}, wantContains: "root_pass must be at least 12 characters"},
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

	t.Run("client error maps to tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10/password", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, writeErr := w.Write([]byte(`{"errors":[{"reason":"invalid password"}]}`))
			assert.NoError(t, writeErr, "writing error response should not fail")
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskPasswordResetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to reset password")
		assertErrorContains(t, result, "invalid password")
	})

	t.Run("successful password reset", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/disks/10/password", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Empty(t, r.URL.RawQuery, "request should not include query params")

			var body map[string]string
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should decode")
			assert.Equal(t, rootPassStrong, body[keyDiskPassword], "password should match request")

			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(srv.Close)

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceDiskPasswordResetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyDiskID: float64(10), keyDiskPassword: rootPassStrong, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Password reset", "response should confirm password reset")
		assert.Contains(t, textContent.Text, "10", "response should contain disk ID")
	})
}
