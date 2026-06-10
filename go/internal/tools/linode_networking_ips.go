package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	paramSkipIPv6RDNS = "skip_ipv6_rdns"
	paramAddress      = "address"
	paramRDNS         = "rdns"
	paramIPs          = "ips"
)

// NewLinodeNetworkingIPListTool creates a tool for listing account IP addresses.
func NewLinodeNetworkingIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_list",
		"Lists IP addresses on the account. Set skip_ipv6_rdns to true to skip IPv6 reverse DNS lookups.",
		[]mcp.ToolOption{
			mcp.WithBoolean(paramSkipIPv6RDNS, mcp.Description("Skip IPv6 reverse DNS lookups (optional).")),
		},
		handleLinodeNetworkingIPListRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeNetworkingIPListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	skipIPv6RDNS, validationMessage := optionalNetworkingBoolArg(args, paramSkipIPv6RDNS)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ips, err := client.ListNetworkingIPs(ctx, skipIPv6RDNS)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve networking IPs: %v", err)), nil
	}

	return MarshalToolResponse(ips)
}

// NewLinodeNetworkingIPGetTool creates a tool for retrieving one account-level IP address.
func NewLinodeNetworkingIPGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_get",
		"Gets details for one account-level IP address.",
		[]mcp.ToolOption{
			mcp.WithString(paramAddress, mcp.Required(),
				mcp.Description("The IPv4 or IPv6 address to retrieve.")),
		},
		handleLinodeNetworkingIPGetRequest,
	)

	return tool, profiles.CapRead, handler
}

func handleLinodeNetworkingIPGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	address, validationMessage := requiredNetworkingIPAddressArg(request.GetArguments(), paramAddress)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, err := client.GetNetworkingIP(ctx, address)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve networking IP %s: %v", address, err)), nil
	}

	return MarshalToolResponse(ipAddr)
}

// NewLinodeNetworkingIPUpdateRDNSTool creates a tool for updating account-level IP reverse DNS.
func NewLinodeNetworkingIPUpdateRDNSTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_update",
		"Updates reverse DNS for one account-level IP address.",
		[]mcp.ToolOption{
			mcp.WithString(paramAddress, mcp.Required(),
				mcp.Description("The IPv4 or IPv6 address to update.")),
			mcp.WithString(paramRDNS, mcp.Required(),
				mcp.Description("The reverse DNS value to set.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm changing reverse DNS. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNetworkingIPUpdateRDNSRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPUpdateRDNSRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		address, validationMessage := requiredNetworkingIPAddressArg(request.GetArguments(), paramAddress)
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		if _, rdnsMessage := requiredStringArg(request.GetArguments(), paramRDNS); rdnsMessage != "" {
			return mcp.NewToolResultError(rdnsMessage), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_networking_ip_update", "PUT",
			"/networking/ips/"+address,
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetNetworkingIP(ctx, address) },
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return networkingIPUpdateRDNSSideEffects(ctx, request.GetString(paramRDNS, ""))
			})
	}

	if result := RequireConfirm(request, "This updates reverse DNS for an IP address. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	address, validationMessage := requiredNetworkingIPAddressArg(request.GetArguments(), paramAddress)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	rdns, validationMessage := requiredStringArg(request.GetArguments(), paramRDNS)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, failureMessage := updateNetworkingIPRDNS(ctx, client, address, rdns)
	if failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	return MarshalToolResponse(struct {
		Message string            `json:"message"`
		IP      *linode.IPAddress `json:"ip"`
	}{
		Message: fmt.Sprintf("Networking IP %s RDNS updated", address),
		IP:      ipAddr,
	})
}

func updateNetworkingIPRDNS(ctx context.Context, client *linode.Client, address, rdns string) (*linode.IPAddress, string) {
	ipAddr, err := client.UpdateNetworkingIP(ctx, address, linode.UpdateNetworkingIPRequest{RDNS: rdns})
	if err != nil {
		return nil, "Failed to update networking IP " + address + " RDNS: " + err.Error()
	}

	return ipAddr, ""
}

// NewLinodeNetworkingIPAllocateTool creates a tool for allocating an account-level IP address.
func NewLinodeNetworkingIPAllocateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_allocate",
		"Allocates an account-level IP address. WARNING: Additional IPs may incur charges.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the Linode that receives the new IP address.")),
			mcp.WithString("type", mcp.Required(),
				mcp.Description("The type of IP address to allocate, for example ipv4.")),
			mcp.WithBoolean("public", mcp.Required(),
				mcp.Description("Whether the IP address should be public.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP allocation. Additional IPs may incur charges. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNetworkingIPAllocateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPAllocateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if _, validationMessage := networkingIPAllocateRequestFromTool(request.GetArguments()); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_networking_ip_allocate", httpMethodPost, "/networking/ips", nil)
	}

	if result := RequireConfirm(request, "This allocates a new IP address which may incur charges. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := networkingIPAllocateRequestFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipAddr, err := client.AllocateNetworkingIP(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to allocate networking IP: %v", err)), nil
	}

	response := struct {
		Message string            `json:"message"`
		IP      *linode.IPAddress `json:"ip"`
	}{
		Message: fmt.Sprintf("IP %s allocated for Linode %d", ipAddr.Address, req.LinodeID),
		IP:      ipAddr,
	}

	return MarshalToolResponse(response)
}

