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
	temporaryFailure = "temporary failure"

	databaseEnginesPath              = "/databases/engines"
	databaseEngineEscapedPath        = "/databases/engines/mysql%2F8.0.26"
	databaseEngineID                 = "mysql/8.0.26"
	databaseEngineIDParam            = "engine_id"
	databaseEngineIDRequiredMessage  = "engine_id must be a non-empty string"
	databaseEngineIDShapeMessage     = "engine_id must use the engine/version format"
	databaseEngineIDSeparatorMessage = "engine_id must not contain query, fragment, or traversal segments"
	databaseEngineName               = "mysql"
	databaseVersion                  = "8.0.26"
	databaseInstancesPath            = "/databases/mysql/instances"
	databaseMySQLConfigPath          = "/databases/mysql/config"
	databaseInstanceID               = 123
	databaseInstanceLabel            = "primary-db"
)

func TestLinodeDatabaseEngineListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

		assert.Equal(t, "linode_database_engine_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		engines := []linode.DatabaseEngine{{ID: databaseEngineID, Engine: databaseEngineName, Version: databaseVersion}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    engines,
				keyPage:    2,
				keyPages:   3,
				keyResults: 51,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, databaseEngineID)
		assert.Contains(t, textContent.Text, databaseEngineName)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, databaseEnginesPath, r.URL.Path, "request path should be /databases/engines")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve Managed Database engines")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "missing client config should return an error result")
	})

	t.Run("pagination validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseEngineListTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation errors should be returned as tool result errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should return an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})
}

func TestLinodeDatabaseMySQLConfigGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

		assert.Equal(t, "linode_database_mysql_config_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"mysql": map[string]any{
					"connect_timeout": map[string]any{
						"description":      "The number of seconds that the mysqld server waits for a connect packet.",
						"example":          10,
						"requires_restart": false,
						"type":             "integer",
					},
				},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "connect_timeout")
		assert.Contains(t, textContent.Text, "requires_restart")
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, databaseMySQLConfigPath, r.URL.Path, "request path should be /databases/mysql/config")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve MySQL Managed Database advanced parameters")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseMySQLConfigGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "missing client config should return an error result")
	})
}

func TestLinodeDatabaseInstanceListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

		assert.Equal(t, "linode_database_instance_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyPage, "schema should include page")
		assert.Contains(t, props, keyPageSize, "schema should include page_size")
		assert.NotContains(t, props, keyConfirm, "read-only list tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: "g6-dedicated-2", Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
			assert.Equal(t, "page=2&page_size=25", r.URL.RawQuery, "request query should include pagination")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    instances,
				keyPage:    2,
				keyPages:   3,
				keyResults: 51,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyPage: 2, keyPageSize: 25})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, databaseInstanceLabel)
		assert.Contains(t, textContent.Text, databaseEngineName)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve Managed Database instances")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "missing client config should return an error result")
	})

	t.Run("pagination validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceListTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: paginationCasePageZero, args: map[string]any{keyPage: 0}, wantMessage: paginationMessagePageMustBe},
			{name: paginationCasePageString, args: map[string]any{keyPage: "2"}, wantMessage: errPageInteger},
			{name: paginationCasePageSizeTooSmall, args: map[string]any{keyPageSize: 24}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeTooLarge, args: map[string]any{keyPageSize: 501}, wantMessage: errPageSizeRange},
			{name: paginationCasePageSizeString, args: map[string]any{keyPageSize: "25"}, wantMessage: errPageSizeInteger},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation errors should be returned as tool result errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid pagination should return an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})
}

func TestLinodeDatabaseEngineGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

		assert.Equal(t, "linode_database_engine_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, databaseEngineIDParam, "schema should include engine_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseEngine{ID: databaseEngineID, Engine: databaseEngineName, Version: databaseVersion}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, databaseEngineID)
		assert.Contains(t, textContent.Text, databaseEngineName)
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, databaseEngineEscapedPath, r.URL.EscapedPath(), "request path should escape engine id")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve Managed Database engine")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseEngineIDParam: databaseEngineID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "missing client config should return an error result")
	})

	t.Run("engine id validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseEngineGetTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: "missing engine id", args: map[string]any{}, wantMessage: databaseEngineIDRequiredMessage},
			{name: "numeric engine id", args: map[string]any{databaseEngineIDParam: 123}, wantMessage: databaseEngineIDRequiredMessage},
			{name: "blank engine id", args: map[string]any{databaseEngineIDParam: ""}, wantMessage: databaseEngineIDRequiredMessage},
			{name: "query engine id", args: map[string]any{databaseEngineIDParam: "mysql?version=8"}, wantMessage: databaseEngineIDSeparatorMessage},
			{name: "fragment engine id", args: map[string]any{databaseEngineIDParam: "mysql#8.0.26"}, wantMessage: databaseEngineIDSeparatorMessage},
			{name: "traversal engine id", args: map[string]any{databaseEngineIDParam: "mysql/.."}, wantMessage: databaseEngineIDSeparatorMessage},
			{name: "leading slash engine id", args: map[string]any{databaseEngineIDParam: "/mysql/8.0.26"}, wantMessage: databaseEngineIDShapeMessage},
			{name: "trailing slash engine id", args: map[string]any{databaseEngineIDParam: "mysql/"}, wantMessage: databaseEngineIDShapeMessage},
			{name: "repeated slash engine id", args: map[string]any{databaseEngineIDParam: "mysql//8.0.26"}, wantMessage: databaseEngineIDShapeMessage},
			{name: "extra segment engine id", args: map[string]any{databaseEngineIDParam: "mysql/8.0.26/extra"}, wantMessage: databaseEngineIDShapeMessage},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation errors should be returned as tool result errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid engine_id should return an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})
}
