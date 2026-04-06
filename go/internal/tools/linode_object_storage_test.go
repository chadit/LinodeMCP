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

// TestLinodeObjectStorageBucketsListTool verifies the bucket listing tool
// registers correctly, handles missing environments, and returns bucket data.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns buckets, verify response format
//  3. **Missing environment**: Confirm error when environment does not exist
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_buckets_list"
//   - Populated response includes bucket details and count
//   - Missing environment returns an error result
//
// Purpose: End-to-end verification of object storage bucket listing.
func TestLinodeObjectStorageBucketsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_buckets_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		buckets := []linode.ObjectStorageBucket{
			{Label: "my-bucket", Region: "us-east-1", Hostname: "my-bucket.us-east-1.linodeobjects.com", Objects: 42, Size: 1024},
			{Label: "backups", Region: "us-southeast-1", Hostname: "backups.us-southeast-1.linodeobjects.com", Objects: 10, Size: 512},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets", r.URL.Path, "request path should match buckets endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    buckets,
				"page":    1,
				"pages":   1,
				"results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-bucket", "response should contain first bucket name")
		assert.Contains(t, textContent.Text, "backups", "response should contain second bucket name")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain correct count")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageBucketsListTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageBucketGetTool verifies the bucket get tool
// registers correctly, validates required fields, and returns bucket details.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns bucket, verify response format
//  3. **Validation**: Confirm errors for missing region and missing label
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_get"
//   - Missing region or label returns validation error
//   - Populated response includes bucket details
//
// Purpose: End-to-end verification of object storage bucket retrieval.
func TestLinodeObjectStorageBucketGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		bucket := linode.ObjectStorageBucket{
			Label:    "my-bucket",
			Region:   "us-east-1",
			Hostname: "my-bucket.us-east-1.linodeobjects.com",
			Objects:  42,
			Size:     1024,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket", r.URL.Path, "request path should match bucket endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(bucket), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-bucket", "response should contain bucket name")
		assert.Contains(t, textContent.Text, "us-east-1", "response should contain region")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args map[string]any
		}{
			{name: "missing region", args: map[string]any{"label": "my-bucket"}},
			{name: "missing label", args: map[string]any{"region": "us-east-1"}},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
			})
		}
	})
}

