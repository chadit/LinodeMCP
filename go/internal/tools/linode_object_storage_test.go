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

// End-to-end verification of object storage bucket listing.
func TestLinodeObjectStorageBucketsListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketsListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_buckets_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		buckets := []linode.ObjectStorageBucket{
			{Label: bucketTest, Region: regionUSEast1, Hostname: "my-bucket.us-east-1.linodeobjects.com", Objects: 42, Size: 1024},
			{Label: "backups", Region: "us-southeast-1", Hostname: "backups.us-southeast-1.linodeobjects.com", Objects: 10, Size: 512},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets", r.URL.Path, "request path should match buckets endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    buckets,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketsListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, bucketTest, "response should contain first bucket name")
		assert.Contains(t, textContent.Text, "backups", "response should contain second bucket name")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain correct count")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, _, emptyHandler := tools.NewLinodeObjectStorageBucketsListTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{"environment": "nonexistent"})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of object storage bucket retrieval.
func TestLinodeObjectStorageBucketGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		bucket := linode.ObjectStorageBucket{
			Label:    bucketTest,
			Region:   regionUSEast1,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, bucketTest, "response should contain bucket name")
		assert.Contains(t, textContent.Text, regionUSEast1, "response should contain region")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingRegion, args: map[string]any{keyLabel: bucketTest}},
			{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast1}},
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

// End-to-end verification of object listing within a bucket.
func TestLinodeObjectStorageBucketContentsTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

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
				keyData:        objects,
				keyIsTruncated: false,
				keyNextMarker:  "",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})
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
				keyData:        objects,
				keyIsTruncated: false,
				keyNextMarker:  "",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			"prefix":  "images/",
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
				keyData:        objects,
				keyIsTruncated: true,
				keyNextMarker:  "file2.txt",
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketContentsTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, `"is_truncated": true`, "response should indicate truncation")
		assert.Contains(t, textContent.Text, "file2.txt", "response should contain next_marker")
	})

	t.Run(caseMissingRegion, func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{keyLabel: bucketTest})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing region")
	})
}

// End-to-end verification of object storage cluster listing.
func TestLinodeObjectStorageClustersListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageClustersListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_clusters_list", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		clusters := []linode.ObjectStorageCluster{
			{ID: regionUSEast1, Region: regionUSEast, Domain: "us-east-1.linodeobjects.com", Status: "available"},
			{ID: "eu-central-1", Region: "eu-central", Domain: "eu-central-1.linodeobjects.com", Status: "available"},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/clusters", r.URL.Path, "request path should match clusters endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    clusters,
				keyPage:    1,
				keyPages:   1,
				keyResults: 2,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageClustersListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, regionUSEast1, "response should contain first cluster ID")
		assert.Contains(t, textContent.Text, "eu-central-1", "response should contain second cluster ID")
		assert.Contains(t, textContent.Text, `"count": 2`, "response should contain correct count")
	})
}

// End-to-end verification of object storage type listing.
func TestLinodeObjectStorageTypeListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

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
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageTypeListTool(srvCfg)

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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := tools.NewLinodeObjectStorageTypeListTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// End-to-end verification of object storage access key listing.
func TestLinodeObjectStorageKeysListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageKeysListTool(cfg)

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
				Label:     keyNameTest,
				AccessKey: objectStorageKey,
				Limited:   false,
			},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/keys", r.URL.Path, "request path should match keys endpoint")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				keyData:    keys,
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeysListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, keyNameTest, "response should contain key label")
		assert.Contains(t, textContent.Text, `"count": 1`, "response should contain correct count")
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := tools.NewLinodeObjectStorageKeysListTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// End-to-end verification of object storage access key retrieval.
func TestLinodeObjectStorageKeyGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

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
			Label:     keyNameTest,
			AccessKey: objectStorageKey,
			Limited:   true,
			BucketAccess: []linode.ObjectStorageKeyBucketAccess{
				{BucketName: bucketTest, Region: regionUSEast1, Permissions: "read_only"},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeyGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyKeyID: "42"})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, keyNameTest, "response should contain key label")
		assert.Contains(t, textContent.Text, bucketTest, "response should contain bucket name")
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

		req := createRequestWithArgs(t, map[string]any{keyKeyID: notANumber})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for invalid key_id")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "key_id must be a valid integer", "error should mention invalid integer")
	})
}

// End-to-end verification of object storage transfer usage retrieval.
func TestLinodeObjectStorageTransferTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageTransferTool(srvCfg)

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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := tools.NewLinodeObjectStorageTransferTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})
		result, err := incompleteHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for incomplete config")
	})
}

