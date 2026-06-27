package tools_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

const (
	bucketHostnameUSEast1       = "my-bucket.us-east-1.linodeobjects.com"
	keyObjectStorageQuotaID     = "obj_quota_id"
	objectStorageEndpointUSEast = "us-east-1.linodeobjects.com"
	objectStorageQuotaTestID    = "obj-buckets-us-sea-1.linodeobjects.com"
)

// End-to-end verification of object storage bucket listing.
func TestLinodeObjectStorageBucketsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageBucketsListToolSuccess(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: bucketTest, Region: regionUSEast1, Hostname: bucketHostnameUSEast1, Objects: 42, Size: 1024},
		{Label: tcBackups, Region: "us-southeast-1", Hostname: "backups.us-southeast-1.linodeobjects.com", Objects: 10, Size: 512},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    buckets,
			keyPage:    1,
			keyPages:   1,
			keyResults: 2,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageBucketListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, bucketTest) {
		t.Errorf("textContent.Text does not contain %v", bucketTest)
	}

	if !strings.Contains(textContent.Text, tcBackups) {
		t.Errorf("textContent.Text does not contain %v", tcBackups)
	}

	if !strings.Contains(textContent.Text, `"count": 2`) {
		t.Errorf("textContent.Text does not contain %v", `"count": 2`)
	}
}

func TestLinodeObjectStorageBucketsListToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := tools.NewLinodeObjectStorageBucketListTool(emptyCfg)

	req := createRequestWithArgs(t, map[string]any{canRunKeyEnv: "nonexistent"})

	result, err := emptyHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage bucket listing by region.
func TestLinodeObjectStorageBucketsListByRegionToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketListByRegionTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_by_region_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_by_region_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageBucketsListByRegionToolSuccess(t *testing.T) {
	t.Parallel()

	buckets := []linode.ObjectStorageBucket{
		{Label: bucketTest, Region: regionUSEast1, Hostname: bucketHostnameUSEast1, Objects: 42, Size: 1024},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/buckets/us-east-1" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    buckets,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageBucketListByRegionTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, bucketTest) {
		t.Errorf("textContent.Text does not contain %v", bucketTest)
	}

	if !strings.Contains(textContent.Text, regionUSEast1) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast1)
	}

	if !strings.Contains(textContent.Text, `"count": 1`) {
		t.Errorf("textContent.Text does not contain %v", `"count": 1`)
	}
}

func TestLinodeObjectStorageBucketsListByRegionToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketListByRegionTool(cfg)

	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingRegion, args: map[string]any{}},
		{name: "slash in region", args: map[string]any{keyRegion: "us/east-1"}},
		{name: "query in region", args: map[string]any{keyRegion: "us-east-1?x=1"}},
		{name: "traversal in region", args: map[string]any{keyRegion: pathTraversalValue}},
		{name: "encoded separator in region", args: map[string]any{keyRegion: "us%2Feast-1"}},
		{name: "fragment in region", args: map[string]any{keyRegion: "us-east-1#frag"}},
		{name: "ampersand in region", args: map[string]any{keyRegion: "us-east-1&x=1"}},
		{name: "space in region", args: map[string]any{keyRegion: "us east-1"}},
		{name: "leading dash in region", args: map[string]any{keyRegion: "-us-east-1"}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}
		})
	}
}

// End-to-end verification of object storage bucket retrieval.
func TestLinodeObjectStorageBucketGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageBucketGetToolSuccess(t *testing.T) {
	t.Parallel()

	bucket := linode.ObjectStorageBucket{
		Label:    bucketTest,
		Region:   regionUSEast1,
		Hostname: bucketHostnameUSEast1,
		Objects:  42,
		Size:     1024,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucket {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucket)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(bucket); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, bucketTest) {
		t.Errorf("textContent.Text does not contain %v", bucketTest)
	}

	if !strings.Contains(textContent.Text, regionUSEast1) {
		t.Errorf("textContent.Text does not contain %v", regionUSEast1)
	}
}

func TestLinodeObjectStorageBucketGetToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketGetTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}
		})
	}
}

// End-to-end verification of object listing within a bucket.
func TestLinodeObjectStorageBucketContentsToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_object_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_object_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageBucketContentsToolSuccess(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "file1.txt", Size: 1024, LastModified: "2024-01-15T10:00:00Z"},
		{Name: "file2.jpg", Size: 2048, LastModified: "2024-01-16T10:00:00Z"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/object-list" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/object-list")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:        objects,
			keyIsTruncated: false,
			keyNextMarker:  "",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "file1.txt") {
		t.Errorf("textContent.Text does not contain %v", "file1.txt")
	}

	if !strings.Contains(textContent.Text, "file2.jpg") {
		t.Errorf("textContent.Text does not contain %v", "file2.jpg")
	}

	if !strings.Contains(textContent.Text, `"count": 2`) {
		t.Errorf("textContent.Text does not contain %v", `"count": 2`)
	}
}

