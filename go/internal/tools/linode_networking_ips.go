package tools

import (
	"context"
	"fmt"
	"math"
	"net/netip"
	"slices"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

const (
	paramSkipIPv6RDNS = "skip_ipv6_rdns"
	paramAddress      = "address"
	paramRDNS         = "rdns"
	paramIPs          = "ips"
)

// NewLinodeNetworkingIPListTool creates a tool for listing account IP addresses.
func NewLinodeNetworkingIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_list",
		"Lists IP addresses on the account. Set skip_ipv6_rdns to true to skip IPv6 reverse DNS lookups.",
		toolschemas.Schema("linode.mcp.v1.NetworkingIPListInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPListRequest(ctx, &request, cfg)
	}

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

	ips, err := client.ListNetworkingIPsProto(ctx, skipIPv6RDNS)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
	}

	return finishProtoList(request, ips, nil, networkingIPListResponse)
}

func networkingIPListResponse(items []*linodev1.IPAddress, count int32, filter *string) *linodev1.NetworkingIPListResponse {
	return &linodev1.NetworkingIPListResponse{Count: count, Filter: filter, Ips: items}
}

// NewLinodeNetworkingIPGetTool creates a tool for retrieving one account-level IP address.
func NewLinodeNetworkingIPGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_get",
		"Gets details for one account-level IP address.",
		toolschemas.Schema("linode.mcp.v1.IPAddressGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPGetRequest(ctx, &request, cfg)
	}

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

	ipAddr, err := client.GetNetworkingIPProto(ctx, address)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve networking IP %s: %v", address, err)), nil
	}

	return MarshalProtoToolResponse(ipAddr)
}

// NewLinodeNetworkingIPUpdateRDNSTool creates a tool for updating account-level IP reverse DNS.
func NewLinodeNetworkingIPUpdateRDNSTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_update",
		"Updates reverse DNS for one account-level IP address.",
		toolschemas.Schema("linode.mcp.v1.IPAddressUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPUpdateRDNSRequest(ctx, &request, cfg)
	}

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

	return MarshalProtoToolResponse(&linodev1.IPAddressWriteResponse{
		Message: fmt.Sprintf("Networking IP %s RDNS updated", address),
		Ip:      ipAddr,
	})
}

func updateNetworkingIPRDNS(ctx context.Context, client *linode.Client, address, rdns string) (*linodev1.IPAddress, string) {
	ipAddr, err := client.UpdateNetworkingIPProto(ctx, address, linode.UpdateNetworkingIPRequest{RDNS: rdns})
	if err != nil {
		return nil, "Failed to update networking IP " + address + " RDNS: " + err.Error()
	}

	return ipAddr, ""
}

// NewLinodeNetworkingIPAllocateTool creates a tool for allocating an account-level IP address.
func NewLinodeNetworkingIPAllocateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_allocate",
		"Allocates an account-level IP address. WARNING: Additional IPs may incur charges.",
		toolschemas.Schema("linode.mcp.v1.IPAddressAllocateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPAllocateRequest(ctx, &request, cfg)
	}

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

	ipAddr, err := client.AllocateNetworkingIPProto(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to allocate networking IP: %v", err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.IPAddressWriteResponse{
		Message: fmt.Sprintf("IP %s allocated for Linode %d", ipAddr.GetAddress(), req.LinodeID),
		Ip:      ipAddr,
	})
}

// NewLinodeNetworkingIPAssignTool creates a tool for assigning IP addresses to Linodes.
func NewLinodeNetworkingIPAssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_assign",
		"Assigns IP addresses to Linodes in a region. WARNING: This changes IP ownership assignments.",
		toolschemas.Schema("linode.mcp.v1.NetworkingIPAssignInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPAssignRequest(ctx, &request, cfg)
	}

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
// that captures the parsed request. The assign and share endpoints return an
// opaque body, so the execute closure performs the action and the result builder
// produces the id-echo proto response from the already-parsed request.
// validationMessage is checked in both the dry-run and real paths so a malformed
// call fails the same way either way.
func runNetworkingIPWrite(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
	spec *networkingIPWriteSpec,
	validationMessage string,
	execute func(ctx context.Context, client *linode.Client) (map[string]any, error),
	buildResponse func() proto.Message,
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

	// The assign and share endpoints return an opaque body; the helper performs
	// the action and discards it, then builds the id-echo response from the
	// already-parsed request.
	if _, err := execute(ctx, client); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("%s: %v", spec.FailureLabel, err)), nil
	}

	return MarshalProtoToolResponse(buildResponse())
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
	}, func() proto.Message {
		return networkingIPAssignResponse("Networking IP assignments updated", req)
	})
}

// NewLinodeNetworkingIPv4AssignTool creates a tool for assigning IPv4 addresses to Linodes.
func NewLinodeNetworkingIPv4AssignTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ipv4_assign",
		"Assigns IPv4 addresses to Linodes in a region. WARNING: This changes IP ownership assignments.",
		toolschemas.Schema("linode.mcp.v1.NetworkingIPv4AssignInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPv4AssignRequest(ctx, &request, cfg)
	}

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
	}, func() proto.Message {
		return networkingIPAssignResponse("Networking IPv4 assignments updated", req)
	})
}

