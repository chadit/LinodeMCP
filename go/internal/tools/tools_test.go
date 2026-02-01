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
	"github.com/chadit/LinodeMCP/internal/tools"
	"github.com/chadit/LinodeMCP/internal/version"
)

func TestNewHelloTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	tool, handler := tools.NewHelloTool()

	assert.Equal(t, "hello", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestHelloTool_DefaultName(t *testing.T) {
	t.Parallel()

	_, handler := tools.NewHelloTool()

	req := mcp.CallToolRequest{}
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)

	// The result should contain text content with "World" since no name was provided.
	require.NotEmpty(t, result.Content)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent type.")
	assert.Contains(t, textContent.Text, "World")
	assert.Contains(t, textContent.Text, "LinodeMCP")
}

func TestHelloTool_CustomName(t *testing.T) {
	t.Parallel()

	_, handler := tools.NewHelloTool()

	req := createRequestWithArgs(t, map[string]any{"name": "Alice"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Alice")
}

func TestNewVersionTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	tool, handler := tools.NewVersionTool()

	assert.Equal(t, "version", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestVersionTool_ReturnsVersionInfo(t *testing.T) {
	t.Parallel()

	_, handler := tools.NewVersionTool()

	result, err := handler(t.Context(), mcp.CallToolRequest{})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var info version.Info
	err = json.Unmarshal([]byte(textContent.Text), &info)
	require.NoError(t, err, "version response should be valid JSON.")
	assert.Equal(t, version.Version, info.Version)
}

func TestNewLinodeInstancesTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstancesTool(cfg)

	assert.Equal(t, "linode_instances_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestNewLinodeProfileTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeProfileTool(cfg)

	assert.Equal(t, "linode_profile", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeInstancesTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeInstancesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err, "tool errors are returned as error results, not Go errors.")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should return an error result.")
}

func TestLinodeInstancesTool_IncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "", Token: ""},
			},
		},
	}
	_, handler := tools.NewLinodeInstancesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeInstancesTool_SuccessfulList(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{
		{ID: 1, Label: "web-1", Status: "running"},
		{ID: 2, Label: "db-1", Status: "stopped"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    instances,
			"page":    1,
			"pages":   1,
			"results": 2,
		}))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstancesTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "should not be an error result.")

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "web-1")
	assert.Contains(t, textContent.Text, "db-1")
}

func TestLinodeProfileTool_IncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{},
			},
		},
	}
	_, handler := tools.NewLinodeProfileTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeProfileTool_Success(t *testing.T) {
	t.Parallel()

	profile := linode.Profile{
		Username: "testuser",
		Email:    "test@example.com",
		UID:      42,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(profile))
	}))
	defer srv.Close()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeProfileTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "testuser")
}

func TestSelectEnvironment_SpecificEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"prod":    {Label: "Production"},
			"staging": {Label: "Staging"},
		},
	}

	env, err := tools.SelectEnvironment(cfg, "prod")
	require.NoError(t, err)
	assert.Equal(t, "Production", env.Label)
}

func TestSelectEnvironment_NotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}

	_, err := tools.SelectEnvironment(cfg, "missing")
	require.Error(t, err)
	assert.ErrorIs(t, err, tools.ErrEnvironmentNotFound)
}

func TestValidateLinodeConfig_Complete(t *testing.T) {
	t.Parallel()

	env := &config.EnvironmentConfig{
		Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "tok"},
	}
	assert.NoError(t, tools.ValidateLinodeConfig(env))
}

func TestValidateLinodeConfig_MissingToken(t *testing.T) {
	t.Parallel()

	env := &config.EnvironmentConfig{
		Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4"},
	}
	assert.ErrorIs(t, tools.ValidateLinodeConfig(env), tools.ErrLinodeConfigIncomplete)
}

func TestValidateLinodeConfig_MissingURL(t *testing.T) {
	t.Parallel()

	env := &config.EnvironmentConfig{
		Linode: config.LinodeConfig{Token: "tok"},
	}
	assert.ErrorIs(t, tools.ValidateLinodeConfig(env), tools.ErrLinodeConfigIncomplete)
}

func TestFilterInstancesByStatus(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{
		{ID: 1, Status: "running"},
		{ID: 2, Status: "stopped"},
		{ID: 3, Status: "Running"},
	}

	filtered := tools.FilterInstancesByStatus(instances, "running")
	assert.Len(t, filtered, 2, "should match case-insensitively.")
}

func TestFilterInstancesByStatus_NoMatch(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{
		{ID: 1, Status: "running"},
	}

	filtered := tools.FilterInstancesByStatus(instances, "stopped")
	assert.Empty(t, filtered)
}

func TestFormatInstancesResponse_WithFilter(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{{ID: 1, Label: "test"}}
	result, err := tools.FormatInstancesResponse(instances, "running")

	require.NoError(t, err)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "status=running")
	assert.Contains(t, textContent.Text, `"count": 1`)
}

func TestFormatInstancesResponse_NoFilter(t *testing.T) {
	t.Parallel()

	instances := []linode.Instance{{ID: 1}}
	result, err := tools.FormatInstancesResponse(instances, "")

	require.NoError(t, err)
	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.NotContains(t, textContent.Text, "filter")
}

// createRequestWithArgs builds a CallToolRequest with the given arguments.
func createRequestWithArgs(t *testing.T, args map[string]any) mcp.CallToolRequest {
	t.Helper()

	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}