func TestLinodeObjectStorageBucketContentsToolWithPrefix(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "images/photo1.jpg", Size: 2048},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("prefix") != "images/" {
			t.Errorf("got %v, want %v", r.URL.Query().Get("prefix"), "images/")
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:        objects,
			keyIsTruncated: false,
			keyNextMarker:  "",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "images/photo1.jpg") {
		t.Errorf("textContent.Text does not contain %v", "images/photo1.jpg")
	}

	if !strings.Contains(textContent.Text, "prefix=images/") {
		t.Errorf("textContent.Text does not contain %v", "prefix=images/")
	}
}

func TestLinodeObjectStorageBucketContentsToolTruncated(t *testing.T) {
	t.Parallel()

	objects := []linode.ObjectStorageObject{
		{Name: "file1.txt", Size: 1024},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:        objects,
			keyIsTruncated: true,
			keyNextMarker:  "file2.txt",
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, `"is_truncated": true`) {
		t.Errorf("textContent.Text does not contain %v", `"is_truncated": true`)
	}

	if !strings.Contains(textContent.Text, "file2.txt") {
		t.Errorf("textContent.Text does not contain %v", "file2.txt")
	}
}

func TestLinodeObjectStorageBucketContentsToolCaseMissingRegion(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketContentsTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{keyLabel: bucketTest})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage endpoint listing.
func TestLinodeObjectStorageEndpointsListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageEndpointListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_endpoint_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_endpoint_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageEndpointsListToolSuccess(t *testing.T) {
	t.Parallel()

	s3Endpoint := objectStorageEndpointUSEast
	endpoints := []linode.ObjectStorageEndpoint{
		{Region: regionUSEast, S3Endpoint: &s3Endpoint, EndpointType: "E0"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/endpoints" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/endpoints")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    endpoints,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageEndpointListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, s3Endpoint) {
		t.Errorf("textContent.Text does not contain %v", s3Endpoint)
	}

	if !strings.Contains(textContent.Text, `"endpoint_type": "E0"`) {
		t.Errorf("textContent.Text does not contain %v", `"endpoint_type": "E0"`)
	}

	if !strings.Contains(textContent.Text, `"count": 1`) {
		t.Errorf("textContent.Text does not contain %v", `"count": 1`)
	}
}

func TestLinodeObjectStorageEndpointsListToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/endpoints" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/endpoints")
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		http.Error(w, `{"errors":[{"reason":"service unavailable"}]}`, http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageEndpointListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Object Storage endpoints") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Object Storage endpoints")
	}
}

func TestLinodeObjectStorageEndpointsListToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	incompleteCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
		},
	}
	_, _, incompleteHandler := tools.NewLinodeObjectStorageEndpointListTool(incompleteCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := incompleteHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageTypeListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageTypeListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_object_storage_type_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_type_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		types := []linode.ObjectStorageType{
			{ID: "objectstorage", Label: "Object Storage", Transfer: 1000},
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/object-storage/types" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/types")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData:    types,
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "objectstorage") {
			t.Errorf("textContent.Text does not contain %v", "objectstorage")
		}

		if !strings.Contains(textContent.Text, `"count": 1`) {
			t.Errorf("textContent.Text does not contain %v", `"count": 1`)
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

// End-to-end verification of object storage quota listing.
func TestLinodeObjectStorageQuotasListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageQuotasListTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_quota_list" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_quota_list")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageQuotasListToolSuccess(t *testing.T) {
	const (
		quotaIDKey = "quota_id"
		quotaID    = "endpoint-type-1"
	)

	t.Parallel()

	quotas := []linode.ObjectStorageQuota{
		{keyBetaID: quotaID, quotaIDKey: quotaID, "s3_endpoint": objectStorageEndpointUSEast},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if len(body) != 0 {
			t.Errorf("body = %v, want empty", body)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]any{
			keyData:    quotas,
			keyPage:    1,
			keyPages:   1,
			keyResults: 1,
		}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageQuotasListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, quotaID) {
		t.Errorf("textContent.Text does not contain %v", quotaID)
	}

	if !strings.Contains(textContent.Text, `"count": 1`) {
		t.Errorf("textContent.Text does not contain %v", `"count": 1`)
	}
}

