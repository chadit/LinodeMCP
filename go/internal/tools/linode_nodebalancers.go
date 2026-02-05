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

// NewLinodeNodeBalancersListTool creates a tool for listing NodeBalancers.
func NewLinodeNodeBalancersListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancers_list",
		mcp.WithDescription("Lists all NodeBalancers on your account. Can filter by region or label."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithString("region",
			mcp.Description("Filter by region ID (e.g., us-east, eu-west)"),
		),
		mcp.WithString("label_contains",
			mcp.Description("Filter NodeBalancers by label containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancersListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancersListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
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

	nodeBalancers, err := client.ListNodeBalancers(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve NodeBalancers: %v", err)), nil
	}

	if regionFilter != "" {
		nodeBalancers = filterNodeBalancersByRegion(nodeBalancers, regionFilter)
	}

	if labelContains != "" {
		nodeBalancers = filterNodeBalancersByLabel(nodeBalancers, labelContains)
	}

	return formatNodeBalancersResponse(nodeBalancers, regionFilter, labelContains)
}

func filterNodeBalancersByRegion(nodeBalancers []linode.NodeBalancer, regionFilter string) []linode.NodeBalancer {
	var filtered []linode.NodeBalancer

	regionFilter = strings.ToLower(regionFilter)

	for _, nb := range nodeBalancers {
		if strings.ToLower(nb.Region) == regionFilter {
			filtered = append(filtered, nb)
		}
	}

	return filtered
}

func filterNodeBalancersByLabel(nodeBalancers []linode.NodeBalancer, labelContains string) []linode.NodeBalancer {
	var filtered []linode.NodeBalancer

	labelContains = strings.ToLower(labelContains)

	for _, nb := range nodeBalancers {
		if strings.Contains(strings.ToLower(nb.Label), labelContains) {
			filtered = append(filtered, nb)
		}
	}

	return filtered
}

func formatNodeBalancersResponse(nodeBalancers []linode.NodeBalancer, regionFilter, labelContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count         int                   `json:"count"`
		Filter        string                `json:"filter,omitempty"`
		NodeBalancers []linode.NodeBalancer `json:"nodebalancers"`
	}{
		Count:         len(nodeBalancers),
		NodeBalancers: nodeBalancers,
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

	jsonResponse, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// NewLinodeNodeBalancerGetTool creates a tool for getting a single NodeBalancer.
func NewLinodeNodeBalancerGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_nodebalancer_get",
		mcp.WithDescription("Gets detailed information about a specific NodeBalancer by its ID."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("nodebalancer_id",
			mcp.Required(),
			mcp.Description("The ID of the NodeBalancer to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNodeBalancerGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeNodeBalancerGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	nodeBalancerID := request.GetInt("nodebalancer_id", 0)

	if nodeBalancerID == 0 {
		return mcp.NewToolResultError("nodebalancer_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	nodeBalancer, err := client.GetNodeBalancer(ctx, nodeBalancerID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve NodeBalancer %d: %v", nodeBalancerID, err)), nil
	}

	jsonResponse, err := json.MarshalIndent(nodeBalancer, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}
