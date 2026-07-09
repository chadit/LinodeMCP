package tools

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	instanceFirewallsPageSizeMin = 25
	instanceFirewallsPageSizeMax = 500
)

// NewLinodeInstanceFirewallListTool creates a tool for listing Cloud Firewalls assigned to a Linode instance.
func NewLinodeInstanceFirewallListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	// The raw-schema tool advertises the generated InstanceFirewallListInput; the
	// list helper still builds the fetch/paginate/serialize handler, which is
	// schema-source independent.
	_, handler := newProtoListToolSubresourcePaginated(
		cfg,
		"linode_instance_firewall_list",
		"Lists Cloud Firewalls assigned to a Linode instance with optional pagination.",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		instanceConfigsPaginationFromTool,
		func(ctx context.Context, client *linode.Client, linodeID, page, pageSize int) ([]*linodev1.Firewall, error) {
			return client.ListInstanceFirewallsProto(ctx, linodeID, page, pageSize)
		},
		nil,
		instanceFirewallListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_instance_firewall_list",
		"Lists Cloud Firewalls assigned to a Linode instance with optional pagination.",
		toolschemas.Schema("linode.mcp.v1.InstanceFirewallListInput"),
	)

	return tool, profiles.CapRead, handler
}

func instanceFirewallListResponse(items []*linodev1.Firewall, count int32, filter *string) *linodev1.FirewallListResponse {
	return &linodev1.FirewallListResponse{Count: count, Filter: filter, Firewalls: items}
}

// NewLinodeInstanceInterfaceFirewallsListTool creates a tool for listing Cloud Firewalls assigned to a Linode interface.
func NewLinodeInstanceInterfaceFirewallsListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	_, handler := newProtoListToolSubresource2(
		cfg,
		"linode_instance_interface_firewall_list",
		"Lists Cloud Firewalls assigned to a specific Linode interface.",
		protoListPathID{
			option: mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			parse: instanceConfigLinodeIDFromTool,
		},
		protoListPathID{
			option: mcp.WithNumber("interface_id", mcp.Required(),
				mcp.Description("The ID of the Linode interface")),
			parse: instanceInterfaceIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, linodeID, interfaceID int) ([]*linodev1.Firewall, error) {
			return client.ListInstanceInterfaceFirewallsProto(ctx, linodeID, interfaceID)
		},
		nil,
		instanceFirewallListResponse,
	)

	tool := mcp.NewToolWithRawSchema(
		"linode_instance_interface_firewall_list",
		"Lists Cloud Firewalls assigned to a specific Linode interface.",
		toolschemas.Schema("linode.mcp.v1.InstanceInterfaceFirewallListInput"),
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeInstanceFirewallsUpdateTool creates a tool for replacing firewall assignments on a Linode instance.
func NewLinodeInstanceFirewallsUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_firewall_update",
		"Replaces the Cloud Firewall assignments for a Linode instance. Pass an empty firewall_ids list to remove all assignments.",
		toolschemas.Schema("linode.mcp.v1.InstanceFirewallUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceFirewallsUpdateRequest(ctx, &request, cfg)
	}

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

	firewalls, err := client.UpdateInstanceFirewallsProto(ctx, linodeID, page, pageSize, &linode.UpdateInstanceFirewallsRequest{FirewallIDs: firewallIDs})
	if err != nil {
		return mcp.NewToolResultError(formatInstanceFirewallsUpdateError(linodeID, err)), nil
	}

	var count int32
	if n := len(firewalls); n <= math.MaxInt32 {
		count = int32(n)
	}

	return MarshalProtoToolResponse(&linodev1.FirewallListResponse{
		Count:     count,
		Firewalls: firewalls,
	})
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