func TestLinodeObjectStorageQuotasListToolApiError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas")
		}

		w.WriteHeader(http.StatusInternalServerError)

		_, err := w.Write([]byte(`{"errors":[{"reason":"quota service unavailable"}]}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageQuotasListTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to retrieve Object Storage quotas") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve Object Storage quotas")
	}
}

func TestLinodeObjectStorageQuotasListToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	incompleteCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
		},
	}
	_, _, incompleteHandler := tools.NewLinodeObjectStorageQuotasListTool(incompleteCfg)

	req := createRequestWithArgs(t, map[string]any{})

	result, err := incompleteHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage access key listing.
func TestLinodeObjectStorageKeysListTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageKeyListTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_object_storage_key_list" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_key_list")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
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
			if r.URL.Path != "/object-storage/keys" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/keys")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(map[string]any{
				keyData:    keys,
				keyPage:    1,
				keyPages:   1,
				keyResults: 1,
			}); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}))
		defer srv.Close()

		srvCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
			},
		}
		_, _, srvHandler := tools.NewLinodeObjectStorageKeyListTool(srvCfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := srvHandler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, keyNameTest) {
			t.Errorf("textContent.Text does not contain %v", keyNameTest)
		}

		if !strings.Contains(textContent.Text, `"count": 1`) {
			t.Errorf("textContent.Text does not contain %v", `"count": 1`)
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := tools.NewLinodeObjectStorageKeyListTool(incompleteCfg)

		req := createRequestWithArgs(t, map[string]any{})

		result, err := incompleteHandler(t.Context(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

// End-to-end verification of object storage access key retrieval.
func TestLinodeObjectStorageKeyGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_key_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_key_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageKeyGetToolSuccess(t *testing.T) {
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
		if r.URL.Path != tcObjectStorageKeys42 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageKeys42)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(key); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageKeyGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyKeyID: float64(42)})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, keyNameTest) {
		t.Errorf("textContent.Text does not contain %v", keyNameTest)
	}

	if !strings.Contains(textContent.Text, bucketTest) {
		t.Errorf("textContent.Text does not contain %v", bucketTest)
	}
}

func TestLinodeObjectStorageKeyGetToolMissingKeyId(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageKeyGetToolInvalidKeyId(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyGetTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{keyKeyID: notANumber})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "key_id must be an integer") {
		t.Errorf("textContent.Text does not contain %v", "key_id must be an integer")
	}
}

// End-to-end verification of object storage quota usage retrieval.
func TestLinodeObjectStorageQuotaUsageToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageQuotaUsageTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_quota_usage_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_quota_usage_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageQuotaUsageToolSuccess(t *testing.T) {
	t.Parallel()

	used := 10
	usage := linode.ObjectStorageQuotaUsage{QuotaLimit: 100, Usage: &used}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas/obj-bucket-us-ord-1/usage" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas/obj-bucket-us-ord-1/usage")
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(usage); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageQuotaUsageTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{"obj_quota_id": "obj-bucket-us-ord-1"})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "100") {
		t.Errorf("textContent.Text does not contain %v", "100")
	}

	if !strings.Contains(textContent.Text, "10") {
		t.Errorf("textContent.Text does not contain %v", "10")
	}
}

func TestLinodeObjectStorageQuotaUsageToolMissingQuotaId(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeObjectStorageQuotaUsageTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageQuotaUsageToolInvalidQuotaId(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeObjectStorageQuotaUsageTool(cfg)

	t.Parallel()

	for _, quotaID := range []string{"obj/bucket", "obj?bucket", "obj..bucket"} {
		t.Run(quotaID, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, map[string]any{"obj_quota_id": quotaID})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "obj_quota_id must not contain") {
				t.Errorf("textContent.Text does not contain %v", "obj_quota_id must not contain")
			}
		})
	}
}

// End-to-end verification of object storage transfer usage retrieval.
func TestLinodeObjectStorageTransferTool(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageTransferTool(cfg)

	t.Run("definition", func(t *testing.T) {
		t.Parallel()

		if tool.Name != "linode_object_storage_transfer_get" {
			t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_transfer_get")
		}

		if tool.Description == "" {
			t.Error("tool.Description is empty")
		}

		if handler == nil {
			t.Fatal("handler is nil")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		transfer := linode.ObjectStorageTransfer{UsedBytes: 1073741824}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/object-storage/transfer" {
				t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/transfer")
			}

			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(transfer); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if result.IsError {
			t.Error("result.IsError = true, want false")
		}

		textContent, ok := result.Content[0].(mcp.TextContent)
		if !ok {
			t.Fatal("ok = false, want true")
		}

		if !strings.Contains(textContent.Text, "1073741824") {
			t.Errorf("textContent.Text does not contain %v", "1073741824")
		}
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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result == nil {
			t.Fatal("result is nil")
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}
	})
}

// End-to-end verification of object storage quota retrieval.
func TestLinodeObjectStorageQuotaGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageQuotaGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_quota_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_quota_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageQuotaGetToolSuccess(t *testing.T) {
	t.Parallel()

	quota := linode.ObjectStorageQuota{keyBetaID: objectStorageQuotaTestID, "quota": 250}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		if r.URL.Path != "/object-storage/quotas/"+objectStorageQuotaTestID {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/quotas/"+objectStorageQuotaTestID)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(quota); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageQuotaGetTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyObjectStorageQuotaID: objectStorageQuotaTestID})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, objectStorageQuotaTestID) {
		t.Errorf("textContent.Text does not contain %v", objectStorageQuotaTestID)
	}

	if !strings.Contains(textContent.Text, "250") {
		t.Errorf("textContent.Text does not contain %v", "250")
	}
}

func TestLinodeObjectStorageQuotaGetToolMissingQuotaId(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageQuotaGetTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageQuotaGetToolInvalidPathParameterValues(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageQuotaGetTool(cfg)

	t.Parallel()

	for _, invalid := range []string{"quota/extra", "quota?x=1", "quota#frag", "quota..extra", " quota"} {
		t.Run(invalid, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, map[string]any{keyObjectStorageQuotaID: invalid})

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, "obj_quota_id must not contain path separators") {
				t.Errorf("textContent.Text does not contain %v", "obj_quota_id must not contain path separators")
			}
		})
	}
}

func TestLinodeObjectStorageQuotaGetToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	incompleteCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
		},
	}
	_, _, incompleteHandler := tools.NewLinodeObjectStorageQuotaGetTool(incompleteCfg)

	req := createRequestWithArgs(t, map[string]any{keyObjectStorageQuotaID: objectStorageQuotaTestID})

	result, err := incompleteHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of bucket access settings retrieval.
func TestLinodeObjectStorageBucketAccessGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_access_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_access_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}
}

func TestLinodeObjectStorageBucketAccessGetToolSuccess(t *testing.T) {
	t.Parallel()

	access := linode.ObjectStorageBucketAccess{
		ACL:         aclPublicRead,
		CORSEnabled: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != objStorageAccessPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, objStorageAccessPath)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(access); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, aclPublicRead) {
		t.Errorf("textContent.Text does not contain %v", aclPublicRead)
	}

	if !strings.Contains(textContent.Text, boolStringTrue) {
		t.Errorf("textContent.Text does not contain %v", boolStringTrue)
	}
}

func TestLinodeObjectStorageBucketAccessGetToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}
		})
	}
}

func TestLinodeObjectStorageBucketAccessGetToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := tools.NewLinodeObjectStorageBucketAccessGetTool(emptyCfg)

	req := createRequestWithArgs(t, map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest})

	result, err := emptyHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage bucket creation.
func TestLinodeObjectStorageBucketCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props["acl"]; !ok {
		t.Errorf("props missing key %v", "acl")
	}

	if _, ok := props["cors_enabled"]; !ok {
		t.Errorf("props missing key %v", "cors_enabled")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageBucketCreateToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageBucketCreateToolLabelStartWithHyphen(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   "-my-bucket",
		keyRegion:  regionUSEast1,
		keyConfirm: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageBucketCreateToolSuccess(t *testing.T) {
	t.Parallel()

	bucket := linode.ObjectStorageBucket{
		Label:   bucketTest,
		Region:  regionUSEast1,
		Created: "2024-01-01T00:00:00",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(bucket); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, bucketTest) {
		t.Errorf("textContent.Text does not contain %v", bucketTest)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}
}

// End-to-end verification of object storage bucket deletion.
func TestLinodeObjectStorageBucketDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if capability != profiles.CapDestroy {
		t.Errorf("capability = %v, want %v", capability, profiles.CapDestroy)
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageBucketDeleteToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

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
			args:     map[string]any{keyLabel: bucketTest, keyConfirm: true, keyConfirmedDryRun: true},
			contains: errRegionRequired,
		},
		{
			name:     caseMissingLabel,
			args:     map[string]any{keyRegion: regionUSEast1, keyConfirm: true, keyConfirmedDryRun: true},
			contains: errLabelRequired,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageBucketDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucket {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucket)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

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
		keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "removed successfully") {
		t.Errorf("textContent.Text does not contain %v", "removed successfully")
	}
}

func TestLinodeObjectStorageCancelToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := tools.NewLinodeObjectStorageCancelTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_cancel" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_cancel")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if capability != profiles.CapWrite {
		t.Errorf("capability = %v, want %v", capability, profiles.CapWrite)
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	props := tool.InputSchema.Properties
	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}

	if _, ok := props["dry_run"]; !ok {
		t.Errorf("props missing key %v", "dry_run")
	}
}

func TestLinodeObjectStorageCancelToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageCancelTool(cfg)

	t.Parallel()

	tests := []struct {
		name string
		args map[string]any
	}{
		{name: caseMissingConfirm, args: map[string]any{}},
		{name: "false confirm", args: map[string]any{keyConfirm: false}},
		{name: "string confirm rejected", args: map[string]any{keyConfirm: boolStringTrue}},
		{name: "numeric confirm rejected", args: map[string]any{keyConfirm: 1}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, errConfirmEqualsTrue) {
				t.Errorf("error text %q does not contain %q", text.Text, errConfirmEqualsTrue)
			}
		})
	}
}

func TestLinodeObjectStorageCancelToolDryRunSkipsDestructiveCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageCancelTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyDryRun: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	if calls.Load() != int32(0) {
		t.Errorf("calls.Load() = %v, want %v", calls.Load(), int32(0))
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "POST") {
		t.Errorf("textContent.Text does not contain %v", "POST")
	}

	if !strings.Contains(textContent.Text, "/object-storage/cancel") {
		t.Errorf("textContent.Text does not contain %v", "/object-storage/cancel")
	}
}

func TestLinodeObjectStorageCancelToolClientError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/cancel" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/cancel")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageCancelTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "Failed to cancel Object Storage") {
		t.Errorf("error text %q does not contain %q", text.Text, "Failed to cancel Object Storage")
	}
}

func TestLinodeObjectStorageCancelToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/cancel" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/cancel")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		if r.URL.RawQuery != "" {
			t.Errorf("r.URL.RawQuery = %v, want empty", r.URL.RawQuery)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	srvCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, srvHandler := tools.NewLinodeObjectStorageCancelTool(srvCfg)

	req := createRequestWithArgs(t, map[string]any{keyConfirm: true})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "cancellation requested successfully") {
		t.Errorf("textContent.Text does not contain %v", "cancellation requested successfully")
	}
}

func TestLinodeObjectStorageBucketAccessAllowToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessAllowTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_access_allow" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_access_allow")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["acl"]; !ok {
		t.Errorf("props missing key %v", "acl")
	}

	if _, ok := props["cors_enabled"]; !ok {
		t.Errorf("props missing key %v", "cors_enabled")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageBucketAccessAllowToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketAccessAllowTool(cfg)

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
			name:     caseConfirmFalse,
			args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: false},
			contains: errConfirmEqualsTrue,
		},
		{
			name:     "confirm string rejected",
			args:     map[string]any{keyRegion: regionUSEast1, keyLabel: bucketTest, keyACL: aclPublicRead, keyConfirm: boolStringTrue},
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageBucketAccessAllowToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != objStorageAccessPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, objStorageAccessPath)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body[keyACL], aclPublicRead) {
			t.Errorf("body[keyACL] = %v, want %v", body[keyACL], aclPublicRead)
		}

		if !reflect.DeepEqual(body["cors_enabled"], true) {
			t.Errorf("got %v, want %v", body["cors_enabled"], true)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "applied successfully") {
		t.Errorf("textContent.Text does not contain %v", "applied successfully")
	}
}

func TestLinodeObjectStorageBucketAccessAllowToolMissingEnvironment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of bucket access settings update.
func TestLinodeObjectStorageBucketAccessUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_bucket_access_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_bucket_access_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["acl"]; !ok {
		t.Errorf("props missing key %v", "acl")
	}

	if _, ok := props["cors_enabled"]; !ok {
		t.Errorf("props missing key %v", "cors_enabled")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageBucketAccessUpdateToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageBucketAccessUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != objStorageAccessPath {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, objStorageAccessPath)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

func TestLinodeObjectStorageBucketAccessUpdateToolMissingEnvironment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage access key creation.
func TestLinodeObjectStorageKeyCreateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_key_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_key_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	if !strings.Contains(tool.Description, "WARNING") {
		t.Errorf("tool.Description does not contain %v", "WARNING")
	}

	if !strings.Contains(tool.Description, "secret_key") {
		t.Errorf("tool.Description does not contain %v", "secret_key")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["bucket_access"]; !ok {
		t.Errorf("props missing key %v", "bucket_access")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageKeyCreateToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyCreateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			for _, expected := range testCase.contains {
				if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, expected) {
					t.Errorf("error text %q does not contain %q", text.Text, expected)
				}
			}
		})
	}
}

func TestLinodeObjectStorageKeyCreateToolSuccess(t *testing.T) {
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
		if r.URL.Path != "/object-storage/keys" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/keys")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(key); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, keyNameTest) {
		t.Errorf("textContent.Text does not contain %v", keyNameTest)
	}

	if !strings.Contains(textContent.Text, "created successfully") {
		t.Errorf("textContent.Text does not contain %v", "created successfully")
	}

	if !strings.Contains(textContent.Text, "IMPORTANT") {
		t.Errorf("textContent.Text does not contain %v", "IMPORTANT")
	}

	if !strings.Contains(textContent.Text, "secret_key") {
		t.Errorf("textContent.Text does not contain %v", "secret_key")
	}

	if !strings.Contains(textContent.Text, "wJalrXUtnFEMI") {
		t.Errorf("textContent.Text does not contain %v", "wJalrXUtnFEMI")
	}
}

func TestLinodeObjectStorageKeyCreateToolMissingEnvironment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object storage access key update.
func TestLinodeObjectStorageKeyUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_key_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_key_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props["key_id"]; !ok {
		t.Errorf("props missing key %v", "key_id")
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["bucket_access"]; !ok {
		t.Errorf("props missing key %v", "bucket_access")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageKeyUpdateToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageKeyUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageKeys42 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageKeys42)
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "modified successfully") {
		t.Errorf("textContent.Text does not contain %v", "modified successfully")
	}
}

// End-to-end verification of object storage access key revocation.
func TestLinodeObjectStorageKeyDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_key_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_key_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props["key_id"]; !ok {
		t.Errorf("props missing key %v", "key_id")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageKeyDeleteToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

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
			args:     map[string]any{keyKeyID: float64(-1), keyConfirm: true, keyConfirmedDryRun: true},
			contains: "key_id is required",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			req := createRequestWithArgs(t, testCase.args)

			result, err := handler(t.Context(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, testCase.contains) {
				t.Errorf("error text %q does not contain %q", text.Text, testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageKeyDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageKeys42 {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageKeys42)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

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
		keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "revoked successfully") {
		t.Errorf("textContent.Text does not contain %v", "revoked successfully")
	}
}

func TestLinodeObjectStorageKeyDeleteToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := tools.NewLinodeObjectStorageKeyDeleteTool(emptyCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyKeyID:   float64(42),
		keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := emptyHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageKeyDeleteToolDryRunSchemaProperty(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, _ := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	t.Parallel()

	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeObjectStorageKeyDeleteToolDryRunReturnsPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	keyBody := `{"id":77,"label":"backups-key","access_key":"AKIA-EXAMPLE","limited":false}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != "/object-storage/keys/77" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/keys/77")
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(keyBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dryRunCfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, dryRunHandler := tools.NewLinodeObjectStorageKeyDeleteTool(dryRunCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyKeyID:  float64(77),
		keyDryRun: true,
	})

	result, err := dryRunHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_object_storage_key_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_key_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], "/object-storage/keys/77") {
		t.Errorf("got %v, want %v", would["path"], "/object-storage/keys/77")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeObjectStorageKeyDeleteToolDryRunStillRejectsNegativeKeyId(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	t.Parallel()

	// The pre-validation guard catches negative IDs with the
	// specific ErrKeyIDRequired message, independent of the
	// dry-run branch. Locks the wire-compat with the existing
	// invalid_key_id real-path test above.
	req := createRequestWithArgs(t, map[string]any{
		keyKeyID:  float64(-1),
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "key_id is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "key_id is required")
	}
}

// End-to-end verification of presigned URL generation.
func TestLinodeObjectStoragePresignedURLToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_presigned_url_create" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_presigned_url_create")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["name"]; !ok {
		t.Errorf("props missing key %v", "name")
	}

	if _, ok := props["method"]; !ok {
		t.Errorf("props missing key %v", "method")
	}

	if _, ok := props["expires_in"]; !ok {
		t.Errorf("props missing key %v", "expires_in")
	}
}

