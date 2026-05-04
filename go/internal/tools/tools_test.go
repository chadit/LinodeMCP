package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// End-to-end verification of the hello tool.
func TestHelloTool(t *testing.T) {
	t.Parallel()

	tool, handler := tools.NewHelloTool()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "hello", tool.Name, "tool name should be hello")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("default name", func(t *testing.T) {
		t.Parallel()

		req := mcp.CallToolRequest{}
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "World", "default greeting should contain World")
		assert.Contains(t, textContent.Text, "LinodeMCP", "greeting should mention LinodeMCP")
	})

	t.Run("custom name", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{"name": "Alice"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Alice", "greeting should contain the provided name")
	})
}

// End-to-end verification of the version tool.
func TestVersionTool(t *testing.T) {
	t.Parallel()

	tool, handler := tools.NewVersionTool()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "version", tool.Name, "tool name should be version")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		result, err := handler(t.Context(), mcp.CallToolRequest{})

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		require.NotEmpty(t, result.Content, "result should have content")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")

		var info appinfo.Info

		err = json.Unmarshal([]byte(textContent.Text), &info)
		require.NoError(t, err, "version response should be valid JSON")
		assert.Equal(t, appinfo.Version, info.Version, "version should match appinfo.Version")
	})
}

// End-to-end verification of the instance listing workflow.
func TestLinodeInstancesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeInstancesTool(cfg)

		assert.Equal(t, "linode_instances_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, handler := tools.NewLinodeInstancesTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for missing environment")
	})

	t.Run("incomplete config", func(t *testing.T) {
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

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for incomplete config")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instances := []linode.Instance{
			{ID: 1, Label: "web-1", Status: "running"},
			{ID: 2, Label: "db-1", Status: "stopped"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "web-1", "response should contain first instance label")
		assert.Contains(t, textContent.Text, "db-1", "response should contain second instance label")
	})
}

// End-to-end verification of the profile tool.
func TestLinodeProfileTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeProfileTool(cfg)

		assert.Equal(t, "linode_profile", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("incomplete config", func(t *testing.T) {
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

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for incomplete config")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		profile := linode.Profile{
			Username: "testuser",
			Email:    "test@example.com",
			UID:      42,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(profile))
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "testuser", "response should contain the username")
	})
}

// End-to-end verification of the instance get workflow.
func TestLinodeInstanceGetTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeInstanceGetTool(cfg)

		assert.Equal(t, "linode_instance_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing instance ID", func(t *testing.T) {
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

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for missing instance ID")
	})

	t.Run("invalid instance ID", func(t *testing.T) {
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

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for invalid instance ID")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instance := linode.Instance{
			ID:     123,
			Label:  "test-instance",
			Status: "running",
			Region: "us-east",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123", r.URL.Path, "request path should include instance ID")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance))
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "test-instance", "response should contain instance label")
		assert.Contains(t, textContent.Text, "running", "response should contain instance status")
	})
}

// End-to-end verification of account info retrieval.
func TestLinodeAccountTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeAccountTool(cfg)

		assert.Equal(t, "linode_account", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		account := linode.Account{
			FirstName: "Test",
			LastName:  "User",
			Email:     "test@example.com",
			Company:   "TestCo",
			Balance:   100.50,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(account))
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Test", "response should contain first name")
		assert.Contains(t, textContent.Text, "test@example.com", "response should contain email")
	})
}

