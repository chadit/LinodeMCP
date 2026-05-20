package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chadit/LinodeMCP/internal/appinfo"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/tools"
)

// End-to-end verification of the hello tool.
func TestHelloTool(t *testing.T) {
	t.Parallel()

	tool, _, handler := tools.NewHelloTool(nil)

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

		req := createRequestWithArgs(t, map[string]any{keyName: "Alice"})
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

	tool, _, handler := tools.NewVersionTool(nil)

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
		tool, _, handler := tools.NewLinodeInstanceListTool(cfg)

		assert.Equal(t, "linode_instance_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

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
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: "", Token: ""},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "tool errors are returned as error results, not Go errors")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "should return an error result for incomplete config")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		instances := []linode.Instance{
			{ID: 1, Label: "web-1", Status: statusRunning},
			{ID: 2, Label: "db-1", Status: "stopped"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    instances,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceListTool(cfg)

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
		tool, _, handler := tools.NewLinodeProfileTool(cfg)

		assert.Equal(t, "linode_profile", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

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
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeProfileTool(cfg)

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
		tool, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		assert.Equal(t, "linode_instance_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("missing instance ID", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

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
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: notANumber})
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
			Status: statusRunning,
			Region: regionUSEast,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/instances/123", r.URL.Path, "request path should include instance ID")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(instance))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeInstanceGetTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyInstanceID: "123"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "test-instance", "response should contain instance label")
		assert.Contains(t, textContent.Text, statusRunning, "response should contain instance status")
	})
}

// End-to-end verification of account info retrieval.
func TestLinodeAccountTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeAccountTool(cfg)

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
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountTool(cfg)

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
		tool, _, handler := tools.NewLinodeRegionListTool(cfg)

		assert.Equal(t, "linode_region_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: regionUSEast, Label: "Newark, NJ", Country: countryUS, Capabilities: []string{"Linodes", "Block Storage"}, Status: statusOK},
			{ID: regionEUWest, Label: "London, UK", Country: "uk", Capabilities: []string{"Linodes"}, Status: statusOK},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/regions", r.URL.Path, "request path should be /regions")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    regions,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeRegionListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain us-east region")
		assert.Contains(t, textContent.Text, regionEUWest, "response should contain eu-west region")
	})

	t.Run("filter by country", func(t *testing.T) {
		t.Parallel()

		regions := []linode.Region{
			{ID: regionUSEast, Label: "Newark, NJ", Country: countryUS, Status: statusOK},
			{ID: regionUSWest, Label: "Fremont, CA", Country: countryUS, Status: statusOK},
			{ID: regionEUWest, Label: "London, UK", Country: "uk", Status: statusOK},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    regions,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeRegionListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"country": countryUS})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, regionUSEast, "response should contain us-east")
		assert.Contains(t, textContent.Text, regionUSWest, "response should contain us-west")
		assert.NotContains(t, textContent.Text, regionEUWest, "response should not contain eu-west")
	})
}

// End-to-end verification of type listing and filtering.
func TestLinodeTypesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeTypeListTool(cfg)

		assert.Equal(t, "linode_type_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: typeG6Nanode1, Label: "Nanode 1GB", Class: "nanode", Disk: 25600, Memory: 1024, VCPUs: 1},
			{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard, Disk: 81920, Memory: 4096, VCPUs: 2},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/linode/types", r.URL.Path, "request path should be /linode/types")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeTypeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, typeG6Nanode1, "response should contain nanode type")
		assert.Contains(t, textContent.Text, typeG6Standard2, "response should contain standard type")
	})

	t.Run("filter by class", func(t *testing.T) {
		t.Parallel()

		types := []linode.InstanceType{
			{ID: typeG6Nanode1, Label: "Nanode 1GB", Class: "nanode"},
			{ID: typeG6Standard2, Label: typeLinode4GB, Class: classStandard},
			{ID: "g6-standard-4", Label: "Linode 8GB", Class: classStandard},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeTypeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"class": classStandard})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.NotContains(t, textContent.Text, typeG6Nanode1, "response should not contain nanode type")
	})
}