func TestLinodeObjectStoragePresignedURLToolMissingName(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyMethod: httpMethodGET,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "name") {
		t.Errorf("textContent.Text does not contain %v", "name")
	}
}

func TestLinodeObjectStoragePresignedURLToolInvalidMethod(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyName:   objectPhotoJPG,
		keyMethod: "DELETE",
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, httpMethodGET) {
		t.Errorf("textContent.Text does not contain %v", httpMethodGET)
	}

	if !strings.Contains(textContent.Text, "PUT") {
		t.Errorf("textContent.Text does not contain %v", "PUT")
	}
}

func TestLinodeObjectStoragePresignedURLToolInvalidExpiresIn(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStoragePresignedURLTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:    regionUSEast1,
		keyLabel:     bucketTest,
		keyName:      objectPhotoJPG,
		keyMethod:    httpMethodGET,
		"expires_in": float64(700000),
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "604800") {
		t.Errorf("textContent.Text does not contain %v", "604800")
	}
}

func TestLinodeObjectStoragePresignedURLToolSuccess(t *testing.T) {
	t.Parallel()

	resp := linode.PresignedURLResponse{
		URL: "https://my-bucket.us-east-1.linodeobjects.com/photo.jpg?signed=abc123",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/object-url" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/object-url")
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "signed=abc123") {
		t.Errorf("textContent.Text does not contain %v", "signed=abc123")
	}
}

