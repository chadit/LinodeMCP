package tools_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/gentools"
	"github.com/chadit/LinodeMCP/go/internal/linode"
)

const objStorageAccessPath = "/object-storage/buckets/us-east-1/my-bucket/access"

func TestLinodeObjectStorageBucketCreateToolDryRunSchemaAdvertisesDryRun(t *testing.T) {
	t.Parallel()

	tool, _, _ := gentools.NewLinodeObjectStorageBucketCreateTool(&config.Config{})
	if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
		t.Errorf("RawInputSchema missing key %v", keyDryRun)
	}
}

func TestLinodeObjectStorageBucketCreateToolDryRunPreviewWithoutCreating(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(dryRunNoCallServer(t))

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyLabel:  bucketTest,
		keyRegion: regionUSEast1,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Fatal("result.IsError = true, want false")
	}

	var body map[string]any
	if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(body["tool"], "linode_object_storage_bucket_create") {
		t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_bucket_create")
	}

	would, _ := body["would_execute"].(map[string]any)
	if !reflect.DeepEqual(would["method"], "POST") {
		t.Errorf("got %v, want %v", would["method"], "POST")
	}

	if !reflect.DeepEqual(would["path"], "/object-storage/buckets") {
		t.Errorf("got %v, want %v", would["path"], "/object-storage/buckets")
	}

	if body["current_state"] != nil {
		t.Errorf("value = %v, want nil", body["current_state"])
	}

	sideEffects, _ := body["side_effects"].([]any)
	if len(sideEffects) != 1 {
		t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
	}

	effect, gotString := sideEffects[0].(string)
	if !gotString {
		t.Fatal("gotString = false, want true")
	}

	if !strings.Contains(effect, bucketTest) {
		t.Errorf("effect does not contain %v", bucketTest)
	}

	warnings, _ := body["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want %d", len(warnings), 1)
	}
}

func TestLinodeObjectStorageBucketCreateToolDryRunStillValidatesLabel(t *testing.T) {
	t.Parallel()

	_, _, handler := gentools.NewLinodeObjectStorageBucketCreateTool(&config.Config{})

	result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
		keyRegion: regionUSEast1,
		keyDryRun: true,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsError {
		t.Error("result.IsError = false, want true")
	}

	if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "label is required") {
		t.Errorf("error text %q does not contain %q", text.Text, "label is required")
	}
}

func TestLinodeObjectStorageObjectACLUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeObjectStorageObjectACLUpdateTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl",
			linode.ObjectACL{ACL: aclPrivate})
		_, _, handler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   "object.txt",
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		var body map[string]any
		if err := json.Unmarshal([]byte(dryRunResultText(t, result)), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_object_storage_object_acl_update") {
			t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_object_acl_update")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "PUT") {
			t.Errorf("got %v, want %v", would["method"], "PUT")
		}

		if !reflect.DeepEqual(would["path"], "/object-storage/buckets/us-east-1/my-bucket/object-acl") {
			t.Errorf("got %v, want %v", would["path"], "/object-storage/buckets/us-east-1/my-bucket/object-acl")
		}

		if !reflect.DeepEqual(*methods, []string{http.MethodGet}) {
			t.Errorf("*methods = %v, want %v", *methods, []string{http.MethodGet})
		}

		sideEffects, _ := body["side_effects"].([]any)
		if len(sideEffects) != 1 {
			t.Fatalf("len(sideEffects) = %d, want %d", len(sideEffects), 1)
		}

		effect, gotString := sideEffects[0].(string)
		if !gotString {
			t.Fatal("gotString = false, want true")
		}

		if !strings.Contains(effect, aclPrivate) {
			t.Errorf("effect does not contain %v", aclPrivate)
		}
	})

	t.Run("still validates name", func(t *testing.T) {
		t.Parallel()

		_, _, handler := gentools.NewLinodeObjectStorageObjectACLUpdateTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "name (object key) is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "name (object key) is required")
		}
	})
}

func TestLinodeObjectStorageSSLUploadToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := gentools.NewLinodeObjectStorageSSLUploadTool(&config.Config{})
		if !strings.Contains(string(tool.RawInputSchema), keyDryRun) {
			t.Errorf("RawInputSchema missing key %v", keyDryRun)
		}
	})

	t.Run("preview without uploading, no key echoed", func(t *testing.T) {
		t.Parallel()

		_, _, handler := gentools.NewLinodeObjectStorageSSLUploadTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "cert-pem",
			keyPrivateKey:  "key-pem",
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.IsError {
			t.Fatal("result.IsError = true, want false")
		}

		text := dryRunResultText(t, result)

		var body map[string]any
		if err := json.Unmarshal([]byte(text), &body); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !reflect.DeepEqual(body["tool"], "linode_object_storage_ssl_upload") {
			t.Errorf("got %v, want %v", body["tool"], "linode_object_storage_ssl_upload")
		}

		would, _ := body["would_execute"].(map[string]any)
		if !reflect.DeepEqual(would["method"], "POST") {
			t.Errorf("got %v, want %v", would["method"], "POST")
		}

		if !reflect.DeepEqual(would["path"], tcObjectStorageBucketsUsEast1MyBucketSsl) {
			t.Errorf("got %v, want %v", would["path"], tcObjectStorageBucketsUsEast1MyBucketSsl)
		}

		if body["current_state"] != nil {
			t.Errorf("value = %v, want nil", body["current_state"])
		}

		if strings.Contains(text, "key-pem") {
			t.Errorf("text should not contain %v", "key-pem")
		}
	})

	t.Run("still validates private_key", func(t *testing.T) {
		t.Parallel()

		_, _, handler := gentools.NewLinodeObjectStorageSSLUploadTool(&config.Config{})

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "cert-pem",
			keyDryRun:      true,
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.IsError {
			t.Error("result.IsError = false, want true")
		}

		if text, ok := result.Content[0].(mcp.TextContent); !ok || !strings.Contains(text.Text, "private_key is required") {
			t.Errorf("error text %q does not contain %q", text.Text, "private_key is required")
		}
	})
}
