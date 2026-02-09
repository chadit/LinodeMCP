//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"encoding/json"
	"fmt"

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
		Message string                       `json:"message"`
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
