package tools

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const defaultPresignedExpiry = 3600

// NewLinodeObjectStorageBucketListTool creates a tool for listing Object Storage buckets.
func NewLinodeObjectStorageBucketListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_object_storage_bucket_list",
		"Lists all Object Storage buckets across all regions for the authenticated user",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.ObjectStorageBucket, error) {
			return client.ListObjectStorageBucketsProto(ctx)
		},
		nil,
		objectStorageBucketListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_bucket_list",
		"Lists all Object Storage buckets across all regions for the authenticated user",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageBucketListInput"),
	)

	return tool, profiles.CapRead, handler
}

func objectStorageBucketListResponse(items []*linodev1.ObjectStorageBucket, count int32, filter *string) *linodev1.ObjectStorageBucketListResponse {
	return &linodev1.ObjectStorageBucketListResponse{Count: count, Filter: filter, Buckets: items}
}

// NewLinodeObjectStorageBucketListByRegionTool creates a tool for listing buckets in a region.
func NewLinodeObjectStorageBucketListByRegionTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_bucket_by_region_list",
		"Lists Object Storage buckets in a specific region for the authenticated user",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageBucketByRegionListInput"),
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
		return mcp.NewToolResultError("region must be a valid region or cluster ID"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	buckets, err := client.ListObjectStorageBucketsByRegionProto(ctx, region)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage buckets in region '%s': %v", region, err)), nil
	}

	// Routed through the shared ObjectStorageBucketListResponse envelope (count +
	// buckets) so the output is byte-identical to linode_object_storage_bucket_list
	// and to the Python handler. The region is an input echo, not part of the proto
	// contract, so it is not emitted.
	return finishProtoList(request, buckets, nil, objectStorageBucketListResponse)
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

const objectStorageBucketLabelMaxLength = 63

var objectStorageBucketLabelPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]{1,2}$`)

// validObjectStorageBucketLabel mirrors Python _valid_bucket_label: an S3 bucket
// label is lowercase alphanumeric with internal hyphens, at most 63 characters.
// Ported to Go so both languages reject malformed labels locally instead of
// forwarding them to the API (strictest-wins; Go previously only checked presence).
func validObjectStorageBucketLabel(label string) bool {
	return len(label) <= objectStorageBucketLabelMaxLength && objectStorageBucketLabelPattern.MatchString(label)
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

	if !isSafeObjectStorageRegion(region) {
		return mcp.NewToolResultError("region must be a valid region or cluster ID"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if !validObjectStorageBucketLabel(label) {
		return mcp.NewToolResultError("label must be a valid bucket label"), nil
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
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_bucket_object_list",
		"Lists objects in an Object Storage bucket with optional prefix/delimiter filtering and pagination",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageBucketObjectListInput"),
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

	if !isSafeObjectStorageRegion(region) {
		return mcp.NewToolResultError("region must be a valid region or cluster ID"), nil
	}

	if label == "" {
		return mcp.NewToolResultError("label is required"), nil
	}

	if !validObjectStorageBucketLabel(label) {
		return mcp.NewToolResultError("label must be a valid bucket label"), nil
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

	page, err := client.ListObjectStorageBucketContentsProto(ctx, region, label, params)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list contents of bucket '%s' in region '%s': %v", label, region, err)), nil
	}

	return formatBucketContentsResponse(page, prefix, delimiter)
}

func formatBucketContentsResponse(page *linode.ObjectStorageBucketContentsPage, prefix, delimiter string) (*mcp.CallToolResult, error) {
	var count int32
	if n := len(page.Objects); n <= math.MaxInt32 {
		count = int32(n)
	}

	response := &linodev1.ObjectStorageObjectListResponse{
		Count:       count,
		IsTruncated: page.IsTruncated,
		Objects:     page.Objects,
	}

	if page.NextMarker != "" {
		response.NextMarker = &page.NextMarker
	}

	var filters []string
	if prefix != "" {
		filters = append(filters, "prefix="+prefix)
	}

	if delimiter != "" {
		filters = append(filters, "delimiter="+delimiter)
	}

	if len(filters) > 0 {
		filter := strings.Join(filters, ", ")
		response.Filter = &filter
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeObjectStorageEndpointListTool creates a tool for listing Object Storage endpoints.
func NewLinodeObjectStorageEndpointListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_object_storage_endpoint_list",
		"Lists Object Storage endpoints across regions",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.ObjectStorageEndpoint, error) {
			return client.ListObjectStorageEndpointsProto(ctx)
		},
		nil,
		objectStorageEndpointListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_endpoint_list",
		"Lists Object Storage endpoints across regions",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageEndpointListInput"),
	)

	return tool, profiles.CapRead, handler
}

func objectStorageEndpointListResponse(items []*linodev1.ObjectStorageEndpoint, count int32, filter *string) *linodev1.ObjectStorageEndpointListResponse {
	return &linodev1.ObjectStorageEndpointListResponse{Count: count, Filter: filter, Endpoints: items}
}

// NewLinodeObjectStorageTypeListTool creates a tool for listing Object Storage types and pricing.
func NewLinodeObjectStorageTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_object_storage_type_list",
		"Lists Object Storage types and pricing information",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.LinodeType, error) {
			return client.ListObjectStorageTypesProto(ctx)
		},
		nil,
		objectStorageTypeListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_type_list",
		"Lists Object Storage types and pricing information",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageTypeListInput"),
	)

	return tool, profiles.CapRead, handler
}

func objectStorageTypeListResponse(items []*linodev1.LinodeType, count int32, filter *string) *linodev1.ObjectStorageTypeListResponse {
	return &linodev1.ObjectStorageTypeListResponse{Count: count, Filter: filter, Types: items}
}

// NewLinodeObjectStorageQuotasListTool creates a tool for listing Object Storage quotas.
func NewLinodeObjectStorageQuotasListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_object_storage_quota_list",
		"Lists Object Storage quotas on the account",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.ObjectStorageQuota, error) {
			return client.ListObjectStorageQuotasProto(ctx)
		},
		nil,
		objectStorageQuotaListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_quota_list",
		"Lists Object Storage quotas on the account",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageQuotaListInput"),
	)

	return tool, profiles.CapRead, handler
}

func objectStorageQuotaListResponse(items []*linodev1.ObjectStorageQuota, count int32, filter *string) *linodev1.ObjectStorageQuotaListResponse {
	return &linodev1.ObjectStorageQuotaListResponse{Count: count, Filter: filter, Quotas: items}
}

// NewLinodeObjectStorageKeyListTool creates a tool for listing Object Storage access keys.
func NewLinodeObjectStorageKeyListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListTool(
		cfg,
		"linode_object_storage_key_list",
		"Lists all Object Storage access keys for the authenticated user",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.ObjectStorageKey, error) {
			return client.ListObjectStorageKeysProto(ctx)
		},
		nil,
		objectStorageKeyListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_key_list",
		"Lists all Object Storage access keys for the authenticated user",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageKeyListInput"),
	)

	return tool, profiles.CapRead, handler
}

func objectStorageKeyListResponse(items []*linodev1.ObjectStorageKey, count int32, filter *string) *linodev1.ObjectStorageKeyListResponse {
	return &linodev1.ObjectStorageKeyListResponse{Count: count, Filter: filter, Keys: items}
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
	keyID, validationMessage := requiredIDArgument(request, "key_id")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
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
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_quota_usage_get",
		"Gets usage data for a specific Object Storage quota",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageQuotaUsageGetInput"),
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

	usage, err := client.GetObjectStorageQuotaUsageProto(ctx, quotaID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage quota usage for %q: %v", quotaID, err)), nil
	}

	return MarshalProtoToolResponse(usage)
}

// NewLinodeObjectStorageTransferTool creates a tool for getting Object Storage transfer usage.
func NewLinodeObjectStorageTransferTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_transfer_get",
		"Gets Object Storage outbound data transfer usage for the current month",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageTransferGetInput"),
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

	transfer, err := client.GetObjectStorageTransferProto(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage transfer usage: %v", err)), nil
	}

	return MarshalProtoToolResponse(transfer)
}

// NewLinodeObjectStorageQuotaGetTool creates a tool for getting a single Object Storage quota.
func NewLinodeObjectStorageQuotaGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_quota_get",
		"Gets details about a single Object Storage quota by ID",
		toolschemas.Schema("linode.mcp.v1.ObjectStorageQuotaGetInput"),
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

	quota, err := client.GetObjectStorageQuotaProto(ctx, objQuotaID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Object Storage quota %s: %v", objQuotaID, err)), nil
	}

	return MarshalProtoToolResponse(quota)
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
	tool := mcp.NewToolWithRawSchema(
		"linode_object_storage_presigned_url_create",
		"Generates a presigned URL for accessing an object in Object Storage. "+
			"Use method=GET to create a download URL, method=PUT to create an upload URL.",
		toolschemas.Schema("linode.mcp.v1.PresignedURLCreateInput"),
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
	// Presigned method is case-insensitive: GET/PUT are the canonical S3 verbs the
	// schema advertises, but a caller passing "get" must still work, so normalize
	// to uppercase before the enum check and send the canonical form.
	method := strings.ToUpper(request.GetString("method", ""))
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

	if msg := requiredEnumChoiceValue(method, "method", linodev1.PresignedURLMethod_Value_value); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	if err := validateExpiresIn(expiresIn); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.PresignedURLRequest{
		Method:    method,
		Name:      name,
		ExpiresIn: expiresIn,
	}

	result, err := client.CreatePresignedURLProto(ctx, region, label, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to generate presigned URL for '%s' in bucket '%s': %v", name, label, err)), nil
	}

	return MarshalProtoToolResponse(result)
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