// TestLinodeObjectStorageBucketContentsTool verifies the bucket contents listing
// tool registers correctly, handles prefix filtering, truncation, and validation.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns objects, verify response format
//  3. **With prefix**: Confirm prefix query parameter is forwarded
//  4. **Truncated**: Verify truncation indicator and next_marker
//  5. **Validation**: Confirm errors for missing region
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_contents"
//   - Prefix filtering passes query params to the API
//   - Truncated responses include next_marker for pagination
//   - Missing region returns validation error
//
// Purpose: End-to-end verification of object listing within a bucket.
func TestLinodeObjectStorageBucketContentsTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_contents", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		objects := []linode.ObjectStorageObject{
			{Name: "file1.txt", Size: 1024, LastModified: "2024-01-15T10:00:00Z"},
			{Name: "file2.jpg", Size: 2048, LastModified: "2024-01-16T10:00:00Z"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-list", r.URL.Path, "request path should match object-list endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":         objects,
				"is_truncated": false,
				"next_marker":  "",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "file1.txt", "response should contain first object name")
		assert.Contains(t, textContent.Text, "file2.jpg", "response should contain second object name")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain correct count")
	})

	t.Run("with prefix", func(t *testing.T) {
		t.Parallel()

		objects := []linode.ObjectStorageObject{
			{Name: "images/photo1.jpg", Size: 2048},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "images/", r.URL.Query().Get("prefix"), "prefix query param should be forwarded")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":         objects,
				"is_truncated": false,
				"next_marker":  "",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"prefix": "images/",
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "images/photo1.jpg", "response should contain filtered object")
		assert.Contains(t, textContent.Text, "prefix=images/", "response should contain prefix filter info")
	})

	t.Run("truncated", func(t *testing.T) {
		t.Parallel()

		objects := []linode.ObjectStorageObject{
			{Name: "file1.txt", Size: 1024},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":         objects,
				"is_truncated": true,
				"next_marker":  "file2.txt",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"is_truncated": true`, "response should indicate truncation")
		assert.Contains(t, textContent.Text, "file2.txt", "response should contain next_marker")
	})

	t.Run("missing region", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{"label": "my-bucket"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing region")
	})
}

// TestLinodeObjectStorageClustersListTool verifies the clusters listing tool
// registers correctly and returns cluster data from the API.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns clusters, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_clusters_list"
//   - Populated response includes cluster details and count
//
// Purpose: End-to-end verification of object storage cluster listing.
func TestLinodeObjectStorageClustersListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageClustersListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_clusters_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.ObjectStorageCluster{
			{ID: "us-east-1", Region: "us-east", Domain: "us-east-1.linodeobjects.com", Status: "available"},
			{ID: "eu-central-1", Region: "eu-central", Domain: "eu-central-1.linodeobjects.com", Status: "available"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/clusters", r.URL.Path, "request path should match clusters endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    clusters,
				"page":    1,
				"pages":   1,
				"results": 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageClustersListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "us-east-1", "response should contain first cluster ID")
		assert.Contains(t, textContent.Text, "eu-central-1", "response should contain second cluster ID")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain correct count")
	})
}

// TestLinodeObjectStorageTypeListTool verifies the type listing tool
// registers correctly, handles success, and returns errors for incomplete config.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns types, verify response format
//  3. **Incomplete config**: Confirm error for empty API URL and token
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_type_list"
//   - Populated response includes type details and count
//   - Incomplete config returns an error result
//
// Purpose: End-to-end verification of object storage type listing.
func TestLinodeObjectStorageTypeListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_type_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.ObjectStorageType{
			{ID: "objectstorage", Label: "Object Storage", Transfer: 1000},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/types", r.URL.Path, "request path should match types endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    types,
				"page":    1,
				"pages":   1,
				"results": 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageTypeListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "objectstorage", "response should contain type ID")
		assert.Contains(t, textContent.Text, `"count": 1`, "response should contain correct count")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, incompleteHandler := tools.NewLinodeObjectStorageTypeListTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// TestLinodeObjectStorageKeysListTool verifies the access keys listing tool
// registers correctly, handles success, and returns errors for incomplete config.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns keys, verify response format
//  3. **Incomplete config**: Confirm error for empty API URL and token
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_keys_list"
//   - Populated response includes key details and count
//   - Incomplete config returns an error result
//
// Purpose: End-to-end verification of object storage access key listing.
func TestLinodeObjectStorageKeysListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageKeysListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_keys_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		keys := []linode.ObjectStorageKey{
			{
				ID:        1,
				Label:     "my-key",
				AccessKey: "TESTKEY00000000EXAMPLE",
				Limited:   false,
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys", r.URL.Path, "request path should match keys endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data":    keys,
				"page":    1,
				"pages":   1,
				"results": 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageKeysListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-key", "response should contain key label")
		assert.Contains(t, textContent.Text, `"count": 1`, "response should contain correct count")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, incompleteHandler := tools.NewLinodeObjectStorageKeysListTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// TestLinodeObjectStorageKeyGetTool verifies the key get tool
// registers correctly, validates required fields, and returns key details.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns key, verify response format
//  3. **Validation**: Confirm errors for missing and invalid key_id
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_key_get"
//   - Missing key_id returns validation error
//   - Invalid key_id returns descriptive error message
//   - Populated response includes key details
//
// Purpose: End-to-end verification of object storage access key retrieval.
func TestLinodeObjectStorageKeyGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_key_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		key := linode.ObjectStorageKey{
			ID:        42,
			Label:     "my-key",
			AccessKey: "TESTKEY00000000EXAMPLE",
			Limited:   true,
			BucketAccess: []linode.ObjectStorageKeyBucketAccess{
				{BucketName: "my-bucket", Region: "us-east-1", Permissions: "read_only"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys/42", r.URL.Path, "request path should match key endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(key), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageKeyGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"key_id": "42"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-key", "response should contain key label")
		assert.Contains(t, textContent.Text, "my-bucket", "response should contain bucket name")
	})

	t.Run("missing key id", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing key_id")
	})

	t.Run("invalid key id", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{"key_id": "not-a-number"})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for invalid key_id")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "key_id must be a valid integer", "error should mention invalid integer")
	})
}

// TestLinodeObjectStorageTransferTool verifies the transfer usage tool
// registers correctly, handles success, and returns errors for incomplete config.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns transfer data, verify response format
//  3. **Incomplete config**: Confirm error for empty API URL and token
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_transfer"
//   - Response includes used bytes value
//   - Incomplete config returns an error result
//
// Purpose: End-to-end verification of object storage transfer usage retrieval.
func TestLinodeObjectStorageTransferTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_transfer", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.ObjectStorageTransfer{UsedBytes: 1073741824}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/transfer", r.URL.Path, "request path should match transfer endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(transfer), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageTransferTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "1073741824", "response should contain used bytes")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, incompleteHandler := tools.NewLinodeObjectStorageTransferTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// TestLinodeObjectStorageBucketAccessGetTool verifies the bucket access get tool
// registers correctly, validates required fields, and returns access settings.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and handler
//  2. **Success**: Mock API returns access data, verify response format
//  3. **Validation**: Confirm errors for missing region, label, and environment
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_access_get"
//   - Missing region, label, or environment returns validation error
//   - Response includes ACL and CORS settings
//
// Purpose: End-to-end verification of bucket access settings retrieval.
func TestLinodeObjectStorageBucketAccessGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_access_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		access := linode.ObjectStorageBucketAccess{
			ACL:         "public-read",
			CORSEnabled: true,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(access), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketAccessGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "public-read", "response should contain ACL value")
		assert.Contains(t, textContent.Text, "true", "response should contain CORS enabled status")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args map[string]any
		}{
			{name: "missing region", args: map[string]any{"label": "my-bucket"}},
			{name: "missing label", args: map[string]any{"region": "us-east-1"}},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
			})
		}
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageBucketAccessGetTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{"region": "us-east-1", "label": "my-bucket"})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageBucketCreateTool verifies the bucket create tool
// registers correctly, validates all input fields, and creates buckets.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, schema, and WARNING
//  2. **Validation**: Table-driven subtests for missing confirm, invalid labels,
//     invalid ACL, and missing region
//  3. **Success**: Mock API creates bucket, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_create" with WARNING
//   - Missing confirm returns confirm=true prompt
//   - Invalid labels (too short, uppercase, hyphen start) return errors
//   - Invalid ACL and missing region return descriptive errors
//   - Successful creation returns bucket details
//
// Purpose: End-to-end verification of object storage bucket creation.
func TestLinodeObjectStorageBucketCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "acl", "schema should include acl property")
		assert.Contains(t, props, "cors_enabled", "schema should include cors_enabled property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"label": "my-bucket", "region": "us-east-1"},
				contains: "confirm=true",
			},
			{
				name:     "label too short",
				args:     map[string]any{"label": "ab", "region": "us-east-1", "confirm": true},
				contains: "at least 3 characters",
			},
			{
				name:     "label uppercase",
				args:     map[string]any{"label": "MyBucket", "region": "us-east-1", "confirm": true},
				contains: "lowercase",
			},
			{
				name:     "invalid ACL",
				args:     map[string]any{"label": "my-bucket", "region": "us-east-1", "acl": "invalid-acl", "confirm": true},
				contains: "acl must be one of",
			},
			{
				name:     "missing region",
				args:     map[string]any{"label": "my-bucket", "confirm": true},
				contains: "region is required",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
				assertErrorContains(t, result, testCase.contains)
			})
		}
	})

	t.Run("label start with hyphen", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"label":   "-my-bucket",
			"region":  "us-east-1",
			"confirm": true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for label starting with hyphen")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		bucket := linode.ObjectStorageBucket{
			Label:   "my-bucket",
			Region:  "us-east-1",
			Created: "2024-01-01T00:00:00",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets", r.URL.Path, "request path should match buckets endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(bucket), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":   "my-bucket",
			"region":  "us-east-1",
			"acl":     "private",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-bucket", "response should contain bucket name")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// TestLinodeObjectStorageBucketDeleteTool verifies the bucket delete tool
// registers correctly, validates required fields, and deletes buckets.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, schema, and WARNING
//  2. **Validation**: Table-driven subtests for missing confirm, region, and label
//  3. **Success**: Mock API deletes bucket, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_delete" with WARNING
//   - Missing confirm, region, or label returns descriptive errors
//   - Successful deletion returns confirmation message
//
// Purpose: End-to-end verification of object storage bucket deletion.
func TestLinodeObjectStorageBucketDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket"},
				contains: "confirm=true",
			},
			{
				name:     "missing region",
				args:     map[string]any{"label": "my-bucket", "confirm": true},
				contains: "region is required",
			},
			{
				name:     "missing label",
				args:     map[string]any{"region": "us-east-1", "confirm": true},
				contains: "label is required",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
				assertErrorContains(t, result, testCase.contains)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket", r.URL.Path, "request path should match bucket endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "removed successfully", "response should confirm deletion")
	})
}

// TestLinodeObjectStorageBucketAccessUpdateTool verifies the bucket access update
// tool registers correctly, validates fields, and updates access settings.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Table-driven subtests for missing confirm and invalid ACL
//  3. **Success**: Mock API updates access, verify response format
//  4. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_bucket_access_update"
//   - Missing confirm and invalid ACL return descriptive errors
//   - Successful update returns confirmation message
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of bucket access settings update.
func TestLinodeObjectStorageBucketAccessUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_access_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "acl", "schema should include acl property")
		assert.Contains(t, props, "cors_enabled", "schema should include cors_enabled property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket", "acl": "public-read"},
				contains: "confirm=true",
			},
			{
				name:     "invalid ACL",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket", "acl": "bad-acl", "confirm": true},
				contains: "acl must be one of",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
				assertErrorContains(t, result, testCase.contains)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/access", r.URL.Path, "request path should match access endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"acl":     "public-read",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"acl":     "private",
			"confirm": true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageKeyCreateTool verifies the key create tool
// registers correctly, validates all input fields, and creates access keys.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, schema, and WARNING
//  2. **Validation**: Table-driven subtests for missing confirm, empty label,
//     label too long, invalid JSON, invalid permissions, and missing bucket name
//  3. **Success**: Mock API creates key, verify response includes secret_key
//  4. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_key_create" with WARNING and secret_key
//   - Various validation failures return descriptive error messages
//   - Successful creation returns key with IMPORTANT secret_key notice
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of object storage access key creation.
func TestLinodeObjectStorageKeyCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_key_create", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
		assert.Contains(t, tool.Description, "WARNING", "description should contain WARNING")
		assert.Contains(t, tool.Description, "secret_key", "description should mention secret_key")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "bucket_access", "schema should include bucket_access property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains []string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"label": "my-key"},
				contains: []string{"confirm=true", "secret_key"},
			},
			{
				name:     "empty label",
				args:     map[string]any{"label": "", "confirm": true},
				contains: []string{"label is required"},
			},
			{
				name:     "label too long",
				args:     map[string]any{"label": strings.Repeat("a", 51), "confirm": true},
				contains: []string{"50 characters"},
			},
			{
				name:     "invalid bucket access JSON",
				args:     map[string]any{"label": "my-key", "bucket_access": "not-valid-json", "confirm": true},
				contains: []string{"Invalid bucket_access JSON"},
			},
			{
				name:     "invalid permissions",
				args:     map[string]any{"label": "my-key", "bucket_access": `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "admin"}]`, "confirm": true},
				contains: []string{"read_only"},
			},
			{
				name:     "missing bucket name",
				args:     map[string]any{"label": "my-key", "bucket_access": `[{"bucket_name": "", "region": "us-east-1", "permissions": "read_only"}]`, "confirm": true},
				contains: []string{"bucket_name"},
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)

				for _, expected := range testCase.contains {
					assertErrorContains(t, result, expected)
				}
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		key := linode.ObjectStorageKey{
			ID:        42,
			Label:     "my-key",
			AccessKey: "TESTKEY00000000EXAMPLE",
			SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Limited:   true,
			BucketAccess: []linode.ObjectStorageKeyBucketAccess{
				{BucketName: "mybucket", Region: "us-east-1", Permissions: "read_write"},
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys", r.URL.Path, "request path should match keys endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(key), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageKeyCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":         "my-key",
			"bucket_access": `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "read_write"}]`,
			"confirm":       true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "my-key", "response should contain key label")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
		assert.Contains(t, textContent.Text, "IMPORTANT", "response should contain IMPORTANT warning")
		assert.Contains(t, textContent.Text, "secret_key", "response should mention secret_key")
		assert.Contains(t, textContent.Text, "wJalrXUtnFEMI", "response should contain the secret key value")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageKeyCreateTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"label":   "my-key",
			"confirm": true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageKeyUpdateTool verifies the key update tool
// registers correctly, validates required fields, and updates access keys.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Table-driven subtests for missing confirm and invalid key_id
//  3. **Success**: Mock API updates key, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_key_update"
//   - Missing confirm and invalid key_id return descriptive errors
//   - Successful update returns confirmation message
//
// Purpose: End-to-end verification of object storage access key update.
func TestLinodeObjectStorageKeyUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_key_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "key_id", "schema should include key_id property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "bucket_access", "schema should include bucket_access property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"key_id": float64(42), "label": "new-label"},
				contains: "confirm=true",
			},
			{
				name:     "invalid key id",
				args:     map[string]any{"key_id": float64(0), "label": "new-label", "confirm": true},
				contains: "key_id is required",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
				assertErrorContains(t, result, testCase.contains)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys/42", r.URL.Path, "request path should match key endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageKeyUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"key_id":  float64(42),
			"label":   "updated-key",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "modified successfully", "response should confirm update")
	})
}

// TestLinodeObjectStorageKeyDeleteTool verifies the key delete tool
// registers correctly, validates required fields, and revokes access keys.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Table-driven subtests for missing confirm and invalid key_id
//  3. **Success**: Mock API revokes key, verify response format
//  4. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_key_delete"
//   - Missing confirm and invalid key_id return descriptive errors
//   - Successful revocation returns confirmation message
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of object storage access key revocation.
func TestLinodeObjectStorageKeyDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_key_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "key_id", "schema should include key_id property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "requires confirm",
				args:     map[string]any{"key_id": float64(42)},
				contains: "confirm=true",
			},
			{
				name:     "invalid key id",
				args:     map[string]any{"key_id": float64(-1), "confirm": true},
				contains: "key_id is required",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				req := createRequestWithArgs(t, testCase.args)
				result, err := handler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)
				assertErrorContains(t, result, testCase.contains)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys/42", r.URL.Path, "request path should match key endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageKeyDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"key_id":  float64(42),
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "revoked successfully", "response should confirm revocation")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageKeyDeleteTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"key_id":  float64(42),
			"confirm": true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStoragePresignedURLTool verifies the presigned URL tool
// registers correctly, validates all input fields, and generates presigned URLs.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Confirm errors for missing name, invalid method, and
//     out-of-range expires_in
//  3. **Success**: Mock API returns presigned URL, verify response format
//  4. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_presigned_url"
//   - Missing name, invalid method, and invalid expires_in return errors
//   - Successful generation returns signed URL
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of presigned URL generation.
func TestLinodeObjectStoragePresignedURLTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_presigned_url", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "name", "schema should include name property")
		assert.Contains(t, props, "method", "schema should include method property")
		assert.Contains(t, props, "expires_in", "schema should include expires_in property")
	})

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"method": "GET",
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing name")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "name", "error should mention name field")
	})

	t.Run("invalid method", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"name":   "photo.jpg",
			"method": "DELETE",
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for invalid method")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "GET", "error should mention GET")
		assert.Contains(t, textContent.Text, "PUT", "error should mention PUT")
	})

	t.Run("invalid expires in", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"region":     "us-east-1",
			"label":      "my-bucket",
			"name":       "photo.jpg",
			"method":     "GET",
			"expires_in": float64(700000),
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for invalid expires_in")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "604800", "error should mention max expiry value")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		resp := linode.PresignedURLResponse{
			URL: "https://my-bucket.us-east-1.linodeobjects.com/photo.jpg?signed=abc123",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-url", r.URL.Path, "request path should match object-url endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStoragePresignedURLTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"name":   "photo.jpg",
			"method": "GET",
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "signed=abc123", "response should contain signed URL")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStoragePresignedURLTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"name":   "photo.jpg",
			"method": "GET",
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageObjectACLGetTool verifies the object ACL get tool
// registers correctly, validates required fields, and returns ACL data.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Missing name**: Confirm error when object name is missing
//  3. **Success**: Mock API returns ACL, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_object_acl_get"
//   - Missing name returns validation error
//   - Response includes ACL value and XML
//
// Purpose: End-to-end verification of object ACL retrieval.
func TestLinodeObjectStorageObjectACLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_object_acl_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "name", "schema should include name property")
	})

	t.Run("missing name", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing name")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "name", "error should mention name field")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		acl := linode.ObjectACL{
			ACL:    "public-read",
			ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", r.URL.Path, "request path should match object-acl endpoint")
			assert.Equal(t, "photo.jpg", r.URL.Query().Get("name"), "name query param should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageObjectACLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
			"name":   "photo.jpg",
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "public-read", "response should contain ACL value")
	})
}

// TestLinodeObjectStorageObjectACLUpdateTool verifies the object ACL update tool
// registers correctly, validates all input fields, and updates object ACLs.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Confirm errors for missing confirm, name, and invalid ACL
//  3. **Success**: Mock API updates ACL, verify response format
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_object_acl_update"
//   - Missing confirm, name, and invalid ACL return descriptive errors
//   - Successful update returns updated ACL data
//
// Purpose: End-to-end verification of object ACL update.
func TestLinodeObjectStorageObjectACLUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: "https://api.linode.com/v4", Token: "test-token"}},
		},
	}
	tool, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_object_acl_update", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "name", "schema should include name property")
		assert.Contains(t, props, "acl", "schema should include acl property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			args     map[string]any
			contains string
		}{
			{
				name:     "confirm required",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket", "name": "photo.jpg", "acl": "public-read", "confirm": false},
				contains: "confirm=true",
			},
			{
				name:     "missing name",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket", "acl": "public-read", "confirm": true},
				contains: "name",
			},
			{
				name:     "invalid ACL",
				args:     map[string]any{"region": "us-east-1", "label": "my-bucket", "name": "photo.jpg", "acl": "invalid-acl", "confirm": true},
				contains: "acl must be one of",
			},
		}

		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				// Use empty cfg for confirm test (matches original test)
				testCfg := cfg
				if testCase.name == "confirm required" {
					testCfg = &config.Config{}
				}

				_, testHandler := tools.NewLinodeObjectStorageObjectACLUpdateTool(testCfg)

				req := createRequestWithArgs(t, testCase.args)
				result, err := testHandler(t.Context(), req)

				require.NoError(t, err, "handler should not return an error")
				require.NotNil(t, result, "result should not be nil")
				assert.True(t, result.IsError, "result should be an error for %s", testCase.name)

				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok, "content should be TextContent")
				assert.Contains(t, textContent.Text, testCase.contains, "error should contain expected text for %s", testCase.name)
			})
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		resp := linode.ObjectACL{
			ACL:    "public-read",
			ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", r.URL.Path, "request path should match object-acl endpoint")
			assert.Equal(t, http.MethodPut, r.Method, "request method should be PUT")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageObjectACLUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"name":    "photo.jpg",
			"acl":     "public-read",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "public-read", "response should contain ACL value")
	})
}

// TestLinodeObjectStorageSSLGetTool verifies the SSL get tool
// registers correctly, returns SSL status, and handles missing environments.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Success**: Mock API returns SSL status, verify response format
//  3. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_ssl_get"
//   - Response includes SSL boolean status
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of bucket SSL certificate status retrieval.
func TestLinodeObjectStorageSSLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_ssl_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		resp := linode.BucketSSL{
			SSL: true,
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", r.URL.Path, "request path should match ssl endpoint")
			assert.Equal(t, http.MethodGet, r.Method, "request method should be GET")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(resp), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageSSLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "true", "response should contain SSL status")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageSSLGetTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region": "us-east-1",
			"label":  "my-bucket",
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// TestLinodeObjectStorageSSLDeleteTool verifies the SSL delete tool
// registers correctly, validates confirmation, and deletes SSL certificates.
//
// Workflow:
//  1. **Definition**: Verify tool name, description, and schema
//  2. **Validation**: Confirm error when confirm is false
//  3. **Success**: Mock API deletes SSL cert, verify response format
//  4. **Missing environment**: Confirm error when environment is absent
//
// Expected Behavior:
//   - Tool registers as "linode_object_storage_ssl_delete"
//   - Missing confirm returns confirm=true prompt
//   - Successful deletion returns confirmation message
//   - Missing environment returns error result
//
// Purpose: End-to-end verification of bucket SSL certificate deletion.
func TestLinodeObjectStorageSSLDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_ssl_delete", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("confirm required", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"confirm": false,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error when confirm is false")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "confirm=true", "error should mention confirm=true")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", r.URL.Path, "request path should match ssl endpoint")
			assert.Equal(t, http.MethodDelete, r.Method, "request method should be DELETE")
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				"default": {Label: "Default", Linode: config.LinodeConfig{APIURL: srv.URL, Token: "test-token"}},
			},
		}
		_, srvHandler := tools.NewLinodeObjectStorageSSLDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"confirm": true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "SSL certificate deleted", "response should confirm SSL deletion")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, emptyHandler := tools.NewLinodeObjectStorageSSLDeleteTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			"region":  "us-east-1",
			"label":   "my-bucket",
			"confirm": true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}
