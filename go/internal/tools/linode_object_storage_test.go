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
)

// Object Storage Buckets List tests.

func TestNewLinodeObjectStorageBucketsListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketsListTool(cfg)

	assert.Equal(t, "linode_object_storage_buckets_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeObjectStorageBucketsListTool_Success(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: "my-bucket", Region: "us-east-1", Hostname: "my-bucket.us-east-1.linodeobjects.com", Objects: 42, Size: 1024},
		{Label: "backups", Region: "us-southeast-1", Hostname: "backups.us-southeast-1.linodeobjects.com", Objects: 10, Size: 512},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    buckets,
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
	_, handler := tools.NewLinodeObjectStorageBucketsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-bucket")
	assert.Contains(t, textContent.Text, "backups")
	assert.Contains(t, textContent.Text, `"count": 2`)
}

func TestLinodeObjectStorageBucketsListTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageBucketsListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Object Storage Bucket Get tests.

func TestNewLinodeObjectStorageBucketGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeObjectStorageBucketGetTool_Success(t *testing.T) {
	t.Parallel()

	bucket := linode.ObjectStorageBucket{
		Label:    "my-bucket",
		Region:   "us-east-1",
		Hostname: "my-bucket.us-east-1.linodeobjects.com",
		Objects:  42,
		Size:     1024,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(bucket))
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
	_, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-bucket")
	assert.Contains(t, textContent.Text, "us-east-1")
}

func TestLinodeObjectStorageBucketGetTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageBucketGetTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Object Storage Bucket Contents tests.

func TestNewLinodeObjectStorageBucketContentsTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_contents", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeObjectStorageBucketContentsTool_Success(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "file1.txt", Size: 1024, LastModified: "2024-01-15T10:00:00Z"},
		{Name: "file2.jpg", Size: 2048, LastModified: "2024-01-16T10:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-list", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":         objects,
			"is_truncated": false,
			"next_marker":  "",
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
	_, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "file1.txt")
	assert.Contains(t, textContent.Text, "file2.jpg")
	assert.Contains(t, textContent.Text, `"count": 2`)
}

func TestLinodeObjectStorageBucketContentsTool_WithPrefix(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "images/photo1.jpg", Size: 2048},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "images/", r.URL.Query().Get("prefix"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":         objects,
			"is_truncated": false,
			"next_marker":  "",
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
	_, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"prefix": "images/",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "images/photo1.jpg")
	assert.Contains(t, textContent.Text, "prefix=images/")
}

func TestLinodeObjectStorageBucketContentsTool_Truncated(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "file1.txt", Size: 1024},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":         objects,
			"is_truncated": true,
			"next_marker":  "file2.txt",
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
	_, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, `"is_truncated": true`)
	assert.Contains(t, textContent.Text, "file2.txt")
}

func TestLinodeObjectStorageBucketContentsTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Object Storage Clusters List tests.

func TestNewLinodeObjectStorageClustersListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageClustersListTool(cfg)

	assert.Equal(t, "linode_object_storage_clusters_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeObjectStorageClustersListTool_Success(t *testing.T) {
	t.Parallel()

	clusters := []linode.ObjectStorageCluster{
		{ID: "us-east-1", Region: "us-east", Domain: "us-east-1.linodeobjects.com", Status: "available"},
		{ID: "eu-central-1", Region: "eu-central", Domain: "eu-central-1.linodeobjects.com", Status: "available"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/clusters", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    clusters,
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
	_, handler := tools.NewLinodeObjectStorageClustersListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "us-east-1")
	assert.Contains(t, textContent.Text, "eu-central-1")
	assert.Contains(t, textContent.Text, `"count": 2`)
}

// Object Storage Type List tests.

func TestNewLinodeObjectStorageTypeListTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

	assert.Equal(t, "linode_object_storage_type_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.NotNil(t, handler)
}

func TestLinodeObjectStorageTypeListTool_Success(t *testing.T) {
	t.Parallel()

	types := []linode.ObjectStorageType{
		{ID: "objectstorage", Label: "Object Storage", Transfer: 1000},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/types", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    types,
			"page":    1,
			"pages":   1,
			"results": 1,
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
	_, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "objectstorage")
	assert.Contains(t, textContent.Text, `"count": 1`)
}

func TestLinodeObjectStorageTypeListTool_IncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "", Token: ""},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}
