package tools_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/tools"
)

const objStorageAccessPath = "/object-storage/buckets/us-east-1/my-bucket/access"

func TestLinodeObjectStorageBucketCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageBucketCreateTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  bucketTest,
			keyRegion: regionUSEast1,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_bucket_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "POST", would["method"])
		checkEqual(t, "/object-storage/buckets", would["path"])
		expectNil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "create surfaces the new-bucket side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContains(t, effect, bucketTest, "side effect should name the new bucket")

		warnings, _ := body["warnings"].([]any)
		expectLen(t, warnings, 1, "create warns that billing starts immediately")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageBucketCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeObjectStorageBucketAccessAllowToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageBucketAccessAllowTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without applying", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, objStorageAccessPath, linode.ObjectStorageBucketAccess{ACL: aclPrivate})
		_, _, handler := tools.NewLinodeObjectStorageBucketAccessAllowTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_bucket_access_allow", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "POST", would["method"])
		checkEqual(t, objStorageAccessPath, would["path"])
		checkEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageBucketAccessAllowTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeObjectStorageBucketAccessUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageBucketAccessUpdateTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, objStorageAccessPath, linode.ObjectStorageBucketAccess{ACL: aclPrivate})
		_, _, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_bucket_access_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "PUT", would["method"])
		checkEqual(t, objStorageAccessPath, would["path"])
		checkEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "ACL change surfaces one side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContains(t, effect, aclPrivate, "side effect should name the target ACL")
	})

	t.Run("still validates region", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageBucketAccessUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  bucketTest,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "region is required")
	})
}

func TestLinodeObjectStorageKeyCreateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageKeyCreateTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without creating", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageKeyCreateTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  "my-key",
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_key_create", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "POST", would["method"])
		checkEqual(t, "/object-storage/keys", would["path"])
		expectNil(t, body["current_state"], "create has no existing resource to preview")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "create surfaces the new-key side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContains(t, effect, "my-key", "side effect should name the new key")

		warnings, _ := body["warnings"].([]any)
		expectLen(t, warnings, 1, "create warns the secret is shown only once")
	})

	t.Run("still validates label", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageKeyCreateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{keyDryRun: true}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "label is required")
	})
}

func TestLinodeObjectStorageKeyUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageKeyUpdateTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating, fetches key not secret", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/object-storage/keys/77",
			linode.ObjectStorageKey{ID: 77, Label: "my-key"})
		_, _, handler := tools.NewLinodeObjectStorageKeyUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyKeyID:  float64(77),
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_key_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "PUT", would["method"])
		checkEqual(t, "/object-storage/keys/77", would["path"])
		checkEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "renaming the key surfaces the label-change side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContains(t, effect, testRenamedLabel, "side effect should name the new label")
	})

	t.Run("still validates key_id", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageKeyUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyLabel:  testRenamedLabel,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "key_id is required")
	})
}

func TestLinodeObjectStorageObjectACLUpdateToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageObjectACLUpdateTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without updating", func(t *testing.T) {
		t.Parallel()

		cfg, methods := dryRunGetStateServer(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl",
			linode.ObjectACL{ACL: aclPrivate})
		_, _, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(cfg)

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyName:   "object.txt",
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(dryRunResultText(t, result)), &body))
		checkEqual(t, "linode_object_storage_object_acl_update", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "PUT", would["method"])
		checkEqual(t, "/object-storage/buckets/us-east-1/my-bucket/object-acl", would["path"])
		checkEqual(t, []string{http.MethodGet}, *methods, "dry_run must only read state via GET")

		sideEffects, _ := body["side_effects"].([]any)
		expectLen(t, sideEffects, 1, "ACL change surfaces one side effect")

		effect, gotString := sideEffects[0].(string)
		expectTrue(t, gotString)
		expectContains(t, effect, aclPrivate, "side effect should name the target ACL")
	})

	t.Run("still validates name", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageObjectACLUpdateTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion: regionUSEast1,
			keyLabel:  bucketTest,
			keyACL:    aclPrivate,
			keyDryRun: true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "name (object key) is required")
	})
}

func TestLinodeObjectStorageSSLUploadToolDryRun(t *testing.T) {
	t.Parallel()

	t.Run("schema advertises dry_run", func(t *testing.T) {
		t.Parallel()

		tool, _, _ := tools.NewLinodeObjectStorageSSLUploadTool(&config.Config{})
		expectContains(t, tool.InputSchema.Properties, keyDryRun)
	})

	t.Run("preview without uploading, no key echoed", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageSSLUploadTool(dryRunNoCallServer(t))

		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "cert-pem",
			keyPrivateKey:  "key-pem",
			keyDryRun:      true,
		}))
		expectNoError(t, err)
		expectFalse(t, result.IsError)

		text := dryRunResultText(t, result)

		var body map[string]any
		expectNoError(t, json.Unmarshal([]byte(text), &body))
		checkEqual(t, "linode_object_storage_ssl_upload", body["tool"])

		would, _ := body["would_execute"].(map[string]any)
		checkEqual(t, "POST", would["method"])
		checkEqual(t, "/object-storage/buckets/us-east-1/my-bucket/ssl", would["path"])
		expectNil(t, body["current_state"], "upload has no existing resource to preview")
		expectNotContains(t, text, "key-pem", "dry_run preview must not echo the private key")
	})

	t.Run("still validates private_key", func(t *testing.T) {
		t.Parallel()

		_, _, handler := tools.NewLinodeObjectStorageSSLUploadTool(&config.Config{})
		result, err := handler(t.Context(), createRequestWithArgs(t, map[string]any{
			keyRegion:      regionUSEast1,
			keyLabel:       bucketTest,
			keyCertificate: "cert-pem",
			keyDryRun:      true,
		}))
		expectNoError(t, err)
		expectTrue(t, result.IsError)
		assertErrorContains(t, result, "private_key is required")
	})
}
