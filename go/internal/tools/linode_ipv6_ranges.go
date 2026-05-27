package tools

import (
	"context"
	"net/netip"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

const (
	keyIPv6RangePrefixLength = "prefix_length"
	keyIPv6RangeLinodeID     = "linode_id"
	keyIPv6RangeRouteTarget  = "route_target"
)

// NewLinodeIPv6RangesListTool creates a tool for listing IPv6 ranges.
func NewLinodeIPv6RangesListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newLinodeIPv6ListTool(
		cfg,
		"linode_ipv6_ranges_list",
		"Lists IPv6 ranges on the account with optional pagination.",
		func(ctx context.Context, client *linode.Client, page, pageSize int) (*linode.PaginatedResponse[linode.IPv6Range], string) {
			items, err := client.ListIPv6Ranges(ctx, page, pageSize)
			if err != nil {
				return nil, err.Error()
			}

			return items, ""
		},
	)
}

// NewLinodeIPv6RangeCreateTool creates a tool for creating an IPv6 range.
func NewLinodeIPv6RangeCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_ipv6_range_create",
		"Creates an IPv6 range. WARNING: This changes networking configuration and may affect routing.",
		[]mcp.ToolOption{
			mcp.WithNumber(keyIPv6RangePrefixLength, mcp.Required(),
				mcp.Description("The IPv6 prefix length for the created range.")),
			mcp.WithNumber(keyIPv6RangeLinodeID,
				mcp.Description("Optional Linode ID to assign the new range to.")),
			mcp.WithString(keyIPv6RangeRouteTarget,
				mcp.Description("Optional route target for the new IPv6 range.")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be true to confirm IPv6 range creation.")),
		},
		handleLinodeIPv6RangeCreateRequest,
	)

	return tool, profiles.CapWrite, handler
}

func handleLinodeIPv6RangeCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	response := struct {
		Message string            `json:"message"`
		Range   *linode.IPv6Range `json:"range"`
	}{
		Message: "IPv6 range created",
		Range:   ipv6Range,
	}

	return MarshalToolResponse(response)
}

func createIPv6Range(ctx context.Context, client *linode.Client, req linode.CreateIPv6RangeRequest) (*linode.IPv6Range, string) {
	ipv6Range, err := client.CreateIPv6Range(ctx, req)
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

	return req, ""
}
