package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

// NewLinodeObjectStorageBucketCreateTool creates a tool for creating an Object Storage bucket.
func NewLinodeObjectStorageBucketCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_bucket_create",
		"Creates a new Object Storage bucket. WARNING: Billing starts immediately.",
		[]mcp.ToolOption{
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Bucket label (3-63 chars, lowercase alphanumeric and hyphens, must start/end with alphanumeric)")),
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Object Storage region for the bucket (e.g. us-east-1).")),
			mcp.WithString("acl",
				mcp.Description("Access control: private, public-read, authenticated-read, or public-read-write (default: private)")),
			mcp.WithBoolean("cors_enabled",
				mcp.Description("Whether to enable CORS on the bucket (default: true)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm bucket creation. This operation incurs billing charges. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageBucketCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateBucketCreateArgs validates the bucket create args, returning an
// error message or "". Shared by the real create path and the dry-run preview.
func validateBucketCreateArgs(label, region, acl string) string {
	if err := validateBucketLabel(label); err != nil {
		return err.Error()
	}

	if region == "" {
		return ErrBucketRegionRequired.Error()
	}

	if acl != "" {
		if err := validateBucketACL(acl); err != nil {
			return err.Error()
		}
	}

	return ""
}

func handleObjectStorageBucketCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	region := request.GetString("region", "")
	acl := request.GetString("acl", "")

	if IsDryRun(request) {
		if msg := validateBucketCreateArgs(label, region, acl); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_object_storage_bucket_create", httpMethodPost, "/object-storage/buckets", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return bucketCreateSideEffects(ctx, label, region)
			})
	}

	if result := RequireConfirm(request, "This operation creates a billable resource. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateBucketCreateArgs(label, region, acl); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateObjectStorageBucketRequest{
		Label:  label,
		Region: region,
		ACL:    acl,
	}

	if _, ok := request.GetArguments()["cors_enabled"]; ok {
		corsEnabled := request.GetBool("cors_enabled", false)
		req.CORSEnabled = &corsEnabled
	}

	bucket, err := client.CreateObjectStorageBucket(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create bucket: %v", err)), nil
	}

	response := struct {
		Message string                      `json:"message"`
		Bucket  *linode.ObjectStorageBucket `json:"bucket"`
	}{
		Message: fmt.Sprintf("Bucket '%s' created successfully in %s", bucket.Label, bucket.Region),
		Bucket:  bucket,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageBucketDeleteTool creates a tool for deleting an Object Storage bucket.
func NewLinodeObjectStorageBucketDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_bucket_delete",
		"Deletes an Object Storage bucket. WARNING: This is irreversible. All objects must be removed first. Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(), mcp.Description("Region of the bucket to delete")),
			mcp.WithString("label", mcp.Required(), mcp.Description("Label of the bucket to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm deletion. This action is irreversible. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleObjectStorageBucketDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleObjectStorageBucketDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByRegionLabel(ctx, request, cfg, &DestructiveActionByRegionLabel{
		ToolName:       "linode_object_storage_bucket_delete",
		Method:         httpMethodDelete,
		PathPattern:    "/object-storage/buckets/%s/%s",
		ConfirmMessage: "This operation is destructive and irreversible. All objects must be removed first. Set confirm=true to proceed.",
		SuccessKey:     "label",
		SuccessFormat:  "Bucket '%s' in %s removed successfully",
		FetchState: func(ctx context.Context, c *linode.Client, region, label string) (any, error) {
			return c.GetObjectStorageBucket(ctx, region, label)
		},
		Execute: func(ctx context.Context, c *linode.Client, region, label string) error {
			return c.DeleteObjectStorageBucket(ctx, region, label)
		},
		// A bucket carries no cosmetic timestamp, so the whole state is
		// hashed; the unknown "ObjectStorageBucket" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("ObjectStorageBucket"),
	})
}

// NewLinodeObjectStorageCancelTool creates a tool for canceling Object Storage service.
func NewLinodeObjectStorageCancelTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_cancel",
		"Cancels Object Storage service for the account. WARNING: This changes account service state.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm Object Storage cancellation. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageCancelRequest,
	)

	return tool, profiles.CapAdmin, handler
}

func handleObjectStorageCancelRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		return RunDryRunPreview(ctx, request, cfg, "linode_object_storage_cancel", httpMethodPost, "/object-storage/cancel", nil)
	}

	if result := RequireConfirm(request, "This operation cancels Object Storage service for the account. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.CancelObjectStorage(ctx); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to cancel Object Storage: %v", err)), nil
	}

	return MarshalToolResponse(struct {
		Message string `json:"message"`
	}{
		Message: "Object Storage cancellation requested successfully",
	})
}

// NewLinodeObjectStorageBucketAccessAllowTool creates a tool for applying bucket access controls.
func NewLinodeObjectStorageBucketAccessAllowTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_bucket_access_allow",
		"Applies access control settings for an Object Storage bucket. Changes ACL and/or CORS configuration.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region of the bucket")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label of the bucket")),
			mcp.WithString("acl",
				mcp.Description("Access control: private, public-read, authenticated-read, or public-read-write")),
			mcp.WithBoolean("cors_enabled",
				mcp.Description("Whether to enable CORS on the bucket")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm access changes. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageBucketAccessAllowRequest,
	)

	return tool, profiles.CapWrite, handler
}

// runObjectStorageAccessDryRun previews a bucket access change. Both the allow
// (POST) and update (PUT) tools fetch current access via the same GET sibling,
// so the preview path and fetch are shared; callers validate region/label and
// pass the would-be HTTP method.
func runObjectStorageAccessDryRun(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	toolName, method, region, label string,
	detailsFn func(ctx context.Context, client *linode.Client, state any) (DryRunDetails, error),
) (*mcp.CallToolResult, error) {
	return RunDryRunPreviewDetailed(ctx, request, cfg, toolName, method,
		fmt.Sprintf("/object-storage/buckets/%s/%s/access", region, label),
		func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetObjectStorageBucketAccess(ctx, region, label)
		},
		detailsFn)
}

// validateBucketAccessAllowArgs validates the allow-access args, returning an
// error message or "". Shared by the real path and the dry-run preview.
func validateBucketAccessAllowArgs(region, label, acl string) string {
	if err := validateRegionSlug(region); err != nil {
		return err.Error()
	}

	if err := validateBucketLabel(label); err != nil {
		return err.Error()
	}

	if acl != "" {
		if err := validateBucketACL(acl); err != nil {
			return err.Error()
		}
	}

	return ""
}

func handleObjectStorageBucketAccessAllowRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	acl := request.GetString("acl", "")

	if IsDryRun(request) {
		if msg := validateBucketAccessAllowArgs(region, label, acl); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return runObjectStorageAccessDryRun(ctx, request, cfg, "linode_object_storage_bucket_access_allow", httpMethodPost, region, label, nil)
	}

	if result := RequireConfirm(request, "This operation changes bucket access controls. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateBucketAccessAllowArgs(region, label, acl); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.AllowObjectStorageBucketAccessRequest{
		ACL: acl,
	}

	if _, ok := request.GetArguments()["cors_enabled"]; ok {
		corsEnabled := request.GetBool("cors_enabled", false)
		req.CORSEnabled = &corsEnabled
	}

	if err := client.AllowObjectStorageBucketAccess(ctx, region, label, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to apply access for bucket '%s' in %s: %v", label, region, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		Region  string `json:"region"`
		Label   string `json:"label"`
		ACL     string `json:"acl,omitempty"`
	}{
		Message: fmt.Sprintf("Access settings for bucket '%s' in %s applied successfully", label, region),
		Region:  region,
		Label:   label,
		ACL:     acl,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageBucketAccessUpdateTool creates a tool for updating bucket access controls.
func NewLinodeObjectStorageBucketAccessUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_bucket_access_update",
		"Updates access control settings for an Object Storage bucket. Changes ACL and/or CORS configuration.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region of the bucket")),
			mcp.WithString("label", mcp.Required(),
				mcp.Description("Label of the bucket")),
			mcp.WithString("acl",
				mcp.Description("New access control: private, public-read, authenticated-read, or public-read-write")),
			mcp.WithBoolean("cors_enabled",
				mcp.Description("Whether to enable CORS on the bucket")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm access update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageBucketAccessUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateBucketAccessUpdateArgs validates the update-access args, returning an
// error message or "". Shared by the real path and the dry-run preview.
func validateBucketAccessUpdateArgs(region, label, acl string) string {
	if region == "" {
		return ErrBucketRegionRequired.Error()
	}

	if label == "" {
		return ErrBucketLabelRequired.Error()
	}

	if acl != "" {
		if err := validateBucketACL(acl); err != nil {
			return err.Error()
		}
	}

	return ""
}

func handleObjectStorageBucketAccessUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	acl := request.GetString("acl", "")

	if IsDryRun(request) {
		if msg := validateBucketAccessUpdateArgs(region, label, acl); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		_, corsProvided := request.GetArguments()["cors_enabled"]
		corsEnabled := request.GetBool("cors_enabled", false)

		return runObjectStorageAccessDryRun(ctx, request, cfg, "linode_object_storage_bucket_access_update", "PUT", region, label,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return bucketAccessUpdateSideEffects(ctx, acl, corsProvided, corsEnabled)
			})
	}

	if result := RequireConfirm(request, "This operation changes bucket access controls. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateBucketAccessUpdateArgs(region, label, acl); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateObjectStorageBucketAccessRequest{
		ACL: acl,
	}

	if _, ok := request.GetArguments()["cors_enabled"]; ok {
		corsEnabled := request.GetBool("cors_enabled", false)
		req.CORSEnabled = &corsEnabled
	}

	if err := client.UpdateObjectStorageBucketAccess(ctx, region, label, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify access for bucket '%s' in %s: %v", label, region, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		Region  string `json:"region"`
		Label   string `json:"label"`
		ACL     string `json:"acl,omitempty"`
	}{
		Message: fmt.Sprintf("Access settings for bucket '%s' in %s modified successfully", label, region),
		Region:  region,
		Label:   label,
		ACL:     acl,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageKeyCreateTool creates a tool for creating an Object Storage access key.
func NewLinodeObjectStorageKeyCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_key_create",
		mcp.WithDescription("Creates a new Object Storage access key. WARNING: The secret_key is only shown ONCE in the response and cannot be retrieved later."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"label",
			mcp.Required(),
			mcp.Description("Label for the access key (max 50 characters)"),
		),
		mcp.WithString(
			"bucket_access",
			mcp.Description("JSON array of bucket permissions: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only|read_write\"}]. Omit for unrestricted access."),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true. The secret_key is only shown ONCE in the response. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// parseObjectStorageKeyBucketAccess parses+validates the bucket_access JSON,
// returning the entries or an error message. Shared by the key create and
// update paths (and their dry-run previews).
func parseObjectStorageKeyBucketAccess(bucketAccessJSON string) ([]linode.ObjectStorageKeyBucketAccess, string) {
	if bucketAccessJSON == "" {
		return nil, ""
	}

	var bucketAccess []linode.ObjectStorageKeyBucketAccess
	if err := json.Unmarshal([]byte(bucketAccessJSON), &bucketAccess); err != nil {
		return nil, fmt.Sprintf("Invalid bucket_access JSON: %v. Expected format: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only\"}]", err)
	}

	if err := validateBucketAccessEntries(bucketAccess); err != nil {
		return nil, err.Error()
	}

	return bucketAccess, ""
}

func handleObjectStorageKeyCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	label := request.GetString("label", "")
	bucketAccessJSON := request.GetString("bucket_access", "")

	if IsDryRun(request) {
		if err := validateKeyLabel(label); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if _, msg := parseObjectStorageKeyBucketAccess(bucketAccessJSON); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_object_storage_key_create", httpMethodPost, "/object-storage/keys", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return objectStorageKeyCreateSideEffects(ctx, label)
			})
	}

	if result := RequireConfirm(request, "This creates an access key. The secret_key is only shown ONCE in the response. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if err := validateKeyLabel(label); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bucketAccess, msg := parseObjectStorageKeyBucketAccess(bucketAccessJSON)
	if msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateObjectStorageKeyRequest{
		Label:        label,
		BucketAccess: bucketAccess,
	}

	key, err := client.CreateObjectStorageKey(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create access key: %v", err)), nil
	}

	response := struct {
		Warning string                   `json:"warning"`
		Message string                   `json:"message"`
		Key     *linode.ObjectStorageKey `json:"key"`
	}{
		Warning: "IMPORTANT: The secret_key below is shown ONLY ONCE. Save it now - it cannot be retrieved later.",
		Message: fmt.Sprintf("Access key '%s' created successfully (ID: %d)", key.Label, key.ID),
		Key:     key,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageKeyUpdateTool creates a tool for updating an Object Storage access key.
func NewLinodeObjectStorageKeyUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_key_update",
		"Updates an Object Storage access key's label or bucket permissions.",
		[]mcp.ToolOption{
			mcp.WithNumber("key_id", mcp.Required(),
				mcp.Description("ID of the access key to update")),
			mcp.WithString("label",
				mcp.Description("New label for the access key (max 50 characters)")),
			mcp.WithString("bucket_access",
				mcp.Description("JSON array of bucket permissions: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only|read_write\"}]")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm key update. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageKeyUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateObjectStorageKeyUpdateArgs validates the key update args, returning
// an error message or "". Shared by the real path and the dry-run preview.
func validateObjectStorageKeyUpdateArgs(keyID int, label, bucketAccessJSON string) string {
	if keyID <= 0 {
		return ErrKeyIDRequired.Error()
	}

	if label != "" {
		if err := validateKeyLabel(label); err != nil {
			return err.Error()
		}
	}

	if _, msg := parseObjectStorageKeyBucketAccess(bucketAccessJSON); msg != "" {
		return msg
	}

	return ""
}

func handleObjectStorageKeyUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	keyID := request.GetInt("key_id", 0)
	label := request.GetString("label", "")
	bucketAccessJSON := request.GetString("bucket_access", "")

	if IsDryRun(request) {
		if msg := validateObjectStorageKeyUpdateArgs(keyID, label, bucketAccessJSON); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_object_storage_key_update", "PUT",
			fmt.Sprintf("/object-storage/keys/%d", keyID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetObjectStorageKey(ctx, keyID) },
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return objectStorageKeyUpdateSideEffects(ctx, state, label, bucketAccessJSON)
			})
	}

	if result := RequireConfirm(request, "This modifies access key permissions. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateObjectStorageKeyUpdateArgs(keyID, label, bucketAccessJSON); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	bucketAccess, _ := parseObjectStorageKeyBucketAccess(bucketAccessJSON)

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateObjectStorageKeyRequest{
		Label:        label,
		BucketAccess: bucketAccess,
	}

	if err := client.UpdateObjectStorageKey(ctx, keyID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify access key %d: %v", keyID, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		KeyID   int    `json:"key_id"`
	}{
		Message: fmt.Sprintf("Access key %d modified successfully", keyID),
		KeyID:   keyID,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageKeyDeleteTool creates a tool for revoking an Object Storage access key.
func NewLinodeObjectStorageKeyDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_key_delete",
		mcp.WithDescription("Revokes an Object Storage access key permanently. Pass dry_run=true to preview without revoking."+twoStageNote),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"key_id",
			mcp.Required(),
			mcp.Description("ID of the access key to revoke"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm key revocation. This action is permanent. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
		mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleObjectStorageKeyDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	// Pre-validate negatives: the helper's default `id == 0` check
	// would let a negative ID through to the API. ErrKeyIDRequired's
	// "must be a positive integer" message is locked by the existing
	// invalid_key_id test in linode_object_storage_test.go.
	if request.GetInt("key_id", 0) < 0 {
		return mcp.NewToolResultError(ErrKeyIDRequired.Error()), nil
	}

	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_object_storage_key_delete",
		IDParam:        "key_id",
		Method:         httpMethodDelete,
		PathPattern:    "/object-storage/keys/%d",
		ConfirmMessage: "This revokes the access key permanently. Set confirm=true to proceed.",
		SuccessFormat:  "Access key %d revoked successfully",
		FetchState: func(ctx context.Context, c *linode.Client, id int) (any, error) {
			return c.GetObjectStorageKey(ctx, id)
		},
		Execute:    func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteObjectStorageKey(ctx, id) },
		HashIgnore: twostage.HashIgnoreFields("ObjectStorageKey"),
	})
}

// NewLinodeObjectStorageObjectACLUpdateTool creates a tool for updating an object's ACL.
func NewLinodeObjectStorageObjectACLUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_object_acl_update",
		mcp.WithDescription("Updates the Access Control List (ACL) for a specific object in an Object Storage bucket. "+
			"Requires confirm=true to proceed."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString(
			"label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
		mcp.WithString(
			"name",
			mcp.Required(),
			mcp.Description("The object key (path/filename within the bucket)"),
		),
		mcp.WithString(
			"acl",
			mcp.Required(),
			mcp.Description("ACL to set: 'private', 'public-read', 'authenticated-read', or 'public-read-write'"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be true to proceed. This modifies the object's access permissions. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageObjectACLUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateObjectACLUpdateArgs validates the object ACL update args, returning
// an error message or "". Shared by the real path and the dry-run preview.
func validateObjectACLUpdateArgs(region, label, name, acl string) string {
	if region == "" {
		return errRegionRequired
	}

	if label == "" {
		return errLabelRequired
	}

	if name == "" {
		return ErrObjectNameRequired.Error()
	}

	if err := validateBucketACL(acl); err != nil {
		return err.Error()
	}

	return ""
}

func handleObjectStorageObjectACLUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	name := request.GetString("name", "")
	acl := request.GetString("acl", "")

	if IsDryRun(request) {
		if msg := validateObjectACLUpdateArgs(region, label, name, acl); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_object_storage_object_acl_update", "PUT",
			fmt.Sprintf("/object-storage/buckets/%s/%s/object-acl", region, label),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetObjectACL(ctx, region, label, name)
			},
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return objectACLUpdateSideEffects(ctx, acl)
			})
	}

	if result := RequireConfirm(request, "This modifies the object's access permissions. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateObjectACLUpdateArgs(region, label, name, acl); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.ObjectACLUpdateRequest{
		ACL:  acl,
		Name: name,
	}

	result, err := client.UpdateObjectACL(ctx, region, label, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify ACL for object '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return MarshalToolResponse(result)
}

// NewLinodeObjectStorageSSLDeleteTool creates a tool for deleting a bucket's SSL certificate.
func NewLinodeObjectStorageSSLDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_ssl_delete",
		"Deletes the SSL/TLS certificate from an Object Storage bucket. "+
			"Requires confirm=true to proceed. Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')")),
			mcp.WithString("label", mcp.Required(), mcp.Description("The bucket label (name)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to proceed. This removes the SSL certificate from the bucket. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleObjectStorageSSLDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleObjectStorageSSLDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByRegionLabel(ctx, request, cfg, &DestructiveActionByRegionLabel{
		ToolName:       "linode_object_storage_ssl_delete",
		Method:         httpMethodDelete,
		PathPattern:    "/object-storage/buckets/%s/%s/ssl",
		ConfirmMessage: "This removes the SSL certificate from the bucket. Set confirm=true to proceed.",
		SuccessKey:     "bucket",
		SuccessFormat:  "SSL certificate deleted from bucket '%s' in region '%s'",
		FetchState: func(ctx context.Context, c *linode.Client, region, label string) (any, error) {
			return c.GetBucketSSL(ctx, region, label)
		},
		Execute: func(ctx context.Context, c *linode.Client, region, label string) error {
			return c.DeleteBucketSSL(ctx, region, label)
		},
		// Bucket SSL state is just a boolean flag with no cosmetic field, so
		// the whole state is hashed; the unknown "ObjectStorageSSL" key
		// returns nil.
		HashIgnore: twostage.HashIgnoreFields("ObjectStorageSSL"),
	})
}

