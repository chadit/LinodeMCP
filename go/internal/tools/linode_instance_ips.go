package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeInstanceIPsListTool creates a tool for listing all IP addresses for a Linode instance.
func NewLinodeInstanceIPsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_ips_list",
		"Lists all IP addresses (IPv4 and IPv6) for a Linode instance",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
		},
		handleInstanceIPsListRequest,
	)
}

func handleInstanceIPsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ips, err := client.ListInstanceIPs(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list IPs for instance %d: %v", linodeID, err)), nil
	}

	return MarshalToolResponse(ips)
}

// NewLinodeInstanceIPGetTool creates a tool for retrieving a specific IP address for a Linode instance.
func NewLinodeInstanceIPGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_ip_get",
		"Retrieves details of a specific IP address for a Linode instance",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("address", mcp.Required(),
				mcp.Description("The IP address to retrieve (e.g. 203.0.113.1)")),
		},
		handleInstanceIPGetRequest,
	)
}

func handleInstanceIPGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	address := request.GetString("address", "")
	if address == "" {
		return mcp.NewToolResultError("address is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ip, err := client.GetInstanceIP(ctx, linodeID, address)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get IP %s for instance %d: %v", address, linodeID, err)), nil
	}

	return MarshalToolResponse(ip)
}

// NewLinodeInstanceIPAllocateTool creates a tool for allocating a new IP address for a Linode instance.
func NewLinodeInstanceIPAllocateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_ip_allocate",
		"Allocates a new IP address for a Linode instance. WARNING: Additional IPs may incur charges.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("The type of IP address to allocate (e.g. 'ipv4')")),
			mcp.WithBoolean("public", mcp.Required(),
				mcp.Description("Whether the IP address should be public (true) or private (false)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP allocation. Additional IPs may incur charges.")),
		},
		handleInstanceIPAllocateRequest,
	)
}

func handleInstanceIPAllocateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This allocates a new IP address which may incur charges. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	ipType := request.GetString("type", "")
	if ipType == "" {
		return mcp.NewToolResultError("type is required (e.g. 'ipv4')"), nil
	}

	public := request.GetBool("public", false)

	req := linode.AllocateIPRequest{
		Type:   ipType,
		Public: public,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, err := client.AllocateInstanceIP(ctx, linodeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to allocate IP for instance %d: %v", linodeID, err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		IP      *linode.IPAddress `json:"ip"`
	}{
		Message: fmt.Sprintf("IP %s allocated for instance %d", ipAddr.Address, linodeID),
		IP:      ipAddr,
	}

	return MarshalToolResponse(response)
}

// NewLinodeInstanceIPDeleteTool creates a tool for removing an IP address from a Linode instance.
func NewLinodeInstanceIPDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newToolWithHandler(cfg,
		"linode_instance_ip_delete",
		"Removes an IP address from a Linode instance. WARNING: This permanently removes the IP and is irreversible.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode instance")),
			mcp.WithString("address", mcp.Required(),
				mcp.Description("The IP address to remove (e.g. 203.0.113.1)")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP removal. This action is irreversible.")),
		},
		handleInstanceIPDeleteRequest,
	)
}

func handleInstanceIPDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if result := RequireConfirm(request, "This permanently removes the IP address and is irreversible. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	address := request.GetString("address", "")
	if address == "" {
		return mcp.NewToolResultError("address is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := client.DeleteInstanceIP(ctx, linodeID, address); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to remove IP %s from instance %d: %v", address, linodeID, err)), nil
	}

	return MarshalToolResponse(struct {
		Message  string `json:"message"`
		LinodeID int    `json:"linode_id"`
		Address  string `json:"address"`
	}{
		Message:  fmt.Sprintf("IP %s removed from instance %d", address, linodeID),
		LinodeID: linodeID,
		Address:  address,
	})
}