// End-to-end verification of volume listing and filtering.
func TestLinodeVolumesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeVolumeListTool(cfg)

		assert.Equal(t, "linode_volume_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Status: statusActive, Size: 100, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Status: statusActive, Size: 50, Region: regionEUWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/volumes", r.URL.Path, "request path should be /volumes")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, labelDataVol, "response should contain first volume label")
		assert.Contains(t, textContent.Text, labelBackupVol, "response should contain second volume label")
	})

	t.Run("filter by region", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Region: regionEUWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, labelDataVol, "response should contain matching volume")
		assert.NotContains(t, textContent.Text, labelBackupVol, "response should not contain non-matching volume")
	})

	t.Run("filter by label", func(t *testing.T) {
		t.Parallel()

		volumes := []linode.Volume{
			{ID: 1, Label: labelDataVol, Region: regionUSEast},
			{ID: 2, Label: labelBackupVol, Region: regionEUWest},
			{ID: 3, Label: "data-backup", Region: regionUSWest},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    volumes,
				keyPage:    1,
				keyPages:   1,
				keyResults: 3,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeVolumeListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"label_contains": "backup"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 2`, "filtered count should be 2")
		assert.Contains(t, textContent.Text, labelBackupVol, "response should contain backup-vol")
		assert.Contains(t, textContent.Text, "data-backup", "response should contain data-backup")
	})
}

// End-to-end verification of image listing and filtering.
func TestLinodeImagesListTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, _, handler := tools.NewLinodeImageListTool(cfg)

		assert.Equal(t, "linode_image_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Type: "manual", IsPublic: true, Deprecated: false},
			{ID: "private/12345", Label: "Custom Image", Type: "manual", IsPublic: false, Deprecated: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/images", r.URL.Path, "request path should be /images")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, imageIDUbuntu2204, "response should contain public image")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
	})

	t.Run("filter by public", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, IsPublic: true},
			{ID: "private/12345", Label: "Custom Image", IsPublic: false},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"is_public": "false"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "private/12345", "response should contain private image")
		assert.NotContains(t, textContent.Text, imageIDUbuntu2204, "response should not contain public image")
	})

	t.Run("filter by deprecated", func(t *testing.T) {
		t.Parallel()

		images := []linode.Image{
			{ID: imageIDUbuntu2204, Label: imageUbuntu2204, Deprecated: false},
			{ID: "linode/ubuntu18.04", Label: "Ubuntu 18.04", Deprecated: true},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    images,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeImageListTool(cfg)

		req := createRequestWithArgs(t, map[string]any{"deprecated": "true"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"count": 1`, "filtered count should be 1")
		assert.Contains(t, textContent.Text, "linode/ubuntu18.04", "response should contain deprecated image")
		assert.NotContains(t, textContent.Text, imageIDUbuntu2204, "response should not contain non-deprecated image")
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

// End-to-end verification of account update.
func TestLinodeAccountUpdateTool(t *testing.T) {
	t.Parallel()

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{}
		tool, capability, handler := tools.NewLinodeAccountUpdateTool(cfg)

		assert.Equal(t, "linode_account_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		assert.Equal(t, profiles.CapAdmin, capability, "account updates should be CapAdmin")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, keyConfirm, "schema should include confirm")
		assert.Contains(t, props, "email", "schema should include editable account fields")
	})

	t.Run("confirm required before client call", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			value any
			set   bool
		}{
			{name: "missing", set: false},
			{name: "confirm false", value: false, set: true},
			{name: "string", value: boolStringTrue, set: true},
			{name: "numeric", value: 1, set: true},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				var calls int32

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					atomic.AddInt32(&calls, 1)

					w.WriteHeader(http.StatusNoContent)
				}))
				defer srv.Close()

				cfg := &config.Config{
					Environments: map[string]config.EnvironmentConfig{
						envKeyDefault: {
							Label:  envLabelDefault,
							Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
						},
					},
				}
				_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

				args := map[string]any{keyEmail: emailUpdatedExample}
				if tt.set {
					args[keyConfirm] = tt.value
				}

				req := createRequestWithArgs(t, args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return transport error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be a tool error")
				assertErrorContains(t, result, errConfirmEqualsTrue)
				assert.Equal(t, int32(0), calls, "confirm failure must happen before client call")
			})
		}
	})

	t.Run("empty update rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "at least one account field is required")
		assert.Equal(t, int32(0), calls, "empty update must fail before client call")
	})

	t.Run("malformed field rejected before client call", func(t *testing.T) {
		t.Parallel()

		var calls int32

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			atomic.AddInt32(&calls, 1)

			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: 123, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "email must be a string")
		assert.Equal(t, int32(0), calls, "malformed field must fail before client call")
	})

	t.Run("api error produces tool error", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")

			w.WriteHeader(http.StatusBadRequest)
			_, err := w.Write([]byte(`{"errors":[{"field":"email","reason":"invalid email format"}]}`))
			assert.NoError(t, err)
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return transport error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be a tool error")
		assertErrorContains(t, result, "Failed to update account")
		assertErrorContains(t, result, "invalid email format")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		account := linode.Account{FirstName: nameUpdatedTest, LastName: "User", Email: emailUpdatedExample}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			assert.Equal(t, "/account", r.URL.Path, "request path should be /account")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			assert.Equal(t, emailUpdatedExample, body["email"])
			assert.Equal(t, nameUpdatedTest, body["first_name"])

			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(account))
		}))
		defer srv.Close()

		cfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {
					Label:  envLabelDefault,
					Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest},
				},
			},
		}
		_, _, handler := tools.NewLinodeAccountUpdateTool(cfg)

		req := createRequestWithArgs(t, map[string]any{keyEmail: emailUpdatedExample, "first_name": nameUpdatedTest, keyConfirm: true})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "should not be an error result")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Account updated successfully", "response should contain success message")
		assert.Contains(t, textContent.Text, emailUpdatedExample, "response should contain updated email")
	})
}