// NewLinodeNetworkingIPAssignTool creates a tool for assigning IP addresses to Linodes.
func NewLinodeNetworkingIPAssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ip_assign",
		"Assigns IP addresses to Linodes in a region. WARNING: This changes IP ownership assignments.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("The region for the IP assignments.")),
			mcp.WithString("assignments", mcp.Required(),
				mcp.Description("JSON array of assignments, each with address and linode_id.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IP reassignment. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNetworkingIPAssignRequest,
	)

	return tool, profiles.CapWrite, handler
}

// networkingIPWriteSpec describes a bulk networking-IP mutation (assign,
// ipv4 assign, share). These endpoints are POST-only with no single-resource
// GET, so the dry-run preview reports current_state null.
type networkingIPWriteSpec struct {
	ToolName       string
	Path           string
	ConfirmMessage string
	SuccessMessage string
	FailureLabel   string
}

// runNetworkingIPWrite is the shared dry-run/confirm/execute flow for the
// bulk networking-IP mutations. The caller parses+validates its own request
// type and hands in the resulting validation message plus an execute closure
// that captures the parsed request. validationMessage is checked in both the
// dry-run and real paths so a malformed call fails the same way either way.
func runNetworkingIPWrite(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	spec *networkingIPWriteSpec,
	validationMessage string,
	execute func(ctx context.Context, client *linode.Client) (map[string]any, error),
) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreview(ctx, request, cfg, spec.ToolName, httpMethodPost, spec.Path, nil)
	}

	if result := RequireConfirm(request, spec.ConfirmMessage); result != nil {
		return result, nil
	}

	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	response, err := execute(ctx, client)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %v", spec.FailureLabel, err)), nil
	}

	return MarshalToolResponse(struct {
		Message  string         `json:"message"`
		Response map[string]any `json:"response"`
	}{
		Message:  spec.SuccessMessage,
		Response: response,
	})
}

func handleLinodeNetworkingIPAssignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := networkingIPAssignRequestFromTool(request.GetArguments())

	return runNetworkingIPWrite(ctx, request, cfg, &networkingIPWriteSpec{
		ToolName:       "linode_networking_ip_assign",
		Path:           "/networking/ips/assign",
		ConfirmMessage: "This assigns IP addresses to Linodes. Set confirm=true to proceed.",
		SuccessMessage: "Networking IP assignments updated",
		FailureLabel:   "Failed to assign networking IPs",
	}, validationMessage, func(ctx context.Context, client *linode.Client) (map[string]any, error) {
		return client.AssignNetworkingIPs(ctx, req)
	})
}

