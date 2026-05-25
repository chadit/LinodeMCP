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

const (
	longviewToolName        = "linode_longview_client_create"
	longviewClientLabel     = "client789"
	longviewClientAPIKey    = "longview-api-key-test"
	longviewClientInstall   = "longview-install-code-test"
	longviewClientCreatedAt = "2018-01-01T00:01:01"
	longviewClientUpdatedAt = "2018-01-02T00:01:01"
	caseNumber              = "number"
)

func TestLinodeLongviewClientCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeLongviewClientCreateTool(cfg)

		assert.Equal(t, longviewToolName, tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "Longview client creation returns setup credentials")
		assert.Contains(t, tool.Description, "WARNING", "tool should warn about returned credentials")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
	})

	t.Run("confirm required before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			confirm any
			include bool
		}{
			{name: caseMissing},
			{name: caseFalse, confirm: false, include: true},
			{name: "string", confirm: boolStringTrue, include: true},
			{name: caseNumber, confirm: 1, include: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)

				args := map[string]any{keyLabel: longviewClientLabel}
				if testCase.include {
					args[keyConfirm] = testCase.confirm
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid confirm should be an error result")
				assertErrorContains(t, result, "confirm=true")
			})
		}
	})

	t.Run("missing label rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissing, args: map[string]any{keyConfirm: true}},
			{name: caseBlank, args: map[string]any{keyConfirm: true, keyLabel: blankString}},
			{name: caseNumeric, args: map[string]any{keyConfirm: true, keyLabel: 789}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
				req := createRequestWithArgs(t, testCase.args)

				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid label should be an error result")
				assertErrorContains(t, result, errLabelRequired)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		apiClient := linode.CreatedLongviewClient{
			APIKey:      longviewClientAPIKey,
			Apps:        linode.LongviewApps{Apache: true, MySQL: true},
			Created:     longviewClientCreatedAt,
			ID:          789,
			InstallCode: longviewClientInstall,
			Label:       longviewClientLabel,
			Updated:     longviewClientUpdatedAt,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")

			var got linode.CreateLongviewClientRequest
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
			assert.Equal(t, longviewClientLabel, got.Label)

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(apiClient))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyLabel: longviewClientLabel})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "success should not be an error")
		require.NotEmpty(t, result.Content, "result should include content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Longview client created successfully")
		assert.Contains(t, textContent.Text, longviewClientAPIKey)
		assert.Contains(t, textContent.Text, longviewClientInstall)
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		t.Cleanup(srv.Close)

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientCreateTool(cfg)
		req := createRequestWithArgs(t, map[string]any{keyConfirm: true, keyLabel: longviewClientLabel})

		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		assertErrorContains(t, result, "Failed to create linode_longview_client_create")
		assertErrorContains(t, result, errForbidden)
	})
}