// NewLinodeNetworkingIPv4ShareTool creates a tool for sharing IP addresses with a primary Linode.
func NewLinodeNetworkingIPv4ShareTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ipv4_share",
		"Shares IP addresses with a primary Linode. Set ips to a JSON string array; an empty array removes all shared IP addresses.",
		toolschemas.Schema("linode.mcp.v1.NetworkingIPv4ShareInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPv4ShareRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPv4ShareRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := networkingIPShareRequestFromTool(request.GetArguments())

	return runNetworkingIPWrite(ctx, request, cfg, &networkingIPWriteSpec{
		ToolName:       "linode_networking_ipv4_share",
		Path:           "/networking/ipv4/share",
		ConfirmMessage: "This changes shared IP assignments. Set confirm=true to proceed.",
		SuccessMessage: "Networking IP sharing updated",
		FailureLabel:   "Failed to share networking IPs",
	}, validationMessage, func(ctx context.Context, client *linode.Client) (map[string]any, error) {
		return client.ShareNetworkingIPv4s(ctx, req)
	}, func() proto.Message {
		return networkingIPShareResponse("Networking IP sharing updated", req)
	})
}

// NewLinodeNetworkingIPShareTool creates a tool for sharing IP addresses with a
// primary Linode via the generic /networking/ips/share endpoint (the IPv4-only
// variant above uses /networking/ipv4/share).
func NewLinodeNetworkingIPShareTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_networking_ip_share",
		"Shares IP addresses with a primary Linode. Set ips to a JSON string array; an empty array removes all shared IP addresses.",
		toolschemas.Schema("linode.mcp.v1.NetworkingIPShareInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeNetworkingIPShareRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeNetworkingIPShareRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	req, validationMessage := networkingIPShareRequestFromTool(request.GetArguments())

	return runNetworkingIPWrite(ctx, request, cfg, &networkingIPWriteSpec{
		ToolName:       "linode_networking_ip_share",
		Path:           "/networking/ips/share",
		ConfirmMessage: "This changes shared IP assignments. Set confirm=true to proceed.",
		SuccessMessage: "Networking IP sharing updated",
		FailureLabel:   "Failed to share networking IPs",
	}, validationMessage, func(ctx context.Context, client *linode.Client) (map[string]any, error) {
		return client.ShareNetworkingIPs(ctx, req)
	}, func() proto.Message {
		return networkingIPShareResponse("Networking IP sharing updated", req)
	})
}

// linodeIDToInt32 narrows a Linode ID to the proto int32 field, returning 0 for
// the rare out-of-range value so the bounded conversion never overflows.
func linodeIDToInt32(id int) int32 {
	if id < math.MinInt32 || id > math.MaxInt32 {
		return 0
	}

	return int32(id)
}

// intSliceToInt32 narrows a slice of Linode IDs to the proto repeated int32
// field, bounding each element the same way linodeIDToInt32 does.
func intSliceToInt32(ids []int) []int32 {
	out := make([]int32, len(ids))
	for i, id := range ids {
		out[i] = linodeIDToInt32(id)
	}

	return out
}

// networkingIPAssignResponse builds the id-echo proto for the IP assign tools
// from the parsed request. The assign endpoint returns an opaque body, so the
// response echoes the region and the assignment list the caller submitted.
func networkingIPAssignResponse(message string, req linode.AssignNetworkingIPsRequest) *linodev1.NetworkingIPAssignWriteResponse {
	assignments := make([]*linodev1.IPAssignment, 0, len(req.Assignments))
	for _, assignment := range req.Assignments {
		assignments = append(assignments, &linodev1.IPAssignment{
			Address:  assignment.Address,
			LinodeId: linodeIDToInt32(assignment.LinodeID),
		})
	}

	return &linodev1.NetworkingIPAssignWriteResponse{
		Message:     message,
		Region:      req.Region,
		Assignments: assignments,
	}
}

// networkingIPShareResponse builds the id-echo proto for the IP share tools from
// the parsed request. The share endpoint returns an opaque body, so the response
// echoes the primary Linode and the shared address list the caller submitted.
func networkingIPShareResponse(message string, req linode.ShareNetworkingIPsRequest) *linodev1.NetworkingIPShareWriteResponse {
	return &linodev1.NetworkingIPShareWriteResponse{
		Message:  message,
		LinodeId: linodeIDToInt32(req.LinodeID),
		Ips:      req.IPs,
	}
}

func networkingIPAssignRequestFromTool(args map[string]any) (linode.AssignNetworkingIPsRequest, string) {
	region, validationMessage := requiredStringArg(args, "region")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
	}

	rawAssignments, present := args["assignments"]
	if !present {
		return linode.AssignNetworkingIPsRequest{}, "assignments is required"
	}

	assignments, validationMessage := objectSliceFromToolArg[linode.IPAssignment](rawAssignments, "assignments")
	if validationMessage != "" {
		return linode.AssignNetworkingIPsRequest{}, validationMessage
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

	ipsArg, ipsPresent := args[paramIPs]
	if !ipsPresent {
		return linode.ShareNetworkingIPsRequest{}, paramIPs + " is required"
	}

	ips, validationMessage := stringSliceFromToolArg(ipsArg, paramIPs)
	if validationMessage != "" {
		return linode.ShareNetworkingIPsRequest{}, validationMessage
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

	if msg := enumChoiceError(ipType, "type", linodev1.InstanceIPType_Value_value); msg != "" {
		return linode.AllocateNetworkingIPRequest{}, msg
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
