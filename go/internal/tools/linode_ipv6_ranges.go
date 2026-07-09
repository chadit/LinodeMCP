package tools

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	paramIPv6Range           = "range"
	keyIPv6RangePrefixLength = "prefix_length"
	keyIPv6RangeLinodeID     = "linode_id"
	keyIPv6RangeRouteTarget  = "route_target"
)

// NewLinodeIPv6RangesListTool creates a tool for listing IPv6 ranges.
func NewLinodeIPv6RangesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolPaginatedRawSchema(
		cfg,
		"linode_ipv6_range_list",
		"Lists IPv6 ranges on the account with optional pagination.",
		"linode.mcp.v1.IPv6RangeListInput",
		"Page of results to return (optional, minimum 1).",
		"Number of results per page (optional, 25-500).",
		func(ctx context.Context, client *linode.Client, page, pageSize int) ([]*linodev1.IPv6Range, error) {
			return client.ListIPv6RangesProto(ctx, page, pageSize)
		},
		ipv6ListPaginationFromTool,
		nil,
		ipv6RangeListResponse,
	)

	return tool, profiles.CapRead, handler
}

func ipv6RangeListResponse(items []*linodev1.IPv6Range, count int32, filter *string) *linodev1.IPv6RangeListResponse {
	return &linodev1.IPv6RangeListResponse{Count: count, Filter: filter, Ipv6Ranges: items}
}

// NewLinodeIPv6RangeGetTool creates a tool for retrieving one IPv6 range.
func NewLinodeIPv6RangeGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_ipv6_range_get",
		"Gets one IPv6 range by CIDR prefix.",
		toolschemas.Schema("linode.mcp.v1.IPv6RangeGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleIPv6RangeGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleIPv6RangeGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ipv6Range, validationMessage := ipv6RangeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	rangeResult, err := client.GetIPv6RangeProto(ctx, ipv6Range)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve IPv6 range: %v", err)), nil
	}

	return MarshalProtoToolResponse(rangeResult)
}

func ipv6RangeFromTool(request *mcp.CallToolRequest) (string, string) {
	ipv6Range, validationMessage := requiredStringArg(request.GetArguments(), paramIPv6Range)
	if validationMessage != "" {
		return "", validationMessage
	}

	// Trim so a whitespace-padded range parses and is sent trimmed, matching
	// Python's range_value.strip() (align the trim-vs-raw asymmetry).
	ipv6Range = strings.TrimSpace(ipv6Range)

	prefix, err := netip.ParsePrefix(ipv6Range)
	if err != nil || !prefix.Addr().Is6() || prefix != prefix.Masked() {
		return "", "range must be a valid IPv6 prefix"
	}

	return ipv6Range, ""
}

// NewLinodeIPv6RangeCreateTool creates a tool for creating an IPv6 range.
func NewLinodeIPv6RangeCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_ipv6_range_create",
		"Creates an IPv6 range. WARNING: This changes networking configuration and may affect routing.",
		toolschemas.Schema("linode.mcp.v1.IPv6RangeCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeIPv6RangeCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeIPv6RangeCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	if IsDryRun(request) {
		if _, validationMessage := ipv6RangeCreateRequestFromTool(request.GetArguments()); validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_ipv6_range_create", httpMethodPost, "/networking/ipv6/ranges", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return ipv6RangeCreateSideEffects(ctx,
					request.GetInt(keyIPv6RangePrefixLength, 0),
					request.GetInt(keyIPv6RangeLinodeID, 0),
					request.GetString(keyIPv6RangeRouteTarget, ""))
			})
	}

	if result := RequireConfirm(request, "This creates an IPv6 range and changes networking configuration. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	req, validationMessage := ipv6RangeCreateRequestFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	ipv6Range, failureMessage := createIPv6Range(ctx, client, req)
	if failureMessage != "" {
		return mcp.NewToolResultError(failureMessage), nil
	}

	response := &linodev1.IPv6RangeWriteResponse{
		Message: "IPv6 range created",
		Range:   ipv6Range,
	}

	return MarshalProtoToolResponse(response)
}

func createIPv6Range(ctx context.Context, client *linode.Client, req linode.CreateIPv6RangeRequest) (*linodev1.IPv6Range, string) {
	ipv6Range, err := client.CreateIPv6RangeProto(ctx, req)
	if err != nil {
		return nil, "Failed to create IPv6 range: " + err.Error()
	}

	return ipv6Range, ""
}

func ipv6RangeCreateRequestFromTool(args map[string]any) (linode.CreateIPv6RangeRequest, string) {
	prefixLength, ok := numberArgToInt(args[keyIPv6RangePrefixLength])
	if !ok || prefixLength < 1 || prefixLength > 128 {
		return linode.CreateIPv6RangeRequest{}, linode.ErrIPv6RangePrefixRange.Error()
	}

	req := linode.CreateIPv6RangeRequest{PrefixLength: prefixLength}

	if rawLinodeID, hasLinodeID := args[keyIPv6RangeLinodeID]; hasLinodeID {
		linodeID, linodeIDOK := numberArgToInt(rawLinodeID)
		if !linodeIDOK || linodeID <= 0 {
			return linode.CreateIPv6RangeRequest{}, linode.ErrLinodeIDPositive.Error()
		}

		req.LinodeID = &linodeID
	}

	routeTarget, hasRouteTarget, validationMessage := optionalStringField(args, keyIPv6RangeRouteTarget)
	if validationMessage != "" {
		return linode.CreateIPv6RangeRequest{}, validationMessage
	}

	if hasRouteTarget {
		parsedRouteTarget, err := netip.ParseAddr(routeTarget)
		if err != nil || !parsedRouteTarget.Is6() || parsedRouteTarget.Zone() != "" {
			return linode.CreateIPv6RangeRequest{}, linode.ErrIPv6RangeRouteTargetInvalid.Error()
		}

		req.RouteTarget = routeTarget
	}

	// Require exactly one assignment target, mirroring Python's
	// _parse_ipv6_range_target (Go previously accepted neither or both).
	if req.LinodeID == nil && req.RouteTarget == "" {
		return linode.CreateIPv6RangeRequest{}, "linode_id or route_target is required"
	}

	if req.LinodeID != nil && req.RouteTarget != "" {
		return linode.CreateIPv6RangeRequest{}, "linode_id and route_target are mutually exclusive"
	}

	return req, ""
}

// NewLinodeIPv6RangeDeleteTool creates a tool for deleting an IPv6 range.
func NewLinodeIPv6RangeDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_ipv6_range_delete",
		"Deletes an IPv6 range. WARNING: This changes networking configuration and may affect routing."+
			" Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.IPv6RangeDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleIPv6RangeDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleIPv6RangeDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	ipv6Range, validationMessage := ipv6RangeFromTool(request)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       "linode_ipv6_range_delete",
		Method:         httpMethodDelete,
		Path:           "/networking/ipv6/ranges/" + ipv6Range,
		ConfirmMessage: "This deletes an IPv6 range and changes networking configuration. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetIPv6Range(ctx, ipv6Range)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteIPv6Range(ctx, ipv6Range)
		},
		Success: func() proto.Message {
			return &linodev1.IPv6RangeDeleteResponse{
				Message: "IPv6 range deleted",
				Range:   ipv6Range,
			}
		},
		// An IPv6 range carries no cosmetic timestamp, so the whole state is
		// hashed; the unknown "IPv6Range" key returns nil.
		HashIgnore: twostage.HashIgnoreFields("IPv6Range"),
	})
}
