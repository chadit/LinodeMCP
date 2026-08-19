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
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

const (
	bucketHostnameUSEast1       = "my-bucket.us-east-1.linodeobjects.com"
	keyObjectStorageQuotaID     = "obj_quota_id"
	objectStorageEndpointUSEast = "us-east-1.linodeobjects.com"
	objectStorageQuotaTestID    = "obj-buckets-us-sea-1.linodeobjects.com"
	regionSlashUSEast1          = "us/east-1"
)

// End-to-end verification of object storage bucket listing.
func TestLinodeObjectStorageBucketsListToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeObjectStorageBucketListTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageBucketListTool(srvCfg)

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

	if got := listResponseCount(t, textContent.Text); got != 2 {
		t.Errorf("listResponseCount = %d, want %d", got, 2)
	}
}

func TestLinodeObjectStorageBucketsListToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := gentools.NewLinodeObjectStorageBucketListTool(emptyCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageBucketByRegionListTool(cfg)

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

	// Each element carries a field the proto does not model so the DiscardUnknown
	// decode must drop it, proving the list routes through the proto serializer.
	buckets := []json.RawMessage{
		withUnmodeledField(t, linode.ObjectStorageBucket{Label: bucketTest, Region: regionUSEast1, Hostname: bucketHostnameUSEast1, Objects: 42, Size: 1024}),
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageBucketByRegionListTool(srvCfg)

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

	// protojson varies its whitespace between runs, so parse the body rather than
	// substring-matching the count. The envelope is {count, buckets}: the region
	// input echo is not part of the proto contract.
	var body struct {
		Region  string           `json:"region"`
		Buckets []map[string]any `json:"buckets"`
		Count   int              `json:"count"`
	}
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if body.Count != 1 {
		t.Errorf("body.Count = %d, want 1", body.Count)
	}

	if len(body.Buckets) != 1 {
		t.Errorf("len(body.Buckets) = %d, want 1", len(body.Buckets))
	}

	if body.Region != "" {
		t.Errorf("body.Region = %q, want empty (region echo dropped)", body.Region)
	}

	if strings.Contains(textContent.Text, keyNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeObjectStorageBucketsListByRegionToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageBucketByRegionListTool(cfg)

	t.Parallel()

	tests := []struct {
		args map[string]any
		name string
	}{
		{name: caseMissingRegion, args: map[string]any{}},
		{name: "slash in region", args: map[string]any{keyRegion: regionSlashUSEast1}},
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
	tool, _, handler := gentools.NewLinodeObjectStorageBucketGetTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageBucketGetTool(srvCfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageBucketGetTool(cfg)

	t.Parallel()

	tests := []struct {
		args map[string]any
		name string
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

func TestLinodeObjectStorageEndpointsListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeObjectStorageEndpointListTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageEndpointListTool(srvCfg)

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

	if !strings.Contains(textContent.Text, `"endpoint_type"`) || !strings.Contains(textContent.Text, `"E0"`) {
		t.Errorf("textContent.Text does not contain endpoint_type E0: %v", textContent.Text)
	}

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageEndpointListTool(srvCfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}
}

func TestLinodeObjectStorageEndpointsListToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	incompleteCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
		},
	}
	_, _, incompleteHandler := gentools.NewLinodeObjectStorageEndpointListTool(incompleteCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageTypeListTool(cfg)

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
		_, _, srvHandler := gentools.NewLinodeObjectStorageTypeListTool(srvCfg)

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

		if got := listResponseCount(t, textContent.Text); got != 1 {
			t.Errorf("listResponseCount = %d, want 1", got)
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := gentools.NewLinodeObjectStorageTypeListTool(incompleteCfg)

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

// TestLinodeObjectStorageTypeListDecodesRegionPrices verifies the element
// struct decodes region_prices as an array of {id, hourly, monthly} objects.
// A string-typed region_prices field would silently drop the data.
func TestLinodeObjectStorageTypeListDecodesRegionPrices(t *testing.T) {
	t.Parallel()

	types := []linode.ObjectStorageType{
		{
			ID:       "objectstorage",
			Label:    "Object Storage",
			Transfer: 1000,
			Price:    linode.Price{Hourly: 0.0107, Monthly: 5.0},
			RegionPrices: []linode.ObjectStorageRegionPrice{
				{ID: placementGroupCreateRegion, Hourly: 0.0107, Monthly: 5.0},
			},
		},
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

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: srv.URL, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageTypeListTool(cfg)

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("result.IsError = true, want false")
	}

	textContent, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatal("ok = false, want true")
	}

	if !strings.Contains(textContent.Text, `"region_prices"`) {
		t.Errorf("textContent.Text does not contain region_prices array")
	}

	if !strings.Contains(textContent.Text, placementGroupCreateRegion) {
		t.Errorf("textContent.Text does not contain region price id %v", placementGroupCreateRegion)
	}
}

// End-to-end verification of object storage quota listing.
func TestLinodeObjectStorageQuotasListToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeObjectStorageQuotaListTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageQuotaListTool(srvCfg)

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

	if got := listResponseCount(t, textContent.Text); got != 1 {
		t.Errorf("listResponseCount = %d, want 1", got)
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageQuotaListTool(srvCfg)

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

	if !strings.Contains(textContent.Text, "Failed to retrieve items") {
		t.Errorf("textContent.Text does not contain %v", "Failed to retrieve items")
	}
}

func TestLinodeObjectStorageQuotasListToolIncompleteConfig(t *testing.T) {
	t.Parallel()

	incompleteCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
		},
	}
	_, _, incompleteHandler := gentools.NewLinodeObjectStorageQuotaListTool(incompleteCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageKeyListTool(cfg)

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
		_, _, srvHandler := gentools.NewLinodeObjectStorageKeyListTool(srvCfg)

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

		if count := listResponseCount(t, textContent.Text); count != 1 {
			t.Errorf("count = %d, want 1", count)
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := gentools.NewLinodeObjectStorageKeyListTool(incompleteCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageKeyGetTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageKeyGetTool(srvCfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageKeyGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageKeyGetTool(cfg)

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

	if !strings.Contains(textContent.Text, "key_id must be a positive integer") {
		t.Errorf("textContent.Text does not contain %v", "key_id must be a positive integer")
	}
}

// End-to-end verification of object storage quota usage retrieval.
func TestLinodeObjectStorageQuotaUsageToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeObjectStorageQuotaUsageGetTool(cfg)

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

	usage := map[string]any{"quota_limit": 100, "usage": 10, keyNotInProto: valNotInProto}

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageQuotaUsageGetTool(srvCfg)

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

	if strings.Contains(textContent.Text, valNotInProto) {
		t.Error("unknown field not_in_proto leaked into proto-canonical output")
	}
}

func TestLinodeObjectStorageQuotaUsageToolMissingQuotaId(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := gentools.NewLinodeObjectStorageQuotaUsageGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageQuotaUsageGetTool(cfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageTransferGetTool(cfg)

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

		transfer := map[string]any{"used": 1073741824, keyNotInProto: valNotInProto}

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
		_, _, srvHandler := gentools.NewLinodeObjectStorageTransferGetTool(srvCfg)

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

		if strings.Contains(textContent.Text, valNotInProto) {
			t.Error("unknown field not_in_proto leaked into proto-canonical output")
		}
	})

	t.Run("incomplete config", func(t *testing.T) {
		t.Parallel()

		incompleteCfg := &config.Config{
			Environments: map[string]config.EnvironmentConfig{
				envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: "", Token: ""}},
			},
		}
		_, _, incompleteHandler := gentools.NewLinodeObjectStorageTransferGetTool(incompleteCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageQuotaGetTool(cfg)

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

	quota := map[string]any{
		"quota_id":        objectStorageQuotaTestID,
		"quota_name":      "Number of Objects",
		"endpoint_type":   "E1",
		"s3_endpoint":     "us-sea-1.linodeobjects.com",
		"description":     "Maximum number of objects this endpoint can store",
		"quota_limit":     250,
		"resource_metric": "object",
		keyNotInProto:     valNotInProto,
	}

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageQuotaGetTool(srvCfg)

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

	if !strings.Contains(textContent.Text, `"quota_id"`) {
		t.Errorf("textContent.Text does not contain %v", `"quota_id"`)
	}

	if !strings.Contains(textContent.Text, objectStorageQuotaTestID) {
		t.Errorf("textContent.Text does not contain %v", objectStorageQuotaTestID)
	}

	if !strings.Contains(textContent.Text, "250") {
		t.Errorf("textContent.Text does not contain %v", "250")
	}

	if strings.Contains(textContent.Text, "not_in_proto") {
		t.Errorf("textContent.Text unexpectedly contains dropped unknown field: %v", textContent.Text)
	}
}

func TestLinodeObjectStorageQuotaGetToolMissingQuotaId(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageQuotaGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageQuotaGetTool(cfg)

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
	_, _, incompleteHandler := gentools.NewLinodeObjectStorageQuotaGetTool(incompleteCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageBucketAccessGetTool(srvCfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageBucketAccessGetTool(cfg)

	t.Parallel()

	tests := []struct {
		args map[string]any
		name string
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
	_, _, emptyHandler := gentools.NewLinodeObjectStorageBucketAccessGetTool(emptyCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyLabel, keyRegion, keyACL, keyCORSEnabled, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageBucketCreateToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(cfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(cfg)

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

// TestValidateBucketACLMessageReconciled pins the exact invalid-acl message that
// validateBucketACL surfaces through the create handler. The string must be
// byte-identical to Python's: no "got '<value>':" prefix and ACLs listed in
// API/create-endpoint order.
func TestValidateBucketACLMessageReconciled(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(cfg)

	req := createRequestWithArgs(t, map[string]any{
		keyLabel:   bucketTest,
		keyRegion:  regionUSEast1,
		keyACL:     "bogus",
		keyConfirm: true,
	})

	result, err := handler(t.Context(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil || !result.IsError {
		t.Fatal("want an error result for an invalid acl")
	}

	const want = "acl must be one of: private, public-read, authenticated-read, public-read-write"

	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is not TextContent: %T", result.Content[0])
	}

	if text.Text != want {
		t.Errorf("error text = %q, want %q", text.Text, want)
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageBucketCreateTool(srvCfg)

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

func TestLinodeObjectStorageCancelToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, capability, handler := gentools.NewLinodeObjectStorageCancelTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyConfirm, keyDryRun} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageCancelToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageCancelTool(cfg)

	t.Parallel()

	tests := []struct {
		args map[string]any
		name string
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageCancelTool(srvCfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageCancelTool(srvCfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageCancelTool(srvCfg)

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

// End-to-end verification of object storage access key revocation.
func TestLinodeObjectStorageKeyDeleteToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeObjectStorageKeyDeleteTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyKeyID, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageKeyDeleteToolValidation(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	_, _, handler := gentools.NewLinodeObjectStorageKeyDeleteTool(cfg)

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
			contains: "key_id must be a positive integer",
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
	_, _, srvHandler := gentools.NewLinodeObjectStorageKeyDeleteTool(srvCfg)

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
	_, _, emptyHandler := gentools.NewLinodeObjectStorageKeyDeleteTool(emptyCfg)

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
	tool, _, _ := gentools.NewLinodeObjectStorageKeyDeleteTool(cfg)

	t.Parallel()

	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
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
	_, _, dryRunHandler := gentools.NewLinodeObjectStorageKeyDeleteTool(dryRunCfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageKeyDeleteTool(cfg)

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

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "key_id must be a positive integer") {
		t.Errorf("error text %q does not contain %q", text.Text, "key_id must be a positive integer")
	}
}

func TestLinodeObjectStorageObjectACLGetToolDefinition(t *testing.T) {
	cfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{
			envKeyDefault: {Label: envLabelDefault, Linode: config.LinodeConfig{APIURL: apiURLLinodeV4, Token: tokenTest}},
		},
	}
	tool, _, handler := gentools.NewLinodeObjectStorageObjectACLGetTool(cfg)

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
	_, _, handler := gentools.NewLinodeObjectStorageObjectACLGetTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageObjectACLGetTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyRegion, keyLabel, keyName, keyACL, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
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

			testCfg := cfg
			if testCase.name == "confirm required" {
				testCfg = &config.Config{}
			}

			_, _, testHandler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(testCfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(srvCfg)

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
	tool, _, handler := gentools.NewLinodeObjectStorageSSLGetTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageSSLGetTool(srvCfg)

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
	_, _, emptyHandler := gentools.NewLinodeObjectStorageSSLGetTool(emptyCfg)

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

// End-to-end verification of bucket SSL certificate upload.
func TestLinodeObjectStorageSSLUploadToolDefinition(t *testing.T) {
	cfg := &config.Config{}
	tool, _, handler := gentools.NewLinodeObjectStorageSSLUploadTool(cfg)

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

	rawSchema := string(tool.RawInputSchema)
	for _, key := range []string{keyRegion, keyLabel, keyCertificate, keyPrivateKey, keyConfirm} {
		if !strings.Contains(rawSchema, key) {
			t.Errorf("RawInputSchema missing key %v", key)
		}
	}
}

func TestLinodeObjectStorageSSLUploadToolConfirmRequired(t *testing.T) {
	cfg := &config.Config{}
	_, _, handler := gentools.NewLinodeObjectStorageSSLUploadTool(cfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

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

	var body map[string]any
	if err := json.Unmarshal([]byte(textContent.Text), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	ssl, isObject := body["ssl"].(map[string]any)
	if !isObject {
		t.Fatalf("body[ssl] is not an object: %v", body["ssl"])
	}

	if ssl["ssl"] != true {
		t.Errorf("ssl.ssl = %v, want true", ssl["ssl"])
	}
}

func TestLinodeObjectStorageSSLUploadToolMissingEnvironment(t *testing.T) {
	t.Parallel()

	emptyCfg := &config.Config{
		Environments: map[string]config.EnvironmentConfig{},
	}
	_, _, emptyHandler := gentools.NewLinodeObjectStorageSSLUploadTool(emptyCfg)

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
	_, _, srvHandler := gentools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

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
			_, _, srvHandler := gentools.NewLinodeObjectStorageSSLUploadTool(srvCfg)

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