func TestLinodeObjectStoragePresignedURLToolMissingEnvironment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of object ACL retrieval.
func TestLinodeObjectStorageObjectACLGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_object_acl_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_object_acl_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keySupportTicketRegion, monitorAlertDefinitionLabelParam, managedContactNameParam} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageObjectACLGetToolMissingName(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := tools.NewLinodeObjectStorageObjectACLGetTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "name") {
		t.Errorf("textContent.Text does not contain %v", "name")
	}
}

func TestLinodeObjectStorageObjectACLGetToolSuccess(t *testing.T) {
	t.Parallel()

	acl := linode.ObjectACL{
		ACL:    aclPublicRead,
		ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/object-acl" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/object-acl")
		}

		if r.URL.Query().Get("name") != objectPhotoJPG {
			t.Errorf("got %v, want %v", r.URL.Query().Get("name"), objectPhotoJPG)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(acl); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, aclPublicRead) {
		t.Errorf("textContent.Text does not contain %v", aclPublicRead)
	}
}

// End-to-end verification of object ACL update.
func TestLinodeObjectStorageObjectACLUpdateToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_object_acl_update" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_object_acl_update")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["name"]; !ok {
		t.Errorf("props missing key %v", "name")
	}

	if _, ok := props["acl"]; !ok {
		t.Errorf("props missing key %v", "acl")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageObjectACLUpdateToolValidation(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
	}}

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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if !result.IsError {
				t.Error("result.IsError = false, want true")
			}

			textContent, ok := result.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatal("ok = false, want true")
			}

			if !strings.Contains(textContent.Text, testCase.contains) {
				t.Errorf("textContent.Text does not contain %v", testCase.contains)
			}
		})
	}
}

