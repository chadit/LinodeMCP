//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeObjectStorageBucketCreateTool creates a tool for creating an Object Storage bucket.
func NewLinodeObjectStorageBucketCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_create",
		mcp.WithDescription("Creates a new Object Storage bucket. WARNING: Billing starts immediately. Use linode_object_storage_clusters_list to find valid regions."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Bucket label (3-63 chars, lowercase alphanumeric and hyphens, must start/end with alphanumeric)"),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region for the bucket (e.g. us-east-1). Use linode_object_storage_clusters_list to find valid regions."),
		),
		mcp.WithString("acl",
			mcp.Description("Access control: private, public-read, authenticated-read, or public-read-write (default: private)"),
		),
		mcp.WithBoolean("cors_enabled",
			mcp.Description("Whether to enable CORS on the bucket (default: true)"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm bucket creation. This operation incurs billing charges."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	label := request.GetString("label", "")
	region := request.GetString("region", "")
	acl := request.GetString("acl", "")
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation creates a billable resource. Set confirm=true to proceed."), nil
	}

	if err := validateBucketLabel(label); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if region == "" {
		return mcp.NewToolResultError(ErrBucketRegionRequired.Error()), nil
	}

	if acl != "" {
		if err := validateBucketACL(acl); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageBucketDeleteTool creates a tool for deleting an Object Storage bucket.
func NewLinodeObjectStorageBucketDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_delete",
		mcp.WithDescription("Deletes an Object Storage bucket. WARNING: This is irreversible. All objects must be removed first."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region of the bucket to delete"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label of the bucket to delete"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion. This action is irreversible."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation is destructive and irreversible. All objects must be removed first. Set confirm=true to proceed."), nil
	}

	if region == "" {
		return mcp.NewToolResultError(ErrBucketRegionRequired.Error()), nil
	}

	if label == "" {
		return mcp.NewToolResultError(ErrBucketLabelRequired.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteObjectStorageBucket(ctx, region, label); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete bucket '%s' in %s: %v", label, region, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		Region  string `json:"region"`
		Label   string `json:"label"`
	}{
		Message: fmt.Sprintf("Bucket '%s' in %s deleted successfully", label, region),
		Region:  region,
		Label:   label,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageBucketAccessUpdateTool creates a tool for updating bucket access controls.
func NewLinodeObjectStorageBucketAccessUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_access_update",
		mcp.WithDescription("Updates access control settings for an Object Storage bucket. Changes ACL and/or CORS configuration."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region of the bucket"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label of the bucket"),
		),
		mcp.WithString("acl",
			mcp.Description("New access control: private, public-read, authenticated-read, or public-read-write"),
		),
		mcp.WithBoolean("cors_enabled",
			mcp.Description("Whether to enable CORS on the bucket"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm access update."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketAccessUpdateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketAccessUpdateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	acl := request.GetString("acl", "")
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This operation changes bucket access controls. Set confirm=true to proceed."), nil
	}

	if region == "" {
		return mcp.NewToolResultError(ErrBucketRegionRequired.Error()), nil
	}

	if label == "" {
		return mcp.NewToolResultError(ErrBucketLabelRequired.Error()), nil
	}

	if acl != "" {
		if err := validateBucketACL(acl); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.UpdateObjectStorageBucketAccessRequest{
		ACL: acl,
	}

	if _, ok := request.GetArguments()["cors_enabled"]; ok {
		corsEnabled := request.GetBool("cors_enabled", false)
		req.CORSEnabled = &corsEnabled
	}

	if err := client.UpdateObjectStorageBucketAccess(ctx, region, label, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update access for bucket '%s' in %s: %v", label, region, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		Region  string `json:"region"`
		Label   string `json:"label"`
		ACL     string `json:"acl,omitempty"`
	}{
		Message: fmt.Sprintf("Access settings for bucket '%s' in %s updated successfully", label, region),
		Region:  region,
		Label:   label,
		ACL:     acl,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageKeyCreateTool creates a tool for creating an Object Storage access key.
func NewLinodeObjectStorageKeyCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_key_create",
		mcp.WithDescription("Creates a new Object Storage access key. WARNING: The secret_key is only shown ONCE in the response and cannot be retrieved later."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("Label for the access key (max 50 characters)"),
		),
		mcp.WithString("bucket_access",
			mcp.Description("JSON array of bucket permissions: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only|read_write\"}]. Omit for unrestricted access."),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true. The secret_key is only shown ONCE in the response."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageKeyCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	label := request.GetString("label", "")
	bucketAccessJSON := request.GetString("bucket_access", "")
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError(
			"This creates an access key. The secret_key is only shown ONCE in the response. Set confirm=true to proceed.",
		), nil
	}

	if err := validateKeyLabel(label); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var bucketAccess []linode.ObjectStorageKeyBucketAccess

	if bucketAccessJSON != "" {
		if err := json.Unmarshal([]byte(bucketAccessJSON), &bucketAccess); err != nil {
			return mcp.NewToolResultError(
				fmt.Sprintf("Invalid bucket_access JSON: %v. Expected format: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only\"}]", err),
			), nil
		}

		if err := validateBucketAccessEntries(bucketAccess); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageKeyUpdateTool creates a tool for updating an Object Storage access key.
func NewLinodeObjectStorageKeyUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_key_update",
		mcp.WithDescription("Updates an Object Storage access key's label or bucket permissions."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("key_id",
			mcp.Required(),
			mcp.Description("ID of the access key to update"),
		),
		mcp.WithString("label",
			mcp.Description("New label for the access key (max 50 characters)"),
		),
		mcp.WithString("bucket_access",
			mcp.Description("JSON array of bucket permissions: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only|read_write\"}]"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm key update."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyUpdateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageKeyUpdateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	keyID := request.GetInt("key_id", 0)
	label := request.GetString("label", "")
	bucketAccessJSON := request.GetString("bucket_access", "")
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This modifies access key permissions. Set confirm=true to proceed."), nil
	}

	if keyID <= 0 {
		return mcp.NewToolResultError(ErrKeyIDRequired.Error()), nil
	}

	if label != "" {
		if err := validateKeyLabel(label); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	var bucketAccess []linode.ObjectStorageKeyBucketAccess

	if bucketAccessJSON != "" {
		if err := json.Unmarshal([]byte(bucketAccessJSON), &bucketAccess); err != nil {
			return mcp.NewToolResultError(
				fmt.Sprintf("Invalid bucket_access JSON: %v. Expected format: [{\"bucket_name\": \"name\", \"region\": \"region\", \"permissions\": \"read_only\"}]", err),
			), nil
		}

		if err := validateBucketAccessEntries(bucketAccess); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.UpdateObjectStorageKeyRequest{
		Label:        label,
		BucketAccess: bucketAccess,
	}

	if err := client.UpdateObjectStorageKey(ctx, keyID, req); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update access key %d: %v", keyID, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		KeyID   int    `json:"key_id"` //nolint:tagliatelle // JSON snake_case for API consistency
	}{
		Message: fmt.Sprintf("Access key %d updated successfully", keyID),
		KeyID:   keyID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageKeyDeleteTool creates a tool for revoking an Object Storage access key.
func NewLinodeObjectStorageKeyDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_key_delete",
		mcp.WithDescription("Revokes an Object Storage access key permanently."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("key_id",
			mcp.Required(),
			mcp.Description("ID of the access key to revoke"),
		),
		mcp.WithBoolean("confirm",
			mcp.Required(),
			mcp.Description("Must be set to true to confirm key revocation. This action is permanent."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageKeyDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	keyID := request.GetInt("key_id", 0)
	confirm := request.GetBool("confirm", false)

	if !confirm {
		return mcp.NewToolResultError("This revokes the access key permanently. Set confirm=true to proceed."), nil
	}

	if keyID <= 0 {
		return mcp.NewToolResultError(ErrKeyIDRequired.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteObjectStorageKey(ctx, keyID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to revoke access key %d: %v", keyID, err)), nil
	}

	response := struct {
		Message string `json:"message"`
		KeyID   int    `json:"key_id"` //nolint:tagliatelle // JSON snake_case for API consistency
	}{
		Message: fmt.Sprintf("Access key %d revoked successfully", keyID),
		KeyID:   keyID,
	}

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// validateBucketAccessEntries validates each entry in a bucket_access array.
func validateBucketAccessEntries(entries []linode.ObjectStorageKeyBucketAccess) error {
	for i, entry := range entries {
		if strings.TrimSpace(entry.BucketName) == "" {
			return fmt.Errorf("entry %d: %w", i, ErrKeyBucketNameRequired)
		}

		if strings.TrimSpace(entry.Region) == "" {
			return fmt.Errorf("entry %d: %w", i, ErrKeyBucketRegionRequired)
		}

		if err := validateKeyPermissions(entry.Permissions); err != nil {
			return fmt.Errorf("entry %d: %w", i, err)
		}
	}

	return nil
}
