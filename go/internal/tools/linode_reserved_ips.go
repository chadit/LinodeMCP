package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"net/url"
	"regexp"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

const (
	linodeReservedIPListToolName     = "linode_networking_reserved_ip_list"
	linodeReservedIPDeleteToolName   = "linode_networking_reserved_ip_delete"
	linodeReservedIPGetToolName      = "linode_networking_reserved_ip_get"
	linodeReservedIPCreateToolName   = "linode_networking_reserved_ip_create"
	linodeReservedIPUpdateToolName   = "linode_networking_reserved_ip_update"
	linodeReservedIPTypeListToolName = "linode_networking_reserved_ip_type_list"
)

// NewLinodeReservedIPGetTool creates a tool for reading one reserved public
// IPv4 address.
func NewLinodeReservedIPGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPGetToolName,
		"Gets a reserved public IPv4 address",
		toolschemas.Schema("linode.mcp.v1.ReservedIPGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		address, validationMessage := reservedIPv4AddressFromTool(request.GetArguments())
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		raw, err := client.GetReservedIPRaw(ctx, address)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get reserved IPv4 address: %v", err)), nil
		}

		return marshalReservedIPResponse(raw)
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeReservedIPTypeListTool creates a tool for listing reserved IPv4
// pricing types.
func NewLinodeReservedIPTypeListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPTypeListToolName,
		"Lists reserved public IPv4 pricing information",
		toolschemas.Schema("linode.mcp.v1.ReservedIPTypeListInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		types, err := client.ListReservedIPTypesProto(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		var count int32
		if n := len(types); n <= math.MaxInt32 {
			count = int32(n)
		}

		return MarshalProtoToolResponse(&linodev1.ReservedIPTypeListResponse{
			Count:           count,
			ReservedIpTypes: types,
		})
	}

	return tool, profiles.CapRead, handler
}

// NewLinodeReservedIPCreateTool creates a tool for reserving a public IPv4
// address in a region.
func NewLinodeReservedIPCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPCreateToolName,
		"Reserves a public IPv4 address in a region. Pass dry_run=true to preview without reserving or starting billing.",
		toolschemas.Schema("linode.mcp.v1.ReservedIPCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleReservedIPCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleReservedIPCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	region, validationMessage := reservedIPRegionFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	tags, supplied, validationMessage := reservedIPTagsFromTool(request.GetArguments(), false)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	body := map[string]any{"region": region}
	if supplied {
		body["tags"] = tags
	}

	if IsDryRun(request) {
		// No resource exists yet, so there is no state to fetch and the preview
		// describes the reservation from the request alone.
		return RunDryRunPreviewWithBodyDetailed(
			ctx, request, cfg,
			linodeReservedIPCreateToolName, httpMethodPost, endpointReservedIPToolPath, body, nil,
			func(context.Context, *linode.Client, any) (DryRunDetails, error) {
				return DryRunDetails{
					BillingDelta: &DryRunBillingDelta{
						// Reserved IPv4 pricing varies by region, and the
						// preview has no region price to read here.
						MonthlyChangeUSD: reservedIPBillingUnknown,
						Note:             "Reserved IPv4 pricing varies by region; see linode_networking_reserved_ip_type_list.",
					},
					Warnings: []string{"Reserved IP billing begins when the address is created."},
				}, nil
			},
		)
	}

	if denial := RequireConfirm(request, "This reserves a public IPv4 address and starts billing. Set confirm=true to proceed."); denial != nil {
		return denial, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	raw, err := client.CreateReservedIPRaw(ctx, region, tags)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to reserve public IPv4 address: %v", err)), nil
	}

	return marshalReservedIPResponse(raw)
}

// NewLinodeReservedIPUpdateTool creates a tool for replacing the tags on a
// reserved public IPv4 address.
func NewLinodeReservedIPUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPUpdateToolName,
		"Replaces all tags on a reserved public IPv4 address. Pass dry_run=true to preview without updating.",
		toolschemas.Schema("linode.mcp.v1.ReservedIPUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleReservedIPUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleReservedIPUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	address, validationMessage := reservedIPv4AddressFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	tags, _, validationMessage := reservedIPTagsFromTool(request.GetArguments(), true)
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	path := endpointReservedIPToolPath + "/" + address

	if IsDryRun(request) {
		return RunDryRunPreviewWithBody(
			ctx, request, cfg,
			linodeReservedIPUpdateToolName, httpMethodPut, path, map[string]any{"tags": tags},
			func(ctx context.Context, client *linode.Client) (any, error) {
				return client.GetReservedIPRaw(ctx, address)
			},
		)
	}

	if denial := RequireConfirm(request, "This replaces all tags on the reserved IP. Set confirm=true to proceed."); denial != nil {
		return denial, nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	raw, err := client.UpdateReservedIPRaw(ctx, address, tags)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to replace reserved IPv4 tags: %v", err)), nil
	}

	return marshalReservedIPResponse(raw)
}

