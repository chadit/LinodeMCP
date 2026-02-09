//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

const defaultPresignedExpiry = 3600

// NewLinodeObjectStorageBucketsListTool creates a tool for listing Object Storage buckets.
func NewLinodeObjectStorageBucketsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_buckets_list",
		mcp.WithDescription("Lists all Object Storage buckets across all regions for the authenticated user"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	buckets, err := client.ListObjectStorageBuckets(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage buckets: %v", err)), nil
	}

	response := struct {
		Count   int                          `json:"count"`
		Buckets []linode.ObjectStorageBucket `json:"buckets"`
	}{
		Count:   len(buckets),
		Buckets: buckets,
	}

	return marshalToolResponse(response)
}

// NewLinodeObjectStorageBucketGetTool creates a tool for getting a specific bucket.
func NewLinodeObjectStorageBucketGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_get",
		mcp.WithDescription("Gets details about a specific Object Storage bucket by region and label"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	bucket, err := client.GetObjectStorageBucket(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return marshalToolResponse(bucket)
}

// NewLinodeObjectStorageBucketContentsTool creates a tool for listing objects in a bucket.
func NewLinodeObjectStorageBucketContentsTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_contents",
		mcp.WithDescription("Lists objects in an Object Storage bucket with optional prefix/delimiter filtering and pagination"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
		mcp.WithString("prefix",
			mcp.Description("Filter objects by key prefix (e.g., 'images/' to list only objects in the images folder)"),
		),
		mcp.WithString("delimiter",
			mcp.Description("Delimiter for grouping keys (typically '/' for folder-like listing)"),
		),
		mcp.WithString("marker",
			mcp.Description("Pagination marker from a previous truncated response"),
		),
		mcp.WithString("page_size",
			mcp.Description("Number of objects to return per page (default 100, max 500)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketContentsRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketContentsRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	prefix := request.GetString("prefix", "")
	delimiter := request.GetString("delimiter", "")
	marker := request.GetString("marker", "")
	pageSize := request.GetString("page_size", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	params := make(map[string]string)
	if prefix != "" {
		params["prefix"] = prefix
	}

	if delimiter != "" {
		params["delimiter"] = delimiter
	}

	if marker != "" {
		params["marker"] = marker
	}

	if pageSize != "" {
		params["page_size"] = pageSize
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	objects, isTruncated, nextMarker, err := client.ListObjectStorageBucketContents(ctx, region, label, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list contents of bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return formatBucketContentsResponse(objects, isTruncated, nextMarker, prefix, delimiter)
}

func formatBucketContentsResponse(objects []linode.ObjectStorageObject, isTruncated bool, nextMarker, prefix, delimiter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count       int                          `json:"count"`
		Filter      string                       `json:"filter,omitempty"`
		IsTruncated bool                         `json:"is_truncated"`          //nolint:tagliatelle // match Linode API naming
		NextMarker  string                       `json:"next_marker,omitempty"` //nolint:tagliatelle // match Linode API naming
		Objects     []linode.ObjectStorageObject `json:"objects"`
	}{
		Count:       len(objects),
		IsTruncated: isTruncated,
		NextMarker:  nextMarker,
		Objects:     objects,
	}

	var filters []string
	if prefix != "" {
		filters = append(filters, "prefix="+prefix)
	}

	if delimiter != "" {
		filters = append(filters, "delimiter="+delimiter)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return marshalToolResponse(response)
}

// NewLinodeObjectStorageClustersListTool creates a tool for listing Object Storage clusters.
func NewLinodeObjectStorageClustersListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_clusters_list",
		mcp.WithDescription("Lists available Object Storage clusters/regions where buckets can be created"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageClustersListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageClustersListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	clusters, err := client.ListObjectStorageClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage clusters: %v", err)), nil
	}

	response := struct {
		Count    int                           `json:"count"`
		Clusters []linode.ObjectStorageCluster `json:"clusters"`
	}{
		Count:    len(clusters),
		Clusters: clusters,
	}

	return marshalToolResponse(response)
}

// NewLinodeObjectStorageTypeListTool creates a tool for listing Object Storage types and pricing.
func NewLinodeObjectStorageTypeListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_type_list",
		mcp.WithDescription("Lists Object Storage types and pricing information"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageTypeListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageTypeListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	types, err := client.ListObjectStorageTypes(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage types: %v", err)), nil
	}

	response := struct {
		Count int                        `json:"count"`
		Types []linode.ObjectStorageType `json:"types"`
	}{
		Count: len(types),
		Types: types,
	}

	return marshalToolResponse(response)
}

// Phase 2: Read-Only Access Key & Transfer Tools

// NewLinodeObjectStorageKeysListTool creates a tool for listing Object Storage access keys.
func NewLinodeObjectStorageKeysListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_keys_list",
		mcp.WithDescription("Lists all Object Storage access keys for the authenticated user"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeysListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageKeysListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	keys, err := client.ListObjectStorageKeys(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage keys: %v", err)), nil
	}

	response := struct {
		Count int                       `json:"count"`
		Keys  []linode.ObjectStorageKey `json:"keys"`
	}{
		Count: len(keys),
		Keys:  keys,
	}

	return marshalToolResponse(response)
}

// NewLinodeObjectStorageKeyGetTool creates a tool for getting a specific access key.
func NewLinodeObjectStorageKeyGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_key_get",
		mcp.WithDescription("Gets details about a specific Object Storage access key by ID"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("key_id",
			mcp.Required(),
			mcp.Description("The ID of the access key to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageKeyGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	keyIDStr := request.GetString("key_id", "")

	if keyIDStr == "" {
		return mcp.NewToolResultError("key_id is required"), nil
	}

	keyID, err := strconv.Atoi(keyIDStr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("key_id must be a valid integer: %v", err)), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	key, err := client.GetObjectStorageKey(ctx, keyID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve access key %d: %v", keyID, err)), nil
	}

	return marshalToolResponse(key)
}

// NewLinodeObjectStorageTransferTool creates a tool for getting Object Storage transfer usage.
func NewLinodeObjectStorageTransferTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_transfer",
		mcp.WithDescription("Gets Object Storage outbound data transfer usage for the current month"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageTransferRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageTransferRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	transfer, err := client.GetObjectStorageTransfer(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage transfer usage: %v", err)), nil
	}

	return marshalToolResponse(transfer)
}

// NewLinodeObjectStorageBucketAccessGetTool creates a tool for getting bucket ACL/CORS settings.
func NewLinodeObjectStorageBucketAccessGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_access_get",
		mcp.WithDescription("Gets the ACL and CORS settings for a specific Object Storage bucket"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketAccessGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketAccessGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	access, err := client.GetObjectStorageBucketAccess(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve bucket access for '%s' in region '%s': %v", label, region, err)), nil
	}

	return marshalToolResponse(access)
}

// NewLinodeObjectStoragePresignedURLTool creates a tool for generating presigned URLs for objects.
func NewLinodeObjectStoragePresignedURLTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_presigned_url",
		mcp.WithDescription("Generates a presigned URL for accessing an object in Object Storage. "+
			"Use method=GET to create a download URL, method=PUT to create an upload URL."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The object key (path/filename within the bucket)"),
		),
		mcp.WithString("method",
			mcp.Required(),
			mcp.Description("HTTP method: 'GET' for download URL, 'PUT' for upload URL"),
		),
		mcp.WithNumber("expires_in",
			mcp.Description("URL expiration in seconds (1-604800, default 3600 = 1 hour)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStoragePresignedURLRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStoragePresignedURLRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	name := request.GetString("name", "")
	method := request.GetString("method", "")
	expiresIn := request.GetInt("expires_in", defaultPresignedExpiry)

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if name == "" {
		return mcp.NewToolResultError(ErrObjectNameRequired.Error()), nil
	}

	if err := validatePresignedMethod(method); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateExpiresIn(expiresIn); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.PresignedURLRequest{
		Method:    strings.ToUpper(method),
		Name:      name,
		ExpiresIn: expiresIn,
	}

	result, err := client.CreatePresignedURL(ctx, region, label, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate presigned URL for '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return marshalToolResponse(result)
}

// NewLinodeObjectStorageObjectACLGetTool creates a tool for getting an object's ACL.
func NewLinodeObjectStorageObjectACLGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_object_acl_get",
		mcp.WithDescription("Gets the Access Control List (ACL) for a specific object in an Object Storage bucket"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
		mcp.WithString("name",
			mcp.Required(),
			mcp.Description("The object key (path/filename within the bucket)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageObjectACLGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageObjectACLGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")
	name := request.GetString("name", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if name == "" {
		return mcp.NewToolResultError(ErrObjectNameRequired.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	acl, err := client.GetObjectACL(ctx, region, label, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve ACL for object '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return marshalToolResponse(acl)
}

// NewLinodeObjectStorageSSLGetTool creates a tool for checking a bucket's SSL certificate status.
func NewLinodeObjectStorageSSLGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_ssl_get",
		mcp.WithDescription("Checks whether an Object Storage bucket has an SSL/TLS certificate installed"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Required(),
			mcp.Description("Region where the bucket is located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
		mcp.WithString("label",
			mcp.Required(),
			mcp.Description("The bucket label (name)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageSSLGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageSSLGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	ssl, err := client.GetBucketSSL(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSL status for bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return marshalToolResponse(ssl)
}
