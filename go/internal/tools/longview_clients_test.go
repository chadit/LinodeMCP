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

func TestLinodeLongviewClientsTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeLongviewClientsTool(cfg)
		assert.Equal(t, "linode_longview_clients", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read-only")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData: []map[string]any{{
					"api_key":      "longview-api-key-secret",
					"apps":         map[string]bool{"apache": true, databaseEngineName: true, "nginx": false},
					"created":      "2018-01-01T00:01:01",
					keyID:          789,
					"install_code": "longview-install-code-secret",
					keyLabel:       "client789",
					keyUpdated:     "2018-01-02T00:01:01",
				}},
				keyPage:    2,
				keyPages:   3,
				keyResults: 75,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "client789", "response should contain client label")
		assert.NotContains(t, textContent.Text, "longview-api-key-secret", "response should not expose Longview API key")
		assert.NotContains(t, textContent.Text, "longview-install-code-secret", "response should not expose Longview install code")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, "/longview/clients", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve linode_longview_clients", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("invalid pagination rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageFractional, args: map[string]any{keyPage: 1.5}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
			{name: paginationCasePageSizeFractional, args: map[string]any{keyPageSize: 25.5}, wantMessage: errPageSizeInteger},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientsTool(cfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage, "response should describe validation error")
			})
		}
	})
}

func TestLinodeLongviewClientUpdateTool(t *testing.T) {
	t.Parallel()

	const (
		longviewClientUpdatedLabel  = "renamed-client"
		errLongviewClientIDPositive = "client_id must be a positive integer"
		errLongviewClientLabel      = "label must be 3-32 characters"
	)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)
		assert.Equal(t, "linode_longview_client_update", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapAdmin, capability, "update should require admin capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyClientID, "schema should include client_id")
		assert.Contains(t, tool.InputSchema.Properties, keyLabel, "schema should include label")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyClientID, "client_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyLabel, "label must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should match")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, map[string]any{keyLabel: longviewClientUpdatedLabel}, body, "request body should only include label")

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"api_key":      "longview-api-key-secret",
				"apps":         map[string]bool{"apache": true, databaseEngineName: true, "nginx": false},
				keyID:          789,
				"install_code": "longview-install-code-secret",
				keyLabel:       longviewClientUpdatedLabel,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, longviewClientUpdatedLabel, "response should contain updated label")
		assert.NotContains(t, textContent.Text, "longview-api-key-secret", "response should not expose Longview API key")
		assert.NotContains(t, textContent.Text, "longview-install-code-secret", "response should not expose Longview install code")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to update linode_longview_client_update", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseFalse, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)

				args := map[string]any{keyClientID: 789, keyLabel: longviewClientUpdatedLabel}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "response should require confirm=true")
			})
		}
	})

	t.Run("validation rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing client id", args: map[string]any{keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "zero client id", args: map[string]any{keyClientID: 0, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "unsafe large client id", args: map[string]any{keyClientID: 9007199254740992.0, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "slash client id", args: map[string]any{keyClientID: "789/1", keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "query client id", args: map[string]any{keyClientID: "789?x=1", keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "traversal client id", args: map[string]any{keyClientID: pathTraversalValue, keyLabel: longviewClientUpdatedLabel, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: caseMissingLabel, args: map[string]any{keyClientID: 789, keyConfirm: true}, want: errLabelRequired},
			{name: "short label", args: map[string]any{keyClientID: 789, keyLabel: "ab", keyConfirm: true}, want: errLongviewClientLabel},
			{name: "slash label", args: map[string]any{keyClientID: 789, keyLabel: "bad/label", keyConfirm: true}, want: errLongviewClientLabel},
			{name: "long label", args: map[string]any{keyClientID: 789, keyLabel: "abcdefghijklmnopqrstuvwxyzabcdefg", keyConfirm: true}, want: errLongviewClientLabel},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientUpdateTool(cfg)
				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})
}

func TestLinodeLongviewClientDeleteTool(t *testing.T) {
	t.Parallel()

	const errLongviewClientIDPositive = "client_id must be a positive integer"

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}

		tool, capability, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)
		assert.Equal(t, "linode_longview_client_delete", tool.Name, "tool name should match")
		assert.Equal(t, profiles.CapDestroy, capability, "delete should require destroy capability")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Contains(t, tool.InputSchema.Properties, keyClientID, "schema should include client_id")
		assert.Contains(t, tool.InputSchema.Properties, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyClientID, "client_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should match")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Longview client deleted successfully", "response should confirm deletion")
		assert.Contains(t, textContent.Text, "789", "response should contain deleted client ID")
	})

	t.Run("api error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			assert.Equal(t, "/longview/clients/789", r.URL.Path, "request path should match")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{keyErrors: []map[string]string{{keyReason: errForbidden}}}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyClientID: 789, keyConfirm: true})
		result, err := handler(t.Context(), req)
		require.NoError(t, err, "handler should return API failures as tool errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failure should be an error result")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to delete linode_longview_client_delete", "response should identify failed tool")
		assert.Contains(t, textContent.Text, errForbidden, "response should include API error detail")
	})

	t.Run("confirm rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: caseMissing, set: false},
			{name: caseFalse, value: false, set: true},
			{name: caseString, value: boolStringTrue, set: true},
			{name: caseNumeric, value: 1, set: true},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)

				args := map[string]any{keyClientID: 789}
				if testCase.set {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "missing or invalid confirm should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "response should require confirm=true")
			})
		}
	})

	t.Run("validation rejects before client", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{name: "missing client id", args: map[string]any{keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "zero client id", args: map[string]any{keyClientID: 0, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "unsafe large client id", args: map[string]any{keyClientID: 9007199254740992.0, keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "slash client id", args: map[string]any{keyClientID: "789/1", keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "query client id", args: map[string]any{keyClientID: "789?x=1", keyConfirm: true}, want: errLongviewClientIDPositive},
			{name: "traversal client id", args: map[string]any{keyClientID: pathTraversalValue, keyConfirm: true}, want: errLongviewClientIDPositive},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				cfg := &config.Config{}
				_, _, handler := tools.NewLinodeLongviewClientDeleteTool(cfg)
				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))
				require.NoError(t, err, "handler should return validation as a tool error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid input should be an error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.want, "response should describe validation error")
			})
		}
	})
}
