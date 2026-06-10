package tools

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	instanceFirewallsPageSizeMin = 25
	instanceFirewallsPageSizeMax = 500
)

// NewLinodeInstanceFirewallListTool creates a tool for listing Cloud Firewalls assigned to a Linode instance.
func NewLinodeInstanceFirewallListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_firewall_list",
		"Lists Cloud Firewalls assigned to a Linode instance with optional pagination.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
		},
		handleInstanceFirewallsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceFirewallsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := instanceConfigsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewalls, err := client.ListInstanceFirewalls(ctx, linodeID, page, pageSize)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list firewalls for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Count     int               `json:"count"`
		Firewalls []linode.Firewall `json:"firewalls"`
	}{
		Count:     len(firewalls),
		Firewalls: firewalls,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceInterfaceFirewallsListTool creates a tool for listing Cloud Firewalls assigned to a Linode interface.
func NewLinodeInstanceInterfaceFirewallsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_interface_firewall_list",
		"Lists Cloud Firewalls assigned to a specific Linode interface.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithNumber("interface_id", mcp.Required(),
				mcp.Description("The ID of the Linode interface")),
		},
		handleInstanceInterfaceFirewallsListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleInstanceInterfaceFirewallsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	interfaceID, validationMessage := instanceInterfaceIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewalls, err := client.ListInstanceInterfaceFirewalls(ctx, linodeID, interfaceID)
	if err != nil {
		return mcp.NewToolResultError(formatInstanceInterfaceFirewallsListError(linodeID, interfaceID, err)), nil
	}

	response := struct {
		Count       int               `json:"count"`
		LinodeID    int               `json:"linode_id"`
		InterfaceID int               `json:"interface_id"`
		Firewalls   []linode.Firewall `json:"firewalls"`
	}{
		Count:       len(firewalls),
		LinodeID:    linodeID,
		InterfaceID: interfaceID,
		Firewalls:   firewalls,
	}

	return MarshalToolResponse(response)
}

func formatInstanceInterfaceFirewallsListError(linodeID, interfaceID int, err error) string {
	return "Failed to list firewalls for interface " + strconv.Itoa(interfaceID) + " on instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

// NewLinodeInstanceFirewallsUpdateTool creates a tool for replacing firewall assignments on a Linode instance.
func NewLinodeInstanceFirewallsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_instance_firewall_update",
		"Replaces the Cloud Firewall assignments for a Linode instance. Pass an empty firewall_ids list to remove all assignments.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithArray("firewall_ids", mcp.Required(),
				mcp.Description("Complete list of firewall IDs to assign to the Linode. Use an empty list to remove all firewall assignments.")),
			mcp.WithNumber("page", mcp.Description("Page of results to return (optional, minimum 1).")),
			mcp.WithNumber("page_size", mcp.Description("Number of results per page (optional, 25-500).")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm firewall assignment changes. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleInstanceFirewallsUpdateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleInstanceFirewallsUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID, validationMessage := instanceConfigLinodeIDFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	page, pageSize, validationMessage := instanceFirewallsPaginationFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	if IsDryRun(request) {
		if _, idsMessage := instanceFirewallsIDsFromTool(request); idsMessage != "" {
			return mcp.NewToolResultError(idsMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_firewall_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/firewalls", linodeID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.ListInstanceFirewalls(ctx, linodeID, page, pageSize)
			})
	}

	if result := RequireConfirm(request, "This replaces firewall assignments for a Linode instance. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	firewallIDs, validationMessage := instanceFirewallsIDsFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	firewalls, err := client.UpdateInstanceFirewalls(ctx, linodeID, page, pageSize, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: firewallIDs})
	if err != nil {
		return mcp.NewToolResultError(formatInstanceFirewallsUpdateError(linodeID, err)), nil
	}

	response := struct {
		Count     int               `json:"count"`
		Firewalls []linode.Firewall `json:"firewalls"`
	}{
		Count:     len(firewalls),
		Firewalls: firewalls,
	}

	return MarshalToolResponse(response)
}

func formatInstanceFirewallsUpdateError(linodeID int, err error) string {
	return "Failed to update firewall assignments for instance " + strconv.Itoa(linodeID) + ": " + err.Error()
}

func instanceFirewallsIDsFromTool(request *mcp.CallToolRequest) ([]int, string) {
	raw, exists := request.GetArguments()["firewall_ids"]
	if !exists {
		return nil, "firewall_ids is required"
	}

	rawIDs, ok := raw.([]any)
	if !ok {
		return nil, "firewall_ids must be an array of positive integers"
	}

	firewallIDs := make([]int, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		firewallID, ok := numberArgToInt(rawID)
		if !ok || firewallID <= 0 {
			return nil, "firewall_ids entries must be positive integers"
		}

		firewallIDs = append(firewallIDs, firewallID)
	}

	return firewallIDs, ""
}

func instanceFirewallsPaginationFromTool(request *mcp.CallToolRequest) (int, int, string) {
	args := request.GetArguments()

	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", instanceFirewallsPageSizeMin, instanceFirewallsPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}
