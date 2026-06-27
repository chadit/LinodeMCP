package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const defaultPresignedExpiry = 3600

// NewLinodeObjectStorageBucketListTool creates a tool for listing Object Storage buckets.
func NewLinodeObjectStorageBucketListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_bucket_list",
		mcp.WithDescription("Lists all Object Storage buckets across all regions for the authenticated user"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageBucketsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageBucketListByRegionTool creates a tool for listing buckets in a region.
func NewLinodeObjectStorageBucketListByRegionTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_bucket_by_region_list",
		mcp.WithDescription("Lists Object Storage buckets in a specific region for the authenticated user"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"region",
			mcp.Required(),
			mcp.Description("Region where buckets are located (e.g., 'us-east-1', 'us-southeast-1')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketsListByRegionRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageBucketsListByRegionRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if !isSafeObjectStorageRegion(region) {
		return mcp.NewToolResultError("region must not contain path separators, query separators, or traversal segments"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	buckets, err := client.ListObjectStorageBucketsByRegion(ctx, region)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage buckets in region '%s': %v", region, err)), nil
	}

	response := struct {
		Region  string                       `json:"region"`
		Count   int                          `json:"count"`
		Buckets []linode.ObjectStorageBucket `json:"buckets"`
	}{
		Region:  region,
		Count:   len(buckets),
		Buckets: buckets,
	}

	return MarshalToolResponse(response)
}

func isSafeObjectStorageRegion(region string) bool {
	if region == "" || strings.HasPrefix(region, "-") || strings.HasSuffix(region, "-") || strings.Contains(region, "--") {
		return false
	}

	for _, r := range region {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}

		return false
	}

	return true
}

func isSafeObjectStorageQuotaID(quotaID string) bool {
	if quotaID == "" || strings.Contains(quotaID, "..") {
		return false
	}

	for _, r := range quotaID {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			continue
		}

		return false
	}

	return true
}

// NewLinodeObjectStorageBucketGetTool creates a tool for getting a specific bucket.
func NewLinodeObjectStorageBucketGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_bucket_get",
		"Gets details about a specific Object Storage bucket by region and label",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageBucketGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageBucketGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bucket, err := client.GetObjectStorageBucketProto(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return MarshalProtoToolResponse(bucket)
}

