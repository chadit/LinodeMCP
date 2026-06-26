package tools

import (
	"context"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeNodeBalancerConfigDeleteTool creates a tool for deleting a NodeBalancer config.
func NewLinodeNodeBalancerConfigDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_nodebalancer_config_delete",
		"Deletes a config from a NodeBalancer. WARNING: This can disrupt load balancer traffic.",
		[]mcp.ToolOption{
			mcp.WithNumber("nodebalancer_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer whose config should be deleted")),
			mcp.WithNumber("config_id", mcp.Required(),
				mcp.Description("The ID of the NodeBalancer config to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm config deletion. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNodeBalancerConfigDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeNodeBalancerConfigDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	nodeBalancerID, validationMessage := nodeBalancerIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	configID, validationMessage := configIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		client, err := prepareClient(request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		state, fetchErr := fetchNodeBalancerConfigForDryRun(ctx, client, nodeBalancerID, configID)
		if fetchErr != "" {
			return mcp.NewToolResultError(fetchErr), nil
		}

		details := nodebalancerConfigDeleteDependencyWalk(ctx, client, nodeBalancerID, configID)

		return BuildDryRunResponseDetailed(
			"linode_nodebalancer_config_delete",
			request.GetString(paramEnvironment, ""),
			httpMethodDelete,
			"/nodebalancers/"+strconv.Itoa(nodeBalancerID)+"/configs/"+strconv.Itoa(configID),
			state,
			&details,
		)
	}

	if result := RequireConfirm(request, destroyConfirmMessage); result != nil {
		return result, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	deleteFailureMessage := deleteNodeBalancerConfig(ctx, client, nodeBalancerID, configID)
	if deleteFailureMessage != "" {
		return mcp.NewToolResultError("Failed to delete config " + strconv.Itoa(configID) + " from NodeBalancer " + strconv.Itoa(nodeBalancerID) + ": " + deleteFailureMessage), nil
	}

	response := struct {
		Message        string `json:"message"`
		NodeBalancerID int    `json:"nodebalancer_id"`
		ConfigID       int    `json:"config_id"`
	}{
		Message:        "Config " + strconv.Itoa(configID) + " removed from NodeBalancer " + strconv.Itoa(nodeBalancerID) + " successfully",
		NodeBalancerID: nodeBalancerID,
		ConfigID:       configID,
	}

	return MarshalToolResponse(response)
}

func deleteNodeBalancerConfig(ctx context.Context, client *linode.Client, nodeBalancerID, configID int) string {
	deleteErr := client.DeleteNodeBalancerConfig(ctx, nodeBalancerID, configID)
	if deleteErr != nil {
		return deleteErr.Error()
	}

	return ""
}

func fetchNodeBalancerConfigForDryRun(ctx context.Context, client *linode.Client, nodeBalancerID, configID int) (linode.NodeBalancerConfig, string) {
	configs, err := client.ListNodeBalancerConfigs(ctx, nodeBalancerID, 0, 0)
	if err != nil {
		return linode.NodeBalancerConfig{}, "Failed to fetch state for dry-run: " + err.Error()
	}

	for i := range configs {
		if configs[i].ID == configID {
			return configs[i], ""
		}
	}

	return linode.NodeBalancerConfig{}, "Failed to fetch state for dry-run: config " + strconv.Itoa(configID) + " not found on NodeBalancer " + strconv.Itoa(nodeBalancerID)
}