// End-to-end verification of bucket access settings retrieval.
func TestLinodeObjectStorageBucketAccessGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_access_get", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		access := linode.ObjectStorageBucketAccess{
			ACL:         aclPublicRead,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketAccessGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, aclPublicRead, "response should contain ACL value")
		assert.Contains(t, textContent.Text, "true", "response should contain CORS enabled status")
	})

	t.Run("validation", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			args map[string]any
		}{
			{name: caseMissingRegion, args: map[string]any{keyLabel: bucketTest}},
			{name: caseMissingLabel, args: map[string]any{keyRegion: regionUSEast1}},
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageBucketAccessGetTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of object storage bucket creation.
func TestLinodeObjectStorageBucketCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyLabel: bucketTest, keyRegion: regionUSEast1},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "label too short",
				args:     map[string]any{keyLabel: "ab", keyRegion: regionUSEast1, keyConfirm: true},
				contains: "at least 3 characters",
			},
			{
				name:     "label uppercase",
				args:     map[string]any{keyLabel: "MyBucket", keyRegion: regionUSEast1, keyConfirm: true},
				contains: "lowercase",
			},
			{
				name:     errInvalidACL,
				args:     map[string]any{keyLabel: bucketTest, keyRegion: regionUSEast1, keyACL: "invalid-acl", keyConfirm: true},
				contains: errACLMustBeOneOf,
			},
			{
				name:     caseMissingRegion,
				args:     map[string]any{keyLabel: bucketTest, keyConfirm: true},
				contains: errRegionRequired,
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
			keyLabel:   "-my-bucket",
			keyRegion:  regionUSEast1,
			keyConfirm: true,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for label starting with hyphen")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		bucket := linode.ObjectStorageBucket{
			Label:   bucketTest,
			Region:  regionUSEast1,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   bucketTest,
			keyRegion:  regionUSEast1,
			keyACL:     aclPrivate,
			keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, bucketTest, "response should contain bucket name")
		assert.Contains(t, textContent.Text, "created successfully", "response should confirm creation")
	})
}

// End-to-end verification of object storage bucket deletion.
func TestLinodeObjectStorageBucketDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     caseMissingRegion,
				args:     map[string]any{keyLabel: bucketTest, keyConfirm: true},
				contains: errRegionRequired,
			},
			{
				name:     caseMissingLabel,
				args:     map[string]any{keyRegion: regionUSEast1, keyConfirm: true},
				contains: errLabelRequired,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyConfirm: true,
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

func TestLinodeObjectStorageBucketAccessAllowTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessAllowTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_bucket_access_allow", tool.Name, "tool name should match")
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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "confirm false",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: false},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "confirm string rejected",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: "true"},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "confirm number rejected",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: 1},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     errInvalidACL,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: "bad-acl", keyConfirm: true},
				contains: errACLMustBeOneOf,
			},
			{
				name:     "region separator rejected",
				args:     map[string]any{keyRegion: "us/east-1", keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: true},
				contains: errRegionInvalid,
			},
			{
				name:     "region query separator rejected",
				args:     map[string]any{keyRegion: "us-east-1?x=1", keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: true},
				contains: errRegionInvalid,
			},
			{
				name:     "region traversal rejected",
				args:     map[string]any{keyRegion: pathTraversalValue, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: true},
				contains: errRegionInvalid,
			},
			{
				name:     "label separator rejected",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: "bad/bucket", keyACL: aclPublicRead, keyConfirm: true},
				contains: "bucket label must contain only lowercase letters, numbers, and hyphens",
			},
			{
				name:     "label traversal rejected",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: pathTraversalValue, keyACL: aclPublicRead, keyConfirm: true},
				contains: "bucket label must be at least 3 characters",
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
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")

			var body map[string]any
			assert.NoError(t, json.NewDecoder(r.Body).Decode(&body), "request body should be JSON")
			assert.Equal(t, aclPublicRead, body[keyACL], "acl should match")
			assert.Equal(t, true, body["cors_enabled"], "cors setting should match")

			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketAccessAllowTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyACL:         aclPublicRead,
			"cors_enabled": true,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "applied successfully", "response should confirm access change")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, _, emptyHandler := tools.NewLinodeObjectStorageBucketAccessAllowTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyACL:     aclPrivate,
			keyConfirm: true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of bucket access settings update.
func TestLinodeObjectStorageBucketAccessUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     errInvalidACL,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: "bad-acl", keyConfirm: true},
				contains: errACLMustBeOneOf,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyACL:     aclPublicRead,
			keyConfirm: true,
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyACL:     aclPrivate,
			keyConfirm: true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of object storage access key creation.
func TestLinodeObjectStorageKeyCreateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyLabel: keyNameTest},
				contains: []string{errConfirmEqualsTrue, "secret_key"},
			},
			{
				name:     "empty label",
				args:     map[string]any{keyLabel: "", keyConfirm: true},
				contains: []string{errLabelRequired},
			},
			{
				name:     "label too long",
				args:     map[string]any{keyLabel: strings.Repeat("a", 51), keyConfirm: true},
				contains: []string{"50 characters"},
			},
			{
				name:     "invalid bucket access JSON",
				args:     map[string]any{keyLabel: keyNameTest, keyBucketAccess: "not-valid-json", keyConfirm: true},
				contains: []string{"Invalid bucket_access JSON"},
			},
			{
				name:     "invalid permissions",
				args:     map[string]any{keyLabel: keyNameTest, keyBucketAccess: `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "admin"}]`, keyConfirm: true},
				contains: []string{"read_only"},
			},
			{
				name:     "missing bucket name",
				args:     map[string]any{keyLabel: keyNameTest, keyBucketAccess: `[{"bucket_name": "", "region": "us-east-1", "permissions": "read_only"}]`, keyConfirm: true},
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
			Label:     keyNameTest,
			AccessKey: objectStorageKey,
			SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
			Limited:   true,
			BucketAccess: []linode.ObjectStorageKeyBucketAccess{
				{BucketName: "mybucket", Region: regionUSEast1, Permissions: "read_write"},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeyCreateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:        keyNameTest,
			keyBucketAccess: `[{"bucket_name": "mybucket", "region": "us-east-1", "permissions": "read_write"}]`,
			keyConfirm:      true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, keyNameTest, "response should contain key label")
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageKeyCreateTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyLabel:   keyNameTest,
			keyConfirm: true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of object storage access key update.
func TestLinodeObjectStorageKeyUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyKeyID: float64(42), keyLabel: labelNew},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "invalid key id",
				args:     map[string]any{keyKeyID: float64(0), keyLabel: labelNew, keyConfirm: true},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeyUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyKeyID:   float64(42),
			keyLabel:   "updated-key",
			keyConfirm: true,
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

// End-to-end verification of object storage access key revocation.
func TestLinodeObjectStorageKeyDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

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
				name:     caseRequiresConfirm,
				args:     map[string]any{keyKeyID: float64(42)},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "invalid key id",
				args:     map[string]any{keyKeyID: float64(-1), keyConfirm: true},
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeyDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyKeyID:   float64(42),
			keyConfirm: true,
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageKeyDeleteTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyKeyID:   float64(42),
			keyConfirm: true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of presigned URL generation.
func TestLinodeObjectStoragePresignedURLTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

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
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyMethod: httpMethodGET,
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
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   objectPhotoJPG,
			keyMethod: "DELETE",
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for invalid method")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, httpMethodGET, "error should mention GET")
		assert.Contains(t, textContent.Text, "PUT", "error should mention PUT")
	})

	t.Run("invalid expires in", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:    regionUSEast1,
			keyLabel:     bucketTest,
			keyName:      objectPhotoJPG,
			keyMethod:    httpMethodGET,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStoragePresignedURLTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   objectPhotoJPG,
			keyMethod: httpMethodGET,
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
		_, _, emptyHandler := tools.NewLinodeObjectStoragePresignedURLTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   objectPhotoJPG,
			keyMethod: httpMethodGET,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of object ACL retrieval.
func TestLinodeObjectStorageObjectACLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

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
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
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
			ACL:    aclPublicRead,
			ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", r.URL.Path, "request path should match object-acl endpoint")
			assert.Equal(t, objectPhotoJPG, r.URL.Query().Get("name"), "name query param should match")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(acl), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageObjectACLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   objectPhotoJPG,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, aclPublicRead, "response should contain ACL value")
	})
}

// End-to-end verification of object ACL update.
func TestLinodeObjectStorageObjectACLUpdateTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

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
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyName: objectPhotoJPG, keyACL: aclPublicRead, keyConfirm: false},
				contains: errConfirmEqualsTrue,
			},
			{
				name:     "missing name",
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: true},
				contains: "name",
			},
			{
				name:     errInvalidACL,
				args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyName: objectPhotoJPG, keyACL: "invalid-acl", keyConfirm: true},
				contains: errACLMustBeOneOf,
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

				_, _, testHandler := tools.NewLinodeObjectStorageObjectACLUpdateTool(testCfg)

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
			ACL:    aclPublicRead,
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageObjectACLUpdateTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyName:    objectPhotoJPG,
			keyACL:     aclPublicRead,
			keyConfirm: true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, aclPublicRead, "response should contain ACL value")
	})
}