func TestLinodeObjectStorageObjectACLUpdateToolSuccess(t *testing.T) {
	t.Parallel()

	resp := linode.ObjectACL{
		ACL:    aclPublicRead,
		ACLXML: "<AccessControlPolicy>...</AccessControlPolicy>",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/object-storage/buckets/us-east-1/my-bucket/object-acl" {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, "/object-storage/buckets/us-east-1/my-bucket/object-acl")
		}

		if r.Method != http.MethodPut {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPut)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, aclPublicRead) {
		t.Errorf("textContent.Text does not contain %v", aclPublicRead)
	}
}

// End-to-end verification of bucket SSL certificate status retrieval.
func TestLinodeObjectStorageSSLGetToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLGetTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_ssl_get" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_ssl_get")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keySupportTicketRegion, monitorAlertDefinitionLabelParam} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageSSLGetToolSuccess(t *testing.T) {
	t.Parallel()

	resp := linode.BucketSSL{
		SSL: true,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucketSsl {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucketSsl)
		}

		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, boolStringTrue) {
		t.Errorf("textContent.Text does not contain %v", boolStringTrue)
	}
}

func TestLinodeObjectStorageSSLGetToolMissingEnvironment(t *testing.T) {
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// End-to-end verification of bucket SSL certificate deletion.
func TestLinodeObjectStorageSSLDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_ssl_delete" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_ssl_delete")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageSSLDeleteToolConfirmRequired(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:  regionUSEast1,
		keyLabel:   bucketTest,
		keyConfirm: false,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
		t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
	}
}

func TestLinodeObjectStorageSSLDeleteToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucketSsl {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucketSsl)
		}

		if r.Method != http.MethodDelete {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodDelete)
		}

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
		keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := srvHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "SSL certificate deleted") {
		t.Errorf("textContent.Text does not contain %v", "SSL certificate deleted")
	}
}

func TestLinodeObjectStorageSSLDeleteToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := tools.NewLinodeObjectStorageSSLDeleteTool(emptyCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:  regionUSEast1,
		keyLabel:   bucketTest,
		keyConfirm: true, keyConfirmedDryRun: true,
	})

	result, err := emptyHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

// Dry-run coverage for bucket delete. Kept in a sibling function so
// the main test's subtest count stays under maintidx's threshold.
func TestLinodeObjectStorageBucketDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeObjectStorageBucketDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeObjectStorageBucketDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	bucketBody := `{"label":"my-bucket","region":"us-east-1","size":1024,"objects":3}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucket {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucket)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(bucketBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_object_storage_bucket_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_bucket_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcObjectStorageBucketsUsEast1MyBucket) {
		t.Errorf("got %v, want %v", would["path"], tcObjectStorageBucketsUsEast1MyBucket)
	}

	state, stateIsObject := body["current_state"].(map[string]any)
	if !stateIsObject {
		t.Fatal("stateIsObject = false, want true")
	}

	if !reflect.DeepEqual(state[keyLabel], "my-bucket") {
		t.Errorf("state[keyLabel] = %v, want %v", state[keyLabel], "my-bucket")
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeObjectStorageBucketDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"label":"my-bucket","region":"us-east-1"}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeObjectStorageBucketDeleteToolDryRunDryRunStillValidatesRegion(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "region is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "region is required")
	}
}

func TestLinodeObjectStorageBucketDeleteToolDryRunDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeObjectStorageBucketDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

// Dry-run coverage for SSL certificate delete. Kept in a sibling function
// so the main test's subtest count stays under maintidx's threshold.
func TestLinodeObjectStorageSSLDeleteToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := tools.NewLinodeObjectStorageSSLDeleteTool(&config.Config{})
	if _, ok := tool.InputSchema.Properties["dry_run"]; !ok {
		t.Errorf("tool.InputSchema.Properties missing key %v", "dry_run")
	}
}

func TestLinodeObjectStorageSSLDeleteToolDryRunPreviewWithoutMutating(t *testing.T) {
	t.Parallel()

	var methodsSeen []string

	sslBody := `{"ssl":true}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsSeen = append(methodsSeen, r.Method)
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucketSsl {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucketSsl)
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(sslBody))

			return
		}

		t.Errorf("dry_run must NOT issue any non-GET request; got %s", r.Method)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	textContent, isText := result.Content[0].(mcp.TextContent)
	if !isText {
		t.Fatal("isText = false, want true")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body[keyDryRun], true) {
		t.Errorf("body[keyDryRun] = %v, want %v", body[keyDryRun], true)
	}

	if !reflect.DeepEqual(body["tool"], "linode_object_storage_ssl_delete") {
		t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_ssl_delete")
	}

	would, isWouldObject := body["would_execute"].(map[string]any)
	if !isWouldObject {
		t.Fatal("isWouldObject = false, want true")
	}

	if !reflect.DeepEqual(would["method"], "DELETE") {
		t.Errorf("got %v, want %v", would["method"], "DELETE")
	}

	if !reflect.DeepEqual(would["path"], tcObjectStorageBucketsUsEast1MyBucketSsl) {
		t.Errorf("got %v, want %v", would["path"], tcObjectStorageBucketsUsEast1MyBucketSsl)
	}

	if !reflect.DeepEqual(methodsSeen, []string{http.MethodGet}) {
		t.Errorf("methodsSeen = %v, want %v", methodsSeen, []string{http.MethodGet})
	}
}

