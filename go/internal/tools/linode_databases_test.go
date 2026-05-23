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
	databaseInstanceIDParam          = "instance_id"
	databaseInstanceIDMessage        = "instance_id must be a positive integer"
	databaseInstancePath             = "/databases/mysql/instances/123"
	databaseInstanceLabel            = "primary-db"
	databaseInstanceType             = typeG6Standard2
	databaseEngineParam              = "engine"
	databaseInvalidInstanceIDQuery   = "123?x=1"
	databaseAllowListParam           = "allow_list"
	databaseEngineConfigParam        = "engine_config"
	databasePrivateNetworkParam      = "private_network"
	databaseUpdatesParam             = "updates"
	databaseVersionParam             = "version"
	databaseInvalidAllowListJSON     = "invalid allow_list JSON"
	databaseInvalidEngineConfigJSON  = "invalid engine_config JSON"
	databaseJSONNull                 = "null"
	databaseJSONArray                = "[]"
	caseFalseConfirm                 = "false confirm"
	caseStringConfirm                = "string confirm"
	caseNumericConfirm               = "numeric confirm"
	invalidJSON                      = "not-json"
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
						keyType:            "integer",
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

		instances := []linode.DatabaseInstance{{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}}

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

func TestLinodeDatabaseInstanceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

		assert.Equal(t, "linode_database_instance_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapRead, capability, "tool should be read capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, databaseInstanceIDParam, "schema should include instance_id")
		assert.NotContains(t, props, keyConfirm, "read-only get tool must not require confirm")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})
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
			assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to retrieve MySQL Managed Database instance")
	})

	t.Run("client configuration error", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "configuration errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "missing client config should return an error result")
	})

	t.Run("instance id validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceGetTool(cfg)

		cases := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingInstanceID, args: map[string]any{}},
			{name: "string instance id", args: map[string]any{databaseInstanceIDParam: "123"}},
			{name: "zero instance id", args: map[string]any{databaseInstanceIDParam: 0}},
			{name: "negative instance id", args: map[string]any{databaseInstanceIDParam: -1}},
			{name: "fractional instance id", args: map[string]any{databaseInstanceIDParam: 123.4}},
			{name: "slash instance id", args: map[string]any{databaseInstanceIDParam: "/"}},
			{name: "query instance id", args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery}},
			{name: "traversal instance id", args: map[string]any{databaseInstanceIDParam: pathTraversalValue}},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "validation errors should be returned as tool result errors")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "invalid instance_id should return an error result")

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, databaseInstanceIDMessage)
			})
		}
	})
}

func TestLinodeDatabaseInstanceCreateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

		assert.Equal(t, "linode_database_instance_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyType, "schema should include type")
		assert.Contains(t, props, databaseEngineParam, "schema should include engine")
		assert.Contains(t, props, keyRegion, "schema should include region")
	})

	t.Run("confirm validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "https://example.invalid", Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

		cases := []struct {
			name  string
			value any
		}{
			{name: "missing confirm"},
			{name: caseFalseConfirm, value: false},
			{name: caseStringConfirm, value: boolStringTrue},
			{name: caseNumericConfirm, value: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast}
				if testCase.value != nil {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm=true")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			assert.Equal(t, databaseInstancesPath, r.URL.Path, "request path should be /databases/mysql/instances")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, databaseInstanceLabel, body[keyLabel])
			assert.Equal(t, databaseInstanceType, body[keyType])
			assert.Equal(t, databaseEngineID, body[databaseEngineParam])
			assert.Equal(t, regionUSEast, body[keyRegion])
			assert.Equal(t, true, body["ssl_connection"])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "ssl_connection": true, keyConfirm: true}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, databaseInstanceLabel)
		assert.Contains(t, textContent.Text, "created")
	})

	t.Run("required field validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingLabel, args: map[string]any{keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "label must be a non-empty string"},
			{name: "missing type", args: map[string]any{keyLabel: databaseInstanceLabel, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "type must be a non-empty string"},
			{name: "missing engine", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, keyRegion: regionUSEast, keyConfirm: true}, wantMessage: "engine must be a non-empty string"},
			{name: "missing region", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyConfirm: true}, wantMessage: "region must be a non-empty string"},
			{name: "invalid allow list", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseInvalidAllowListJSON},
			{name: "invalid cluster size", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "cluster_size": "3", keyConfirm: true}, wantMessage: "cluster_size must be a positive integer"},
			{name: "invalid engine config", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseInvalidEngineConfigJSON},
			{name: "invalid fork", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "fork": invalidJSON, keyConfirm: true}, wantMessage: "invalid fork JSON"},
			{name: "invalid private network bool", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, databasePrivateNetworkParam: boolStringTrue, keyConfirm: true}, wantMessage: "private_network must be a boolean"},
			{name: "invalid ssl bool", args: map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, "ssl_connection": boolStringTrue, keyConfirm: true}, wantMessage: "ssl_connection must be a boolean"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
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
		_, _, handler := tools.NewLinodeDatabaseInstanceCreateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyLabel: databaseInstanceLabel, keyType: databaseInstanceType, databaseEngineParam: databaseEngineID, keyRegion: regionUSEast, keyConfirm: true}))

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to create Managed Database instance")
	})
}

func TestLinodeDatabaseInstanceUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		assert.Equal(t, "linode_database_instance_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapWrite, capability, "tool should be write capability")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, databaseInstanceIDParam, "schema should include instance_id")
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, tool.InputSchema.Required, databaseInstanceIDParam, "instance_id must be marked required")
		assert.Contains(t, tool.InputSchema.Required, keyConfirm, "confirm must be marked required")
		assert.Contains(t, props, keyLabel, "schema should include label")
		assert.Contains(t, props, keyType, "schema should include type")
		assert.Contains(t, props, databaseUpdatesParam, "schema should include updates")
		assert.Contains(t, props, databaseVersionParam, "schema should include version")
	})

	t.Run("confirm validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "https://example.invalid", Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		cases := []struct {
			name  string
			value any
		}{
			{name: "missing confirm"},
			{name: caseFalseConfirm, value: false},
			{name: caseStringConfirm, value: boolStringTrue},
			{name: caseNumericConfirm, value: 1},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				args := map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel}
				if testCase.value != nil {
					args[keyConfirm] = testCase.value
				}

				result, err := handler(t.Context(), createRequestWithArgs(t, args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, "confirm=true")
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
			assert.Empty(t, r.URL.RawQuery, "request query should be empty")
			assert.Equal(t, "Bearer "+tokenTest, r.Header.Get("Authorization"))

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, databaseInstanceLabel, body[keyLabel])
			assert.Equal(t, databaseInstanceType, body[keyType])
			assert.Equal(t, databaseVersion, body[databaseVersionParam])
			assert.Equal(t, []any{"203.0.113.0/24"}, body[databaseAllowListParam])
			assert.Equal(t, map[string]any{"frequency": "weekly", "hour_of_day": float64(1)}, body[databaseUpdatesParam])
			assert.Equal(t, map[string]any{"public_access": false, "vpc_id": float64(123)}, body[databasePrivateNetworkParam])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.DatabaseInstance{ID: databaseInstanceID, Label: databaseInstanceLabel, Region: regionUSEast, Type: databaseInstanceType, Engine: databaseEngineName, Version: databaseVersion, Status: statusActive}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			databaseInstanceIDParam:     databaseInstanceID,
			keyLabel:                    databaseInstanceLabel,
			keyType:                     databaseInstanceType,
			databaseVersionParam:        databaseVersion,
			databaseAllowListParam:      `["203.0.113.0/24"]`,
			databaseUpdatesParam:        `{"frequency":"weekly","hour_of_day":1}`,
			databasePrivateNetworkParam: `{"public_access":false,"vpc_id":123}`,
			databaseEngineConfigParam:   `{"binlog_retention_period":600}`,
			keyConfirm:                  true,
		}))

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, databaseInstanceLabel)
		assert.Contains(t, textContent.Text, "updated")
	})

	t.Run("input validation", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		cases := []struct {
			name        string
			args        map[string]any
			wantMessage string
		}{
			{name: caseMissingInstanceID, args: map[string]any{keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
			{name: "query instance id", args: map[string]any{databaseInstanceIDParam: databaseInvalidInstanceIDQuery, keyLabel: databaseInstanceLabel, keyConfirm: true}, wantMessage: databaseInstanceIDMessage},
			{name: "empty update", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyConfirm: true}, wantMessage: "at least one update field must be provided"},
			{name: caseMissingLabel, args: map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: "", keyConfirm: true}, wantMessage: "label must be a non-empty string"},
			{name: "invalid allow list", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: invalidJSON, keyConfirm: true}, wantMessage: databaseInvalidAllowListJSON},
			{name: "invalid engine config", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: invalidJSON, keyConfirm: true}, wantMessage: databaseInvalidEngineConfigJSON},
			{name: "invalid private network", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: invalidJSON, keyConfirm: true}, wantMessage: "invalid private_network JSON"},
			{name: "invalid updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: invalidJSON, keyConfirm: true}, wantMessage: "invalid updates JSON"},
			{name: "numeric version", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseVersionParam: 8, keyConfirm: true}, wantMessage: "version must be a non-empty string"},
			{name: "null allow list", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: databaseJSONNull, keyConfirm: true}, wantMessage: "allow_list must be a JSON array"},
			{name: "object allow list", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseAllowListParam: "{}", keyConfirm: true}, wantMessage: databaseInvalidAllowListJSON},
			{name: "null engine config", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: databaseJSONNull, keyConfirm: true}, wantMessage: "engine_config must be a JSON object"},
			{name: "array engine config", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseEngineConfigParam: databaseJSONArray, keyConfirm: true}, wantMessage: databaseInvalidEngineConfigJSON},
			{name: "null private network", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: databaseJSONNull, keyConfirm: true}, wantMessage: "private_network must be a JSON object"},
			{name: "array private network", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databasePrivateNetworkParam: databaseJSONArray, keyConfirm: true}, wantMessage: "invalid private_network JSON"},
			{name: "null updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: databaseJSONNull, keyConfirm: true}, wantMessage: "updates must be a JSON object"},
			{name: "array updates", args: map[string]any{databaseInstanceIDParam: databaseInstanceID, databaseUpdatesParam: databaseJSONArray, keyConfirm: true}, wantMessage: "invalid updates JSON"},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				result, err := handler(t.Context(), createRequestWithArgs(t, testCase.args))

				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.wantMessage)
			})
		}
	})

	t.Run("client error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, databaseInstancePath, r.URL.Path, "request path should include instance id")
			w.WriteHeader(http.StatusInternalServerError)
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyErrors: []map[string]string{{keyReason: temporaryFailure}},
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}}}}
		_, _, handler := tools.NewLinodeDatabaseInstanceUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{databaseInstanceIDParam: databaseInstanceID, keyLabel: databaseInstanceLabel, keyConfirm: true}))

		require.NoError(t, err, "client errors should be returned as tool result errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "API failures should return an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to update Managed Database instance")
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