// reservedIPRegionFromTool validates the documented lowercase region slug.
func reservedIPRegionFromTool(args map[string]any) (string, string) {
	region, ok := args["region"].(string)
	if !ok || region == "" {
		return "", "region is required"
	}

	if !reservedIPRegionPattern.MatchString(region) {
		return "", "region must be a lowercase region slug"
	}

	return region, ""
}

// reservedIPTagsFromTool validates the tag list. The second result reports
// whether the caller supplied one at all, which the create path needs: an
// absent list leaves the reservation untagged, while an empty list is a
// request to send no tags.
func reservedIPTagsFromTool(args map[string]any, required bool) ([]string, bool, string) {
	raw, present := args["tags"]
	if !present || raw == nil {
		if required {
			return nil, false, "tags is required"
		}

		return nil, false, ""
	}

	items, ok := raw.([]any)
	if !ok {
		return nil, false, "tags must be a list of strings"
	}

	tags := make([]string, 0, len(items))

	for _, item := range items {
		tag, isString := item.(string)
		if !isString || tag == "" {
			return nil, false, "tags must contain only non-empty strings"
		}

		tags = append(tags, tag)
	}

	return tags, true, ""
}

// marshalReservedIPResponse renders one reserved address the way the list path
// does: the proto fixes the field shape, then the raw object restores the
// documented explicit nulls protojson drops.
func marshalReservedIPResponse(raw json.RawMessage) (*mcp.CallToolResult, error) {
	return marshalReservedIPResponseWithMarshal(raw, MarshalProtoJSON)
}

// marshalReservedIPResponseWithMarshal is marshalReservedIPResponse with the
// proto marshaller injected, matching the seam the list path uses so its
// defensive branches stay reachable from a test.
func marshalReservedIPResponseWithMarshal(
	raw json.RawMessage,
	marshal func(proto.Message) ([]byte, error),
) (*mcp.CallToolResult, error) {
	reservedIP := &linodev1.ReservedIPAddress{}
	if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(raw, reservedIP); err != nil {
		return nil, fmt.Errorf("failed to decode reserved IP API response: %w", err)
	}

	item, err := reservedIPAddressResponseWithMarshal(reservedIP, raw, marshal)
	if err != nil {
		return nil, err
	}

	return marshalReservedIPJSON(&item)
}

// marshalReservedIPJSON renders one address, the single-item sibling of
// marshalReservedIPListJSON.
func marshalReservedIPJSON(item *reservedIPAddressJSON) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reserved IP response: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

// NewLinodeReservedIPDeleteTool creates a tool for permanently unreserving a
// public IPv4 address.
func NewLinodeReservedIPDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPDeleteToolName,
		"Permanently unreserves a public IPv4 address and stops billing. Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.ReservedIPDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleReservedIPDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

func handleReservedIPDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	address, validationMessage := reservedIPv4AddressFromTool(request.GetArguments())
	if validationMessage != "" {
		return mcp.NewToolResultError(validationMessage), nil
	}

	return RunDestructiveAction(ctx, request, cfg, &DestructiveAction{
		ToolName:       linodeReservedIPDeleteToolName,
		Method:         httpMethodDelete,
		Path:           endpointReservedIPToolPath + "/" + url.PathEscape(address),
		ConfirmMessage: "This permanently unreserves the IP and it cannot be recovered. Set confirm=true to proceed.",
		FetchState: func(ctx context.Context, c *linode.Client) (any, error) {
			return c.GetReservedIPRaw(ctx, address)
		},
		Execute: func(ctx context.Context, c *linode.Client) error {
			return c.DeleteReservedIP(ctx, address)
		},
		Success: func() proto.Message {
			return &linodev1.ReservedIPDeleteResponse{
				Message: fmt.Sprintf("Reserved IP %s unreserved successfully", address),
				Address: address,
			}
		},
		HashIgnore: twostage.HashIgnoreFields("ReservedIP"),
	})
}

const (
	endpointReservedIPToolPath = "/networking/reserved/ips"
	// The dry-run contract's stand-in when a monthly change cannot be
	// estimated, which reserved IPv4 pricing cannot be without a region price.
	reservedIPBillingUnknown = "unknown"
)

// reservedIPRegionPattern is the documented lowercase region slug shape, kept
// identical to the Python twin so both languages reject the same input.
var reservedIPRegionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func reservedIPv4AddressFromTool(args map[string]any) (string, string) {
	address, ok := args["address"].(string)
	if !ok || address == "" {
		return "", "address is required"
	}

	addr, err := netip.ParseAddr(address)
	if err != nil || !addr.Is4() {
		return "", "address must be a valid IPv4 address"
	}

	return address, ""
}

