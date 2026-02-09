package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
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
	require.NotNil(t, handler)
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

// Phase 2: Access Key & Transfer Tests

func TestLinodeObjectStorageKeysListTool_Definition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeysListTool(cfg)

	assert.Equal(t, "linode_object_storage_keys_list", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeObjectStorageKeysListTool_Success(t *testing.T) {
	t.Parallel()

	keys := []linode.ObjectStorageKey{
		{
			ID:        1,
			Label:     "my-key",
			AccessKey: "AKIAIOSFODNN7EXAMPLE",
			Limited:   false,
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/keys", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data":    keys,
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
	_, handler := tools.NewLinodeObjectStorageKeysListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-key")
	assert.Contains(t, textContent.Text, `"count": 1`)
}

func TestLinodeObjectStorageKeysListTool_IncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "", Token: ""},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeysListTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageKeyGetTool_Definition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	assert.Equal(t, "linode_object_storage_key_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeObjectStorageKeyGetTool_Success(t *testing.T) {
	t.Parallel()

	key := linode.ObjectStorageKey{
		ID:        42,
		Label:     "my-key",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		Limited:   true,
		BucketAccess: []linode.ObjectStorageKeyBucketAccess{
			{BucketName: "my-bucket", Region: "us-east-1", Permissions: "read_only"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/keys/42", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(key))
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
	_, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"key_id": "42"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-key")
	assert.Contains(t, textContent.Text, "my-bucket")
}

func TestLinodeObjectStorageKeyGetTool_MissingKeyID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageKeyGetTool_InvalidKeyID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"key_id": "not-a-number"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "key_id must be a valid integer")
}

func TestLinodeObjectStorageTransferTool_Definition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

	assert.Equal(t, "linode_object_storage_transfer", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeObjectStorageTransferTool_Success(t *testing.T) {
	t.Parallel()

	transfer := linode.ObjectStorageTransfer{UsedBytes: 1073741824}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/transfer", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(transfer))
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
	_, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "1073741824")
}

func TestLinodeObjectStorageTransferTool_IncompleteConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "", Token: ""},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

	req := createRequestWithArgs(t, map[string]any{})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageBucketAccessGetTool_Definition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_access_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
}

func TestLinodeObjectStorageBucketAccessGetTool_Success(t *testing.T) {
	t.Parallel()

	access := linode.ObjectStorageBucketAccess{
		ACL:         "public-read",
		CORSEnabled: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(access))
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
	_, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "public-read")
	assert.Contains(t, textContent.Text, "true")
}

func TestLinodeObjectStorageBucketAccessGetTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageBucketAccessGetTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestLinodeObjectStorageBucketAccessGetTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Phase 3: Write Bucket Tool Tests.

func TestNewLinodeObjectStorageBucketCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "acl")
	assert.Contains(t, props, "cors_enabled")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageBucketCreateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":  "my-bucket",
		"region": "us-east-1",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeObjectStorageBucketCreateTool_InvalidLabel_TooShort(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "ab",
		"region":  "us-east-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "at least 3 characters")
}

func TestLinodeObjectStorageBucketCreateTool_InvalidLabel_Uppercase(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "MyBucket",
		"region":  "us-east-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "lowercase")
}

func TestLinodeObjectStorageBucketCreateTool_InvalidLabel_StartWithHyphen(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "-my-bucket",
		"region":  "us-east-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestValidateBucketLabel_IPAddress(t *testing.T) {
	t.Parallel()

	ipLabels := []string{"192.168.1.1", "10.0.0.1", "127.0.0.1"}
	for _, label := range ipLabels {
		err := tools.ValidateBucketLabel(label)
		assert.ErrorIs(t, err, tools.ErrBucketLabelIPAddress, "label %q should be rejected as IP", label)
	}
}

func TestValidateBucketLabel_XNPrefix(t *testing.T) {
	t.Parallel()

	err := tools.ValidateBucketLabel("xn--example")
	assert.ErrorIs(t, err, tools.ErrBucketLabelXNPrefix)
}

func TestValidateBucketLabel_ValidNames(t *testing.T) {
	t.Parallel()

	validLabels := []string{"my-bucket", "test123", "a-b-c"}
	for _, label := range validLabels {
		err := tools.ValidateBucketLabel(label)
		assert.NoError(t, err, "label %q should be valid", label)
	}
}

func TestLinodeObjectStorageBucketCreateTool_InvalidACL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-bucket",
		"region":  "us-east-1",
		"acl":     "invalid-acl",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "acl must be one of")
}

func TestLinodeObjectStorageBucketCreateTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-bucket",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeObjectStorageBucketCreateTool_Success(t *testing.T) {
	t.Parallel()

	bucket := linode.ObjectStorageBucket{
		Label:   "my-bucket",
		Region:  "us-east-1",
		Created: "2024-01-01T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
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
	_, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-bucket",
		"region":  "us-east-1",
		"acl":     "private",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-bucket")
	assert.Contains(t, textContent.Text, "created successfully")
}

func TestNewLinodeObjectStorageBucketDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageBucketDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeObjectStorageBucketDeleteTool_MissingRegion(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-bucket",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "region is required")
}

func TestLinodeObjectStorageBucketDeleteTool_MissingLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeObjectStorageBucketDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
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
	_, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "deleted successfully")
}

func TestNewLinodeObjectStorageBucketAccessUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	assert.Equal(t, "linode_object_storage_bucket_access_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "acl")
	assert.Contains(t, props, "cors_enabled")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageBucketAccessUpdateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"acl":    "public-read",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeObjectStorageBucketAccessUpdateTool_InvalidACL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"acl":     "bad-acl",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "acl must be one of")
}

func TestLinodeObjectStorageBucketAccessUpdateTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.WriteHeader(http.StatusOK)
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
	_, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"acl":     "public-read",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

func TestLinodeObjectStorageBucketAccessUpdateTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"acl":     "private",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Phase 4: Write Access Key Tool Tests.

func TestNewLinodeObjectStorageKeyCreateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	assert.Equal(t, "linode_object_storage_key_create", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)
	assert.Contains(t, tool.Description, "WARNING")
	assert.Contains(t, tool.Description, "secret_key")

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "bucket_access")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageKeyCreateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label": "my-key",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
	assertErrorContains(t, result, "secret_key")
}

func TestLinodeObjectStorageKeyCreateTool_EmptyLabel(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "label is required")
}

func TestLinodeObjectStorageKeyCreateTool_LabelTooLong(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	longLabel := strings.Repeat("a", 51)

	req := createRequestWithArgs(t, map[string]any{
		"label":   longLabel,
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "50 characters")
}

func TestLinodeObjectStorageKeyCreateTool_InvalidBucketAccessJSON(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":         "my-key",
		"bucket_access": "not-valid-json",
		"confirm":       true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "Invalid bucket_access JSON")
}

func TestLinodeObjectStorageKeyCreateTool_InvalidPermissions(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":         "my-key",
		"bucket_access": `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "admin"}]`,
		"confirm":       true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "read_only")
}

func TestLinodeObjectStorageKeyCreateTool_MissingBucketName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":         "my-key",
		"bucket_access": `[{"bucket_name": "", "region": "us-east-1", "permissions": "read_only"}]`,
		"confirm":       true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "bucket_name")
}

func TestLinodeObjectStorageKeyCreateTool_Success(t *testing.T) {
	t.Parallel()

	key := linode.ObjectStorageKey{
		ID:        42,
		Label:     "my-key",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Limited:   true,
		BucketAccess: []linode.ObjectStorageKeyBucketAccess{
			{BucketName: "mybucket", Region: "us-east-1", Permissions: "read_write"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/keys", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(key))
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
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":         "my-key",
		"bucket_access": `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "read_write"}]`,
		"confirm":       true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "my-key")
	assert.Contains(t, textContent.Text, "created successfully")
	assert.Contains(t, textContent.Text, "IMPORTANT")
	assert.Contains(t, textContent.Text, "secret_key")
	assert.Contains(t, textContent.Text, "wJalrXUtnFEMI")
}

func TestLinodeObjectStorageKeyCreateTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"label":   "my-key",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Key Update tests.

func TestNewLinodeObjectStorageKeyUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	assert.Equal(t, "linode_object_storage_key_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "key_id")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "bucket_access")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageKeyUpdateTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id": float64(42),
		"label":  "new-label",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeObjectStorageKeyUpdateTool_InvalidKeyID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id":  float64(0),
		"label":   "new-label",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "key_id is required")
}

func TestLinodeObjectStorageKeyUpdateTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/keys/42", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
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
	_, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id":  float64(42),
		"label":   "updated-key",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "updated successfully")
}

// Key Delete tests.

func TestNewLinodeObjectStorageKeyDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	assert.Equal(t, "linode_object_storage_key_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "key_id")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageKeyDeleteTool_RequiresConfirm(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id": float64(42),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "confirm=true")
}

func TestLinodeObjectStorageKeyDeleteTool_InvalidKeyID(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id":  float64(-1),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assertErrorContains(t, result, "key_id is required")
}

func TestLinodeObjectStorageKeyDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/keys/42", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
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
	_, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id":  float64(42),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "revoked successfully")
}

func TestLinodeObjectStorageKeyDeleteTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"key_id":  float64(42),
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Phase 5: Presigned URL tests.

func TestNewLinodeObjectStoragePresignedURLTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	assert.Equal(t, "linode_object_storage_presigned_url", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "method")
	assert.Contains(t, props, "expires_in")
}

func TestLinodeObjectStoragePresignedURLTool_MissingName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"method": "GET",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "name")
}

func TestLinodeObjectStoragePresignedURLTool_InvalidMethod(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"name":   "photo.jpg",
		"method": "DELETE",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "GET")
	assert.Contains(t, textContent.Text, "PUT")
}

func TestLinodeObjectStoragePresignedURLTool_InvalidExpiresIn(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":     "us-east-1",
		"label":      "my-bucket",
		"name":       "photo.jpg",
		"method":     "GET",
		"expires_in": float64(700000),
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "604800")
}

func TestLinodeObjectStoragePresignedURLTool_Success(t *testing.T) {
	t.Parallel()

	resp := linode.PresignedURLResponse{
		URL: "https://my-bucket.us-east-1.linodeobjects.com/photo.jpg?signed=abc123",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-url", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
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
	_, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"name":   "photo.jpg",
		"method": "GET",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "signed=abc123")
}

func TestLinodeObjectStoragePresignedURLTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"name":   "photo.jpg",
		"method": "GET",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Phase 5: Object ACL Get tests.

func TestNewLinodeObjectStorageObjectACLGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	assert.Equal(t, "linode_object_storage_object_acl_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "name")
}

func TestLinodeObjectStorageObjectACLGetTool_MissingName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "name")
}

func TestLinodeObjectStorageObjectACLGetTool_Success(t *testing.T) {
	t.Parallel()

	acl := linode.ObjectACL{
		ACL:    "public-read",
		ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", r.URL.Path)
		assert.Equal(t, "photo.jpg", r.URL.Query().Get("name"))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(acl))
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
	_, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
		"name":   "photo.jpg",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "public-read")
}

// Phase 5: Object ACL Update tests.

func TestNewLinodeObjectStorageObjectACLUpdateTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	assert.Equal(t, "linode_object_storage_object_acl_update", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "name")
	assert.Contains(t, props, "acl")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageObjectACLUpdateTool_ConfirmRequired(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"name":    "photo.jpg",
		"acl":     "public-read",
		"confirm": false,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "confirm=true")
}

func TestLinodeObjectStorageObjectACLUpdateTool_MissingName(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"acl":     "public-read",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "name")
}

func TestLinodeObjectStorageObjectACLUpdateTool_InvalidACL(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {
				Label:  "Default",
				Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"},
			},
		},
	}
	_, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"name":    "photo.jpg",
		"acl":     "invalid-acl",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "acl must be one of")
}

func TestLinodeObjectStorageObjectACLUpdateTool_Success(t *testing.T) {
	t.Parallel()

	resp := linode.ObjectACL{
		ACL:    "public-read",
		ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", r.URL.Path)
		assert.Equal(t, http.MethodPut, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
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
	_, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"name":    "photo.jpg",
		"acl":     "public-read",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "public-read")
}

// Phase 5: SSL Get tests.

func TestNewLinodeObjectStorageSSLGetTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

	assert.Equal(t, "linode_object_storage_ssl_get", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
}

func TestLinodeObjectStorageSSLGetTool_Success(t *testing.T) {
	t.Parallel()

	resp := linode.BucketSSL{
		SSL: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(resp))
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
	_, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "true")
}

func TestLinodeObjectStorageSSLGetTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region": "us-east-1",
		"label":  "my-bucket",
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Phase 5: SSL Delete tests.

func TestNewLinodeObjectStorageSSLDeleteTool_ToolDefinition(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	assert.Equal(t, "linode_object_storage_ssl_delete", tool.Name)
	assert.NotEmpty(t, tool.Description)
	require.NotNil(t, handler)

	props := tool.InputSchema.Properties
	assert.Contains(t, props, "region")
	assert.Contains(t, props, "label")
	assert.Contains(t, props, "confirm")
}

func TestLinodeObjectStorageSSLDeleteTool_ConfirmRequired(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"confirm": false,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "confirm=true")
}

func TestLinodeObjectStorageSSLDeleteTool_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)
		w.WriteHeader(http.StatusOK)
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
	_, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	assert.Contains(t, textContent.Text, "SSL certificate deleted")
}

func TestLinodeObjectStorageSSLDeleteTool_MissingEnvironment(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		"region":  "us-east-1",
		"label":   "my-bucket",
		"confirm": true,
	})
	result, err := handler(t.Context(), req)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}