// NewLinodeNetworkingIPv4AssignTool creates a tool for assigning IPv4 addresses to Linodes.
func NewLinodeNetworkingIPv4AssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ipv4_assign",
		"Assigns IPv4 addresses to Linodes in a region. WARNING: This changes IP ownership assignments.",
		[]mcp.ToolOption{
			mcp.WithString("region", mcp.Required(),
				mcp.Description("The region for the IPv4 assignments.")),
			mcp.WithString("assignments", mcp.Required(),
				mcp.Description("JSON array of assignments, each with address and linode_id.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IPv4 reassignment. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNetworkingIPv4AssignRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPv4AssignRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := networkingIPAssignRequestFromTool(request.GetArguments())

	return runNetworkingIPWrite(ctx, request, cfg, &networkingIPWriteSpec{
		ToolName:       "linode_networking_ipv4_assign",
		Path:           "/networking/ipv4/assign",
		ConfirmMessage: "This assigns IPv4 addresses to Linodes. Set confirm=true to proceed.",
		SuccessMessage: "Networking IPv4 assignments updated",
		FailureLabel:   "Failed to assign networking IPv4s",
	}, validationMessage, func(ctx context.Context, client *linode.Client) (map[string]any, error) {
		return client.AssignNetworkingIPv4s(ctx, req)
	})
}

// NewLinodeNetworkingIPShareTool creates a tool for sharing IP addresses with a primary Linode.
func NewLinodeNetworkingIPShareTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_networking_ipv4_share",
		"Shares IP addresses with a primary Linode. Set ips to a JSON string array; an empty array removes all shared IP addresses.",
		[]mcp.ToolOption{
			mcp.WithNumber("linode_id", mcp.Required(),
				mcp.Description("The ID of the primary Linode that receives the shared IP addresses.")),
			mcp.WithString(paramIPs, mcp.Required(),
				mcp.Description("JSON array of IP addresses or IPv6 ranges to share. Use [] to remove all shared IP addresses.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm changing shared IP assignments. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
		},
		handleLinodeNetworkingIPShareRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPShareRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := networkingIPShareRequestFromTool(request.GetArguments())

	return runNetworkingIPWrite(ctx, request, cfg, &networkingIPWriteSpec{
		ToolName:       "linode_networking_ipv4_share",
		Path:           "/networking/ipv4/share",
		ConfirmMessage: "This changes shared IP assignments. Set confirm=true to proceed.",
		SuccessMessage: "Networking IP sharing updated",
		FailureLabel:   "Failed to share networking IPs",
	}, validationMessage, func(ctx context.Context, client *linode.Client) (map[string]any, error) {
		return client.ShareNetworkingIPs(ctx, req)
	})
}

func networkingIPAssignRequestFromTool(args map[string]any) (linode.AssignNetworkingIPsRequest, string) {
	region, validationMessage := requiredStringArg(args, "region")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
	}

	assignmentsJSON, validationMessage := requiredStringArg(args, "assignments")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
	}

	var assignments []linode.IPAssignment
	if err := json.Unmarshal([]byte(assignmentsJSON), &assignments); err != nil {
		return linode.AssignNetworkingIPsRequest{}, "assignments must be a JSON array of objects with address and linode_id"
	}

	if len(assignments) == 0 {
		return linode.AssignNetworkingIPsRequest{}, "assignments must include at least one assignment"
	}

	for _, assignment := range assignments {
		if assignment.Address == "" {
			return linode.AssignNetworkingIPsRequest{}, "assignment address is required"
		}

		if assignment.LinodeID <= 0 {
			return linode.AssignNetworkingIPsRequest{}, "assignment linode_id must be a positive integer"
		}
	}

	return linode.AssignNetworkingIPsRequest{Region: region, Assignments: assignments}, ""
}

func networkingIPShareRequestFromTool(args map[string]any) (linode.ShareNetworkingIPsRequest, string) {
	linodeID, ok := numberArgToInt(args["linode_id"])
	if !ok || linodeID <= 0 {
		return linode.ShareNetworkingIPsRequest{}, "linode_id must be a positive integer"
	}

	ipsJSON, validationMessage := requiredStringArg(args, paramIPs)
	if validationMessage != "" {
		return linode.ShareNetworkingIPsRequest{}, validationMessage
	}

	var ips []string
	if err := json.Unmarshal([]byte(ipsJSON), &ips); err != nil || ips == nil {
		return linode.ShareNetworkingIPsRequest{}, "ips must be a JSON array of strings"
	}

	if slices.Contains(ips, "") {
		return linode.ShareNetworkingIPsRequest{}, "ips must not include blank IP addresses"
	}

	return linode.ShareNetworkingIPsRequest{LinodeID: linodeID, IPs: ips}, ""
}

func networkingIPAllocateRequestFromTool(args map[string]any) (linode.AllocateNetworkingIPRequest, string) {
	linodeID, ok := numberArgToInt(args["linode_id"])
	if !ok || linodeID <= 0 {
		return linode.AllocateNetworkingIPRequest{}, "linode_id must be a positive integer"
	}

	ipType, validationMessage := requiredStringArg(args, "type")
	if validationMessage != "" {
		return linode.AllocateNetworkingIPRequest{}, validationMessage
	}

	public, validationMessage := requiredNetworkingBoolArg(args, "public")
	if validationMessage != "" {
		return linode.AllocateNetworkingIPRequest{}, validationMessage
	}

	return linode.AllocateNetworkingIPRequest{LinodeID: linodeID, Public: public, Type: ipType}, ""
}

func requiredNetworkingIPAddressArg(args map[string]any, key string) (string, string) {
	address, validationMessage := requiredStringArg(args, key)
	if validationMessage != "" {
		return "", validationMessage
	}

	addr, err := netip.ParseAddr(address)
	if err != nil || addr.Zone() != "" {
		return "", key + " must be a valid IP address"
	}

	return address, ""
}

func optionalNetworkingBoolArg(args map[string]any, key string) (bool, string) {
	raw, rawFound := args[key]
	if !rawFound {
		return false, ""
	}

	value, valueIsBool := raw.(bool)
	if !valueIsBool {
		return false, key + " must be a boolean"
	}

	return value, ""
}

func requiredNetworkingBoolArg(args map[string]any, key string) (bool, string) {
	raw, rawFound := args[key]
	if !rawFound {
		return false, key + " is required"
	}

	value, valueIsBool := raw.(bool)
	if !valueIsBool {
		return false, key + " must be a boolean"
	}

	return value, ""
}
