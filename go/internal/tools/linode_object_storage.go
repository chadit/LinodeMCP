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

// NewLinodeObjectStorageBucketsListTool creates a tool for listing Object Storage buckets.
func NewLinodeObjectStorageBucketsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_buckets_list",
		mcp.WithDescription("Lists all Object Storage buckets across all regions for the authenticated user"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageBucketsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageBucketsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageBucketGetTool creates a tool for getting a specific bucket.
func NewLinodeObjectStorageBucketGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_get",
		mcp.WithDescription("Gets details about a specific Object Storage bucket by region and label"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
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
	environment := request.GetString("environment", "")
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

	jsonResponse, err := json.MarshalIndent(bucket, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageBucketContentsTool creates a tool for listing objects in a bucket.
func NewLinodeObjectStorageBucketContentsTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_bucket_contents",
		mcp.WithDescription("Lists objects in an Object Storage bucket with optional prefix/delimiter filtering and pagination"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
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
	environment := request.GetString("environment", "")
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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageClustersListTool creates a tool for listing Object Storage clusters.
func NewLinodeObjectStorageClustersListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_clusters_list",
		mcp.WithDescription("Lists available Object Storage clusters/regions where buckets can be created"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageClustersListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageClustersListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeObjectStorageTypeListTool creates a tool for listing Object Storage types and pricing.
func NewLinodeObjectStorageTypeListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_object_storage_type_list",
		mcp.WithDescription("Lists Object Storage types and pricing information"),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleObjectStorageTypeListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleObjectStorageTypeListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")

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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
