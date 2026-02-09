//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeVolumesListTool creates a tool for listing Linode block storage volumes.
func NewLinodeVolumesListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_volumes_list",
		mcp.WithDescription("Lists all block storage volumes for the authenticated user with optional filtering by region or label"),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("region",
			mcp.Description("Filter volumes by region (e.g., 'us-east', 'eu-west')"),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter volumes where label contains this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeVolumesListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeVolumesListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	regionFilter := request.GetString("region", "")
	labelContains := request.GetString("label_contains", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	volumes, err := client.ListVolumes(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve Linode volumes: %v", err)), nil
	}

	if regionFilter != "" {
		volumes = filterVolumesByRegion(volumes, regionFilter)
	}

	if labelContains != "" {
		volumes = filterVolumesByLabel(volumes, labelContains)
	}

	return formatVolumesResponse(volumes, regionFilter, labelContains)
}

func filterVolumesByRegion(volumes []linode.Volume, regionFilter string) []linode.Volume {
	filtered := make([]linode.Volume, 0, len(volumes))

	regionFilter = strings.ToLower(regionFilter)

	for _, volume := range volumes {
		if strings.ToLower(volume.Region) == regionFilter {
			filtered = append(filtered, volume)
		}
	}

	return filtered
}

func filterVolumesByLabel(volumes []linode.Volume, labelContains string) []linode.Volume {
	filtered := make([]linode.Volume, 0, len(volumes))

	labelContains = strings.ToLower(labelContains)

	for _, volume := range volumes {
		if strings.Contains(strings.ToLower(volume.Label), labelContains) {
			filtered = append(filtered, volume)
		}
	}

	return filtered
}

func formatVolumesResponse(volumes []linode.Volume, regionFilter, labelContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count   int             `json:"count"`
		Filter  string          `json:"filter,omitempty"`
		Volumes []linode.Volume `json:"volumes"`
	}{
		Count:   len(volumes),
		Volumes: volumes,
	}

	var filters []string
	if regionFilter != "" {
		filters = append(filters, "region="+regionFilter)
	}

	if labelContains != "" {
		filters = append(filters, "label_contains="+labelContains)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return marshalToolResponse(response)
}