// NewLinodeReservedIPListTool creates a tool for listing reserved public IPv4
// addresses on the account.
func NewLinodeReservedIPListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		linodeReservedIPListToolName,
		"Lists reserved public IPv4 addresses on the account.",
		toolschemas.Schema("linode.mcp.v1.ReservedIPListInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, pageSize, validationMessage := reservedIPListPagination(request.GetArguments())
		if validationMessage != "" {
			return mcp.NewToolResultError(validationMessage), nil
		}

		client, err := prepareClient(&request, cfg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		reservedIPs, err := client.ListReservedIPsProto(ctx, page, pageSize)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve items: %v", err)), nil
		}

		return marshalReservedIPListResponse(reservedIPs)
	}

	return tool, profiles.CapRead, handler
}

func reservedIPListPagination(args map[string]any) (int, int, string) {
	page, validationMessage := optionalPaginationInt(args, "page", 1, 0)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", standardPageSizeMin, standardPageSizeMax)
	if validationMessage != "" {
		return 0, 0, validationMessage
	}

	return page, pageSize, ""
}

type reservedIPListJSON struct {
	Count       int32                   `json:"count"`
	ReservedIPs []reservedIPAddressJSON `json:"reserved_ips"`
}

type reservedIPAddressJSON struct {
	Address        json.RawMessage `json:"address"`
	AssignedEntity json.RawMessage `json:"assigned_entity,omitempty"`
	Gateway        json.RawMessage `json:"gateway,omitempty"`
	InterfaceID    json.RawMessage `json:"interface_id,omitempty"`
	LinodeID       json.RawMessage `json:"linode_id,omitempty"`
	Prefix         json.RawMessage `json:"prefix"`
	Public         json.RawMessage `json:"public"`
	RDNS           json.RawMessage `json:"rdns,omitempty"`
	Region         json.RawMessage `json:"region"`
	Reserved       json.RawMessage `json:"reserved"`
	SubnetMask     json.RawMessage `json:"subnet_mask"`
	Tags           json.RawMessage `json:"tags"`
	Type           json.RawMessage `json:"type"`
	VPCNAT11       json.RawMessage `json:"vpc_nat_1_1,omitempty"`
}

func marshalReservedIPListResponse(page *linode.ReservedIPListPage) (*mcp.CallToolResult, error) {
	if len(page.ReservedIPs) != len(page.RawReservedIPs) {
		return nil, fmt.Errorf("%w: %d typed items and %d raw items", errReservedIPListShape, len(page.ReservedIPs), len(page.RawReservedIPs))
	}

	items := make([]reservedIPAddressJSON, 0, len(page.ReservedIPs))
	for index, reservedIP := range page.ReservedIPs {
		item, err := reservedIPAddressResponse(reservedIP, page.RawReservedIPs[index])
		if err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	var count int32
	if n := len(items); n <= math.MaxInt32 {
		count = int32(n)
	}

	return marshalReservedIPListJSON(reservedIPListJSON{Count: count, ReservedIPs: items})
}

func marshalReservedIPListJSON(response reservedIPListJSON) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reserved IP list response: %w", err)
	}

	return mcp.NewToolResultText(string(data)), nil
}

func reservedIPAddressResponse(reservedIP *linodev1.ReservedIPAddress, raw json.RawMessage) (reservedIPAddressJSON, error) {
	return reservedIPAddressResponseWithMarshal(reservedIP, raw, MarshalProtoJSON)
}

func reservedIPAddressResponseWithMarshal(reservedIP *linodev1.ReservedIPAddress, raw json.RawMessage, marshal func(proto.Message) ([]byte, error)) (reservedIPAddressJSON, error) {
	data, err := marshal(reservedIP)
	if err != nil {
		return reservedIPAddressJSON{}, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return reservedIPAddressJSON{}, fmt.Errorf("failed to decode reserved IP proto response: %w", err)
	}

	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &rawFields); err != nil {
		return reservedIPAddressJSON{}, fmt.Errorf("failed to decode reserved IP API response: %w", err)
	}

	for _, name := range []string{"assigned_entity", "gateway", "interface_id", "linode_id", "rdns", "vpc_nat_1_1"} {
		if value, ok := rawFields[name]; ok && string(value) == "null" {
			fields[name] = json.RawMessage("null")
		}
	}

	return reservedIPAddressJSON{
		Address: fields["address"], AssignedEntity: fields["assigned_entity"], Gateway: fields["gateway"],
		InterfaceID: fields["interface_id"], LinodeID: fields["linode_id"], Prefix: fields["prefix"],
		Public: fields["public"], RDNS: fields["rdns"], Region: fields["region"], Reserved: fields["reserved"],
		SubnetMask: fields["subnet_mask"], Tags: fields["tags"], Type: fields["type"], VPCNAT11: fields["vpc_nat_1_1"],
	}, nil
}