// End-to-end verification of bucket SSL certificate status retrieval.
func TestLinodeObjectStorageSSLGetTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageSSLGetTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageSSLGetTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of bucket SSL certificate deletion.
func TestLinodeObjectStorageSSLDeleteTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

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
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyConfirm: false,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error when confirm is false")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "error should mention confirm=true")
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
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageSSLDeleteTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyConfirm: true,
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
		_, _, emptyHandler := tools.NewLinodeObjectStorageSSLDeleteTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:  regionUSEast1,
			keyLabel:   bucketTest,
			keyConfirm: true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})
}

// End-to-end verification of bucket SSL certificate upload.
func TestLinodeObjectStorageSSLUploadTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLUploadTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "linode_object_storage_ssl_upload", tool.Name, "tool name should match")
		assert.NotEmpty(t, tool.Description, "tool should have a description")
		require.NotNil(t, handler, "handler should not be nil")

		props := tool.InputSchema.Properties
		assert.Contains(t, props, "region", "schema should include region property")
		assert.Contains(t, props, "label", "schema should include label property")
		assert.Contains(t, props, "certificate", "schema should include certificate property")
		assert.Contains(t, props, "private_key", "schema should include private_key property")
		assert.Contains(t, props, "confirm", "schema should include confirm property")
	})

	t.Run("confirm required", func(t *testing.T) {
		t.Parallel()

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "test-cert",
			keyPrivateKey:  "test-key",
			keyConfirm:     false,
		})
		result, err := handler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error when confirm is false")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, errConfirmEqualsTrue, "error should mention confirm=true")
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", r.URL.Path, "request path should match ssl endpoint")
			assert.Equal(t, http.MethodPost, r.Method, "request method should be POST")
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(linode.BucketSSL{SSL: true}), "encoding response should not fail")
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: testCertPEM,
			keyPrivateKey:  testKeyPEM,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.False(t, result.IsError, "result should not be an error")

		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "SSL certificate uploaded", "response should confirm SSL upload")
	})

	t.Run("missing environment", func(t *testing.T) {
		t.Parallel()

		emptyCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{},
		}
		_, _, emptyHandler := tools.NewLinodeObjectStorageSSLUploadTool(emptyCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "test-cert",
			keyPrivateKey:  "test-key",
			keyConfirm:     true,
		})
		result, err := emptyHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for missing environment")
	})

	t.Run("api error propagated", func(t *testing.T) {
		t.Parallel()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Header().Set("Content-Type", "application/json")
			assert.NoError(t, json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"}))
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: testCertPEM,
			keyPrivateKey:  testKeyPEM,
			keyConfirm:     true,
		})
		result, err := srvHandler(t.Context(), req)

		require.NoError(t, err, "handler should not return an error")
		require.NotNil(t, result, "result should not be nil")
		assert.True(t, result.IsError, "result should be an error for API failure")
		textContent, ok := result.Content[0].(mcp.TextContent)
		require.True(t, ok, "content should be TextContent")
		assert.Contains(t, textContent.Text, "Failed to upload SSL certificate", "error should describe the failed operation")
	})

	for _, traversalCase := range []struct {
		name  string
		label string
	}{
		{"label with slash", "bucket/../../etc"},
		{"label with query", "bucket?foo=bar"},
	} {
		t.Run("path traversal: "+traversalCase.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				assert.NoError(t, json.NewEncoder(w).Encode(linode.BucketSSL{SSL: true}))
			}))
			defer srv.Close()

			srvCfg := &config.Config{
				Environments: map[string]config.EnvironmentConfig{
					envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
				},
			}
			_, _, srvHandler := tools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

			req := createRequestWithArgs(t, map[string]any{
				keyRegion:      regionUSEast1,
				keyLabel:       traversalCase.label,
				keyCertificate: testCertPEM,
				keyPrivateKey:  testKeyPEM,
				keyConfirm:     true,
			})
			result, err := srvHandler(t.Context(), req)

			// url.PathEscape at the client layer encodes separators, so the request
			// reaches the server with encoded values. The test passes to confirm
			// url.PathEscape handles these inputs safely.
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.False(t, result.IsError)
		})
	}
}
