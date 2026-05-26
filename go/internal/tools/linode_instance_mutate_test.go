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

const mutateTemporaryError = "temporary"

func TestLinodeInstanceMutateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeInstanceMutateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_instance_mutate", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "mutate should be a write capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "linode_id", "schema should include linode_id")
		assert.Contains(t, props, "allow_auto_disk_resize", "schema should include allow_auto_disk_resize")
		assert.Contains(t, props, "confirm", "schema should include confirm")
	})

	validationTests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{name: caseMissingConfirm, args: map[string]any{keyLinodeID: float64(123)}, wantContains: errConfirmEqualsTrue},
		{name: caseFalseConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: false}, wantContains: errConfirmEqualsTrue},
		{name: caseStringConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: boolStringTrue}, wantContains: errConfirmEqualsTrue},
		{name: caseNumericConfirm, args: map[string]any{keyLinodeID: float64(123), keyConfirm: float64(1)}, wantContains: errConfirmEqualsTrue},
		{name: "missing linode id", args: map[string]any{keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseSlashLinodeID, args: map[string]any{keyLinodeID: paymentMethodIDSlash, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseQueryLinodeID, args: map[string]any{keyLinodeID: shareGroupIDQueryValue, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: caseTraversalLinodeID, args: map[string]any{keyLinodeID: pathTraversalValue, keyConfirm: true}, wantContains: errLinodeIDRequired},
		{name: "invalid allow auto disk resize", args: map[string]any{keyLinodeID: float64(123), keyConfirm: true, "allow_auto_disk_resize": boolStringTrue}, wantContains: "allow_auto_disk_resize must be a boolean"},
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

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/mutate", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: mutateTemporaryError}}}))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceMutateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to upgrade instance 123")
		assertErrorContains(t, result, mutateTemporaryError)
	})

	t.Run("successful mutate", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123/mutate", r.URL.Path, "request path should match")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Empty(t, r.URL.RawQuery, "request should not include query parameters")

			var body map[string]any

			decodeErr := json.NewDecoder(r.Body).Decode(&body)
			assert.NoError(t, decodeErr)

			if decodeErr != nil {
				return
			}

			assert.Equal(t, true, body["allow_auto_disk_resize"])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeInstanceMutateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLinodeID: float64(123), "allow_auto_disk_resize": true, keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return Go error")
		require.NotNil(t, result, "handler should return a result")
		assert.False(t, result.IsError, "result should not be a tool error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Upgrade initiated", "response should confirm mutate")
		assert.Contains(t, textContent.Text, "123", "response should contain instance id")
	})
}