// NewLinodeObjectStorageSSLUploadTool creates a tool for uploading an SSL certificate to a bucket.
func NewLinodeObjectStorageSSLUploadTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_object_storage_ssl_upload",
		"Uploads an SSL/TLS certificate to an Object Storage bucket. Requires confirm=true to proceed.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')")),
			mcp.WithString("label", mcp.Required(), mcp.Description("The bucket label (name)")),
			mcp.WithString("certificate", mcp.Required(),
				mcp.Description("The PEM-encoded TLS/SSL certificate to upload")),
			mcp.WithString("private_key", mcp.Required(),
				mcp.Description("The PEM-encoded private key for the certificate")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to proceed. This uploads a certificate to the bucket. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleObjectStorageSSLUploadRequest,
	)

	return tool, profiles.CapWrite, handler
}

// validateSSLUploadArgs validates the SSL upload args, returning an error
// message or "". Shared by the real path and the dry-run preview.
func validateSSLUploadArgs(region, label, certificate, privateKey string) string {
	if region == "" {
		return errRegionRequired
	}

	if label == "" {
		return errLabelRequired
	}

	if certificate == "" {
		return "certificate is required"
	}

	if privateKey == "" {
		return "private_key is required"
	}

	return ""
}

func handleObjectStorageSSLUploadRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	certificate := request.GetString("certificate", "")
	privateKey := request.GetString("private_key", "")

	if IsDryRun(request) {
		if msg := validateSSLUploadArgs(region, label, certificate, privateKey); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		// nil fetchState: current_state is null. The request body (cert +
		// private_key) is never echoed in the v0 preview, so no key leaks.
		return RunDryRunPreview(ctx, request, cfg, "linode_object_storage_ssl_upload", httpMethodPost,
			fmt.Sprintf("/object-storage/buckets/%s/%s/ssl", region, label), nil)
	}

	if result := RequireConfirm(request, "This uploads an SSL certificate to the bucket. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateSSLUploadArgs(region, label, certificate, privateKey); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UploadBucketSSLRequest{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}

	ssl, err := client.UploadBucketSSL(ctx, region, label, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to upload SSL certificate for bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		Region  string            `json:"region"`
		Bucket  string            `json:"bucket"`
		SSL     *linode.BucketSSL `json:"ssl"`
	}{
		Message: fmt.Sprintf("SSL certificate uploaded to bucket '%s' in region '%s'", label, region),
		Region:  region,
		Bucket:  label,
		SSL:     ssl,
	}

	return MarshalToolResponse(response)
}

// validateBucketAccessEntries validates each entry in a bucket_access array.
func validateBucketAccessEntries(entries []linode.ObjectStorageKeyBucketAccess) error {
	for idx, entry := range entries {
		if strings.TrimSpace(entry.BucketName) == "" {
			return fmt.Errorf("entry %d: %w", idx, ErrKeyBucketNameRequired)
		}

		if strings.TrimSpace(entry.Region) == "" {
			return fmt.Errorf("entry %d: %w", idx, ErrKeyBucketRegionRequired)
		}

		if err := validateKeyPermissions(entry.Permissions); err != nil {
			return fmt.Errorf("entry %d: %w", idx, err)
		}
	}

	return nil
}
