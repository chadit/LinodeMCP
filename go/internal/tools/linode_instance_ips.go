package tools

import (
	"context"
	"fmt"
	"net"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeInstanceIPListTool creates a tool for listing all IP addresses for a Linode instance.
func NewLinodeInstanceIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_ip_list",
		"Lists all IP addresses (IPv4 and IPv6) for a Linode instance",
		toolschemas.Schema("linode.mcp.v1.InstanceIPListInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceIPsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	ips, err := client.ListInstanceIPsProto(ctx, linodeID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list IPs for instance %d: %v", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(ips)
}

// NewLinodeInstanceIPGetTool creates a tool for retrieving a specific IP address for a Linode instance.
func NewLinodeInstanceIPGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_ip_get",
		"Retrieves details of a specific IP address for a Linode instance",
		toolschemas.Schema("linode.mcp.v1.InstanceIPGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceIPGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
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

	if net.ParseIP(address) == nil {
		return mcp.NewToolResultError("address must be a valid IP address"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ip, err := client.GetInstanceIPProto(ctx, linodeID, address)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get IP %s for instance %d: %v", address, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(ip)
}

// NewLinodeInstanceIPAllocateTool creates a tool for allocating a new IP address for a Linode instance.
func NewLinodeInstanceIPAllocateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_ip_allocate",
		"Allocates a new IP address for a Linode instance. WARNING: Additional IPs may incur charges.",
		toolschemas.Schema("linode.mcp.v1.InstanceIPAllocateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceIPAllocateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleInstanceIPAllocateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	ipType := request.GetString("type", "")

	if msg := enumChoiceError(ipType, "type", linodev1.InstanceIPType_Value_value); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	if IsDryRun(request) {
		if linodeID == 0 {
			return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
		}

		if ipType == "" {
			return mcp.NewToolResultError("type is required (e.g. 'ipv4')"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_ip_allocate", httpMethodPost,
			fmt.Sprintf("/linode/instances/%d/ips", linodeID), nil)
	}

	if result := RequireConfirm(request, "This allocates a new IP address which may incur charges. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	if ipType == "" {
		return mcp.NewToolResultError("type is required (e.g. 'ipv4')"), nil
	}

	public, validationMessage := requiredNetworkingBoolArg(request.GetArguments(), "public")
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	req := linode.AllocateIPRequest{
		Type:   ipType,
		Public: public,
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, err := client.AllocateInstanceIPProto(ctx, linodeID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to allocate IP for instance %d: %v", linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.IPAddressWriteResponse{
		Message: fmt.Sprintf("IP %s allocated for instance %d", ipAddr.GetAddress(), linodeID),
		Ip:      ipAddr,
	})
}

// NewLinodeInstanceIPUpdateRDNSTool creates a tool for updating the RDNS on a Linode instance IP address.
func NewLinodeInstanceIPUpdateRDNSTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_ip_update",
		"Updates the reverse DNS for a specific IP address on a Linode instance.",
		toolschemas.Schema("linode.mcp.v1.InstanceIPUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceIPUpdateRDNSRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateInstanceIPRDNSArgs validates the IP RDNS update args, returning an
// error message or "". Shared by the real path and the dry-run preview.
func validateInstanceIPRDNSArgs(linodeID int, address, rdns string) string {
	if linodeID == 0 {
		return ErrLinodeIDRequired.Error()
	}

	if address == "" {
		return "address is required"
	}

	if net.ParseIP(address) == nil {
		return "address must be a valid IP address"
	}

	if rdns == "" {
		return "rdns is required"
	}

	return ""
}

func handleInstanceIPUpdateRDNSRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	address := request.GetString("address", "")
	rdns := request.GetString("rdns", "")

	if IsDryRun(request) {
		if msg := validateInstanceIPRDNSArgs(linodeID, address, rdns); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_instance_ip_update", "PUT",
			fmt.Sprintf("/linode/instances/%d/ips/%s", linodeID, address),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetInstanceIP(ctx, linodeID, address)
			})
	}

	if result := RequireConfirm(request, "This updates reverse DNS for the IP address. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateInstanceIPRDNSArgs(linodeID, address, rdns); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateIPRDNSRequest{RDNS: &rdns}

	ipAddr, err := client.UpdateInstanceIPProto(ctx, linodeID, address, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to assign RDNS for IP %s on instance %d: %v", address, linodeID, err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.IPAddressWriteResponse{
		Message: fmt.Sprintf("RDNS for IP %s updated on instance %d", address, linodeID),
		Ip:      ipAddr,
	})
}

// NewLinodeInstanceIPDeleteTool creates a tool for removing an IP address from a Linode instance.
func NewLinodeInstanceIPDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_instance_ip_delete",
		"Removes an IP address from a Linode instance. WARNING: This permanently removes the IP and is irreversible."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.InstanceIPDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleInstanceIPDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleInstanceIPDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	linodeID := request.GetInt("linode_id", 0)
	if linodeID == 0 {
		return mcp.NewToolResultError(ErrLinodeIDRequired.Error()), nil
	}

	address := request.GetString("address", "")
	if address == "" {
		return mcp.NewToolResultError("address is required"), nil
	}

	if net.ParseIP(address) == nil {
		return mcp.NewToolResultError("address must be a valid IP address"), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_instance_ip_delete",
		Method:         httpMethodDelete,
		Path:           fmt.Sprintf("/linode/instances/%d/ips/%s", linodeID, address),
		ConfirmMessage: "This permanently removes the IP address and is irreversible. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetInstanceIP(ctx, linodeID, address)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteInstanceIP(ctx, linodeID, address)
		},
		Success: func() proto.Message {
			return &linodev1.InstanceIPDeleteResponse{
				Message:  fmt.Sprintf("IP %s removed from instance %d", address, linodeID),
				LinodeId: linodeIDToInt32(linodeID),
				Address:  address,
			}
		},
		// An IP address record carries no cosmetic timestamp, so the whole
		// state is hashed; the unknown "InstanceIP" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("InstanceIP"),
	})
}
