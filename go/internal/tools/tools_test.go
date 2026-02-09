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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
}

func TestNewLinodeProfileTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeProfileTool(cfg)

	assert.Equal(t, "linode_profile", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
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

// Stage 2 Tool Tests.

func TestNewLinodeInstanceGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeInstanceGetTool(cfg)

	assert.Equal(t, "linode_instance_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeInstanceGetTool_MissingInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeInstanceGetTool_InvalidInstanceID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": "not-a-number"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeInstanceGetTool_Success(t *testing.T) {
	t.Parallel()

	instance := linode.Instance{
		ID:     123,
		Label:  "test-instance",
		Status: "running",
		Region: "us-east",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/instances/123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(instance))
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
	_, handler := tools.NewLinodeInstanceGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"instance_id": "123"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "test-instance")
	assert.Contains(t, textContent.Text, "running")
}

func TestNewLinodeAccountTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeAccountTool(cfg)

	assert.Equal(t, "linode_account", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeAccountTool_Success(t *testing.T) {
	t.Parallel()

	account := linode.Account{
		FirstName: "Test",
		LastName:  "User",
		Email:     "test@example.com",
		Company:   "TestCo",
		Balance:   100.50,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/account", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(account))
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
	_, handler := tools.NewLinodeAccountTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "Test")
	assert.Contains(t, textContent.Text, "test@example.com")
}

func TestNewLinodeRegionsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeRegionsListTool(cfg)

	assert.Equal(t, "linode_regions_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeRegionsListTool_Success(t *testing.T) {
	t.Parallel()

	regions := []linode.Region{
		{ID: "us-east", Label: "Newark, NJ", Country: "us", Capabilities: []string{"Linodes", "Block Storage"}, Status: "ok"},
		{ID: "eu-west", Label: "London, UK", Country: "uk", Capabilities: []string{"Linodes"}, Status: "ok"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/regions", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    regions,
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
	_, handler := tools.NewLinodeRegionsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "us-east")
	assert.Contains(t, textContent.Text, "eu-west")
}

func TestLinodeRegionsListTool_FilterByCountry(t *testing.T) {
	t.Parallel()

	regions := []linode.Region{
		{ID: "us-east", Label: "Newark, NJ", Country: "us", Status: "ok"},
		{ID: "us-west", Label: "Fremont, CA", Country: "us", Status: "ok"},
		{ID: "eu-west", Label: "London, UK", Country: "uk", Status: "ok"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    regions,
			"page":    1,
			"pages":   1,
			"results": 3,
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
	_, handler := tools.NewLinodeRegionsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"country": "us"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 2`)
	assert.Contains(t, textContent.Text, "us-east")
	assert.Contains(t, textContent.Text, "us-west")
	assert.NotContains(t, textContent.Text, "eu-west")
}

func TestNewLinodeTypesListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeTypesListTool(cfg)

	assert.Equal(t, "linode_types_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeTypesListTool_Success(t *testing.T) {
	t.Parallel()

	types := []linode.InstanceType{
		{ID: "g6-nanode-1", Label: "Nanode 1GB", Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
		{ID: "g6-standard-2", Label: "Linode 4GB", Class: "standard", Disk: 81920, Memory: 4096, VCPUs: 2},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/linode/types", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    types,
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
	_, handler := tools.NewLinodeTypesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "g6-nanode-1")
	assert.Contains(t, textContent.Text, "g6-standard-2")
}

func TestLinodeTypesListTool_FilterByClass(t *testing.T) {
	t.Parallel()

	types := []linode.InstanceType{
		{ID: "g6-nanode-1", Label: "Nanode 1GB", Class: "nanode"},
		{ID: "g6-standard-2", Label: "Linode 4GB", Class: "standard"},
		{ID: "g6-standard-4", Label: "Linode 8GB", Class: "standard"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    types,
			"page":    1,
			"pages":   1,
			"results": 3,
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
	_, handler := tools.NewLinodeTypesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"class": "standard"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 2`)
	assert.NotContains(t, textContent.Text, "g6-nanode-1")
}

func TestNewLinodeVolumesListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeVolumesListTool(cfg)

	assert.Equal(t, "linode_volumes_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeVolumesListTool_Success(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: "data-vol", Status: "active", Size: 100, Region: "us-east"},
		{ID: 2, Label: "backup-vol", Status: "active", Size: 50, Region: "eu-west"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/volumes", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    volumes,
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
	_, handler := tools.NewLinodeVolumesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "data-vol")
	assert.Contains(t, textContent.Text, "backup-vol")
}

func TestLinodeVolumesListTool_FilterByRegion(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: "data-vol", Region: "us-east"},
		{ID: 2, Label: "backup-vol", Region: "eu-west"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    volumes,
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
	_, handler := tools.NewLinodeVolumesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 1`)
	assert.Contains(t, textContent.Text, "data-vol")
	assert.NotContains(t, textContent.Text, "backup-vol")
}

func TestLinodeVolumesListTool_FilterByLabel(t *testing.T) {
	t.Parallel()

	volumes := []linode.Volume{
		{ID: 1, Label: "data-vol", Region: "us-east"},
		{ID: 2, Label: "backup-vol", Region: "eu-west"},
		{ID: 3, Label: "data-backup", Region: "us-west"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    volumes,
			"page":    1,
			"pages":   1,
			"results": 3,
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
	_, handler := tools.NewLinodeVolumesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label_contains": "backup"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 2`)
	assert.Contains(t, textContent.Text, "backup-vol")
	assert.Contains(t, textContent.Text, "data-backup")
}

func TestNewLinodeImagesListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeImagesListTool(cfg)

	assert.Equal(t, "linode_images_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeImagesListTool_Success(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", Type: "manual", IsPublic: true, Deprecated: false},
		{ID: "private/12345", Label: "Custom Image", Type: "manual", IsPublic: false, Deprecated: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/images", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    images,
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
	_, handler := tools.NewLinodeImagesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "linode/ubuntu22.04")
	assert.Contains(t, textContent.Text, "private/12345")
}

func TestLinodeImagesListTool_FilterByPublic(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", IsPublic: true},
		{ID: "private/12345", Label: "Custom Image", IsPublic: false},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    images,
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
	_, handler := tools.NewLinodeImagesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"is_public": "false"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 1`)
	assert.Contains(t, textContent.Text, "private/12345")
	assert.NotContains(t, textContent.Text, "linode/ubuntu22.04")
}

func TestLinodeImagesListTool_FilterByDeprecated(t *testing.T) {
	t.Parallel()

	images := []linode.Image{
		{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", Deprecated: false},
		{ID: "linode/ubuntu18.04", Label: "Ubuntu 18.04", Deprecated: true},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    images,
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
	_, handler := tools.NewLinodeImagesListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"deprecated": "true"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"count": 1`)
	assert.Contains(t, textContent.Text, "linode/ubuntu18.04")
	assert.NotContains(t, textContent.Text, "linode/ubuntu22.04")
}