// NewLinodeObjectStorageBucketContentsTool creates a tool for listing objects in a bucket.
func NewLinodeObjectStorageBucketContentsTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_bucket_object_list",
		mcp.WithDescription("Lists objects in an Object Storage bucket with optional prefix/delimiter filtering and pagination"),
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
			"prefix",
			mcp.Description("Filter objects by key prefix (e.g., 'images/' to list only objects in the images folder)"),
		),
		mcp.WithString(
			"delimiter",
			mcp.Description("Delimiter for grouping keys (typically '/' for folder-like listing)"),
		),
		mcp.WithString(
			"marker",
			mcp.Description("Pagination marker from a previous truncated response"),
		),
		mcp.WithString(
			"page_size",
			mcp.Description("Number of objects to return per page (default 100, max 500)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketContentsRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageBucketContentsRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	client, err := prepareClient(request, cfg)
	if err != nil {
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
		IsTruncated bool                         `json:"is_truncated"`
		NextMarker  string                       `json:"next_marker,omitempty"`
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

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageEndpointListTool creates a tool for listing Object Storage endpoints.
func NewLinodeObjectStorageEndpointListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_endpoint_list",
		mcp.WithDescription("Lists Object Storage endpoints across regions"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageEndpointListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageEndpointListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	endpoints, err := client.ListObjectStorageEndpoints(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage endpoints: %v", err)), nil
	}

	response := struct {
		Count     int                            `json:"count"`
		Endpoints []linode.ObjectStorageEndpoint `json:"endpoints"`
	}{
		Count:     len(endpoints),
		Endpoints: endpoints,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageTypeListTool creates a tool for listing Object Storage types and pricing.
func NewLinodeObjectStorageTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_type_list",
		mcp.WithDescription("Lists Object Storage types and pricing information"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageTypeListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageTypeListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageQuotasListTool creates a tool for listing Object Storage quotas.
func NewLinodeObjectStorageQuotasListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_quota_list",
		mcp.WithDescription("Lists Object Storage quotas on the account"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageQuotasListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageQuotasListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	quotas, err := client.ListObjectStorageQuotas(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage quotas: %v", err)), nil
	}

	response := struct {
		Count  int                         `json:"count"`
		Quotas []linode.ObjectStorageQuota `json:"quotas"`
	}{
		Count:  len(quotas),
		Quotas: quotas,
	}

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageKeyListTool creates a tool for listing Object Storage access keys.
func NewLinodeObjectStorageKeyListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_key_list",
		mcp.WithDescription("Lists all Object Storage access keys for the authenticated user"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeysListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageKeysListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	return MarshalToolResponse(response)
}

// NewLinodeObjectStorageKeyGetTool creates a tool for getting a specific access key.
func NewLinodeObjectStorageKeyGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_key_get",
		"Gets details about a specific Object Storage access key by ID",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageKeyGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageKeyGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageKeyGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	keyID, validationMessage := optionalPaginationInt(request.GetArguments(), "key_id", 1, 0)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if keyID == 0 {
		return mcp.NewToolResultError("key_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	key, err := client.GetObjectStorageKeyProto(ctx, keyID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve access key %d: %v", keyID, err)), nil
	}

	return MarshalProtoToolResponse(key)
}

// NewLinodeObjectStorageQuotaUsageTool creates a tool for getting Object Storage quota usage.
func NewLinodeObjectStorageQuotaUsageTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_quota_usage_get",
		mcp.WithDescription("Gets usage data for a specific Object Storage quota"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"obj_quota_id",
			mcp.Required(),
			mcp.Description("The Object Storage quota ID to retrieve usage for"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageQuotaUsageRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageQuotaUsageRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	quotaID := request.GetString("obj_quota_id", "")

	if quotaID == "" {
		return mcp.NewToolResultError("obj_quota_id is required"), nil
	}

	if !isSafeObjectStorageQuotaID(quotaID) {
		return mcp.NewToolResultError("obj_quota_id must not contain path separators, query separators, traversal segments, or unsupported characters"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	usage, err := client.GetObjectStorageQuotaUsage(ctx, quotaID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage quota usage for %q: %v", quotaID, err)), nil
	}

	return MarshalToolResponse(usage)
}

// NewLinodeObjectStorageTransferTool creates a tool for getting Object Storage transfer usage.
func NewLinodeObjectStorageTransferTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_transfer_get",
		mcp.WithDescription("Gets Object Storage outbound data transfer usage for the current month"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageTransferRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageTransferRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	transfer, err := client.GetObjectStorageTransfer(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage transfer usage: %v", err)), nil
	}

	return MarshalToolResponse(transfer)
}

// NewLinodeObjectStorageQuotaGetTool creates a tool for getting a single Object Storage quota.
func NewLinodeObjectStorageQuotaGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_quota_get",
		mcp.WithDescription("Gets details about a single Object Storage quota by ID"),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString(
			"obj_quota_id",
			mcp.Required(),
			mcp.Description("The Object Storage quota ID to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageQuotaGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageQuotaGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	objQuotaID := request.GetString("obj_quota_id", "")
	if objQuotaID == "" {
		return mcp.NewToolResultError("obj_quota_id is required"), nil
	}

	if objQuotaID != strings.TrimSpace(objQuotaID) || strings.ContainsAny(objQuotaID, "/?#") || strings.Contains(objQuotaID, "..") {
		return mcp.NewToolResultError("obj_quota_id must not contain path separators, query separators, fragments, or traversal segments"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	quota, err := client.GetObjectStorageQuota(ctx, objQuotaID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage quota %s: %v", objQuotaID, err)), nil
	}

	return MarshalToolResponse(quota)
}

// NewLinodeObjectStorageBucketAccessGetTool creates a tool for getting bucket ACL/CORS settings.
func NewLinodeObjectStorageBucketAccessGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_bucket_access_get",
		"Gets the ACL and CORS settings for a specific Object Storage bucket",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageBucketAccessGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketAccessGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageBucketAccessGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	access, err := client.GetObjectStorageBucketAccessProto(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve bucket access for '%s' in region '%s': %v", label, region, err)), nil
	}

	return MarshalProtoToolResponse(access)
}

// NewLinodeObjectStoragePresignedURLTool creates a tool for generating presigned URLs for objects.
func NewLinodeObjectStoragePresignedURLTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_object_storage_presigned_url_create",
		mcp.WithDescription("Generates a presigned URL for accessing an object in Object Storage. "+
			"Use method=GET to create a download URL, method=PUT to create an upload URL."),
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
			"method",
			mcp.Required(),
			mcp.Description("HTTP method: 'GET' for download URL, 'PUT' for upload URL"),
		),
		mcp.WithNumber(
			"expires_in",
			mcp.Description("URL expiration in seconds (1-604800, default 3600 = 1 hour)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStoragePresignedURLRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStoragePresignedURLRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.PresignedURLRequest{
		Method:    strings.ToUpper(method),
		Name:      name,
		ExpiresIn: expiresIn,
	}

	result, err := client.CreatePresignedURL(ctx, region, label, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate presigned URL for '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return MarshalToolResponse(result)
}

// NewLinodeObjectStorageObjectACLGetTool creates a tool for getting an object's ACL.
func NewLinodeObjectStorageObjectACLGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_object_acl_get",
		"Gets the Access Control List (ACL) for a specific object in an Object Storage bucket",
		toolschemas.Schema("linode.mcp.v1.ObjectACLGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageObjectACLGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageObjectACLGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	acl, err := client.GetObjectACLProto(ctx, region, label, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve ACL for object '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return MarshalProtoToolResponse(acl)
}

// NewLinodeObjectStorageSSLGetTool creates a tool for checking a bucket's SSL certificate status.
func NewLinodeObjectStorageSSLGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_ssl_get",
		"Checks whether an Object Storage bucket has an SSL/TLS certificate installed",
		toolschemas.Schema("linode.mcp.v1.BucketSSLGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageSSLGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleObjectStorageSSLGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region := request.GetString("region", "")
	label := request.GetString("label", "")

	if region == "" {
		return mcp.NewToolResultError("region is required"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ssl, err := client.GetBucketSSLProto(ctx, region, label)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve SSL status for bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return MarshalProtoToolResponse(ssl)
}