// End-to-end verification of region listing and filtering.
func TestLinodeRegionsListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeRegionsListTool(cfg)

		assert.Equal(t, "linode_regions_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: "us-east", Label: "Newark, NJ", Country: "us", Capabilities: []string{"Linodes", "Block Storage"}, Status: "ok"},
			{ID: "eu-west", Label: "London, UK", Country: "uk", Capabilities: []string{"Linodes"}, Status: "ok"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/regions", r.URL.Path, "request path should be /regions")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "us-east", "response should contain us-east region")
		assert.Contains(t, textContent.Text, "eu-west", "response should contain eu-west region")
	})

	t.Run("filter by country", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: "us-east", Label: "Newark, NJ", Country: "us", Status: "ok"},
			{ID: "us-west", Label: "Fremont, CA", Country: "us", Status: "ok"},
			{ID: "eu-west", Label: "London, UK", Country: "uk", Status: "ok"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, "us-east", "response should contain us-east")
		assert.Contains(t, textContent.Text, "us-west", "response should contain us-west")
		assert.NotContains(t, textContent.Text, "eu-west", "response should not contain eu-west")
	})
}

// End-to-end verification of type listing and filtering.
func TestLinodeTypesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeTypesListTool(cfg)

		assert.Equal(t, "linode_types_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: "g6-nanode-1", Label: "Nanode 1GB", Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
			{ID: "g6-standard-2", Label: "Linode 4GB", Class: "standard", Disk: 81920, Memory: 4096, VCPUs: 2},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/types", r.URL.Path, "request path should be /linode/types")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "g6-nanode-1", "response should contain nanode type")
		assert.Contains(t, textContent.Text, "g6-standard-2", "response should contain standard type")
	})

	t.Run("filter by class", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: "g6-nanode-1", Label: "Nanode 1GB", Class: "nanode"},
			{ID: "g6-standard-2", Label: "Linode 4GB", Class: "standard"},
			{ID: "g6-standard-4", Label: "Linode 8GB", Class: "standard"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.NotContains(t, textContent.Text, "g6-nanode-1", "response should not contain nanode type")
	})
}

// End-to-end verification of volume listing and filtering.
func TestLinodeVolumesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeVolumesListTool(cfg)

		assert.Equal(t, "linode_volumes_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: "data-vol", Status: "active", Size: 100, Region: "us-east"},
			{ID: 2, Label: "backup-vol", Status: "active", Size: 50, Region: "eu-west"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes", r.URL.Path, "request path should be /volumes")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "data-vol", "response should contain first volume label")
		assert.Contains(t, textContent.Text, "backup-vol", "response should contain second volume label")
	})

	t.Run("filter by region", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: "data-vol", Region: "us-east"},
			{ID: 2, Label: "backup-vol", Region: "eu-west"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "data-vol", "response should contain matching volume")
		assert.NotContains(t, textContent.Text, "backup-vol", "response should not contain non-matching volume")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: "data-vol", Region: "us-east"},
			{ID: 2, Label: "backup-vol", Region: "eu-west"},
			{ID: 3, Label: "data-backup", Region: "us-west"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, "backup-vol", "response should contain backup-vol")
		assert.Contains(t, textContent.Text, "data-backup", "response should contain data-backup")
	})
}

// End-to-end verification of image listing and filtering.
func TestLinodeImagesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, handler := tools.NewLinodeImagesListTool(cfg)

		assert.Equal(t, "linode_images_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", Type: "manual", IsPublic: true, Deprecated: false},
			{ID: "private/12345", Label: "Custom Image", Type: "manual", IsPublic: false, Deprecated: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/images", r.URL.Path, "request path should be /images")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "linode/ubuntu22.04", "response should contain public image")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
	})

	t.Run("filter by public", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", IsPublic: true},
			{ID: "private/12345", Label: "Custom Image", IsPublic: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
		assert.NotContains(t, textContent.Text, "linode/ubuntu22.04", "response should not contain public image")
	})

	t.Run("filter by deprecated", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: "linode/ubuntu22.04", Label: "Ubuntu 22.04", Deprecated: false},
			{ID: "linode/ubuntu18.04", Label: "Ubuntu 18.04", Deprecated: true},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
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

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "linode/ubuntu18.04", "response should contain deprecated image")
		assert.NotContains(t, textContent.Text, "linode/ubuntu22.04", "response should not contain non-deprecated image")
	})
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