func TestLinodeObjectStorageSSLDeleteToolDryRunDoesNotRequireConfirm(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodGet)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ssl":true}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Environments: map[string]config.EnvironmentConfig{
		envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
	}}
	_, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}
}

func TestLinodeObjectStorageSSLDeleteToolDryRunDryRunStillValidatesRegion(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyLabel:  bucketTest,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "region is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "region is required")
	}
}

func TestLinodeObjectStorageSSLDeleteToolDryRunDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := tools.NewLinodeObjectStorageSSLDeleteTool(&config.Config{})
	req := createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyDryRun: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

// End-to-end verification of bucket SSL certificate upload.
func TestLinodeObjectStorageSSLUploadToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := tools.NewLinodeObjectStorageSSLUploadTool(cfg)

	t.Parallel()

	if tool.Name != "linode_object_storage_ssl_upload" {
		t.Errorf("tool.Name = %v, want %v", tool.Name, "linode_object_storage_ssl_upload")
	}

	if tool.Description == "" {
		t.Error("tool.Description is empty")
	}

	if handler == nil {
		t.Fatal("handler is nil")
	}

	props := tool.InputSchema.Properties
	if _, ok := props[keySupportTicketRegion]; !ok {
		t.Errorf("props missing key %v", keySupportTicketRegion)
	}

	if _, ok := props[monitorAlertDefinitionLabelParam]; !ok {
		t.Errorf("props missing key %v", managedServiceLabelParam)
	}

	if _, ok := props["certificate"]; !ok {
		t.Errorf("props missing key %v", "certificate")
	}

	if _, ok := props["private_key"]; !ok {
		t.Errorf("props missing key %v", "private_key")
	}

	if _, ok := props["confirm"]; !ok {
		t.Errorf("props missing key %v", "confirm")
	}
}

func TestLinodeObjectStorageSSLUploadToolConfirmRequired(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := tools.NewLinodeObjectStorageSSLUploadTool(cfg)

	t.Parallel()

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast1,
		keyLabel:       bucketTest,
		keyCertificate: "test-cert",
		keyPrivateKey:  testKeyLabel,
		keyConfirm:     false,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, errConfirmEqualsTrue) {
		t.Errorf("textContent.Text does not contain %v", errConfirmEqualsTrue)
	}
}

func TestLinodeObjectStorageSSLUploadToolSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != tcObjectStorageBucketsUsEast1MyBucketSsl {
			t.Errorf("r.URL.Path = %v, want %v", r.URL.Path, tcObjectStorageBucketsUsEast1MyBucketSsl)
		}

		if r.Method != http.MethodPost {
			t.Errorf("r.Method = %v, want %v", r.Method, http.MethodPost)
		}

		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(linode.BucketSSL{SSL: true}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "SSL certificate uploaded") {
		t.Errorf("textContent.Text does not contain %v", "SSL certificate uploaded")
	}
}

func TestLinodeObjectStorageSSLUploadToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := tools.NewLinodeObjectStorageSSLUploadTool(emptyCfg)

	req := createRequestWithArgs(t, map[string]any{
		keyRegion:      regionUSEast1,
		keyLabel:       bucketTest,
		keyCertificate: "test-cert",
		keyPrivateKey:  testKeyLabel,
		keyConfirm:     true,
	})

	result, err := emptyHandler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}
}

func TestLinodeObjectStorageSSLUploadToolApiErrorPropagated(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"}); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
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
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, "Failed to upload SSL certificate") {
		t.Errorf("textContent.Text does not contain %v", "Failed to upload SSL certificate")
	}
}

func TestLinodeObjectStorageSSLUploadToolTraversalCase(t *testing.T) {
	t.Parallel()

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

				if err := json.NewEncoder(w).Encode(linode.BucketSSL{SSL: true}); err != nil {
					t.Errorf("unexpected error: %v", err)
				}
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
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("result is nil")
			}

			if result.IsError {
				t.Error("result.IsError = true, want false")
			}
		})
	}
}
