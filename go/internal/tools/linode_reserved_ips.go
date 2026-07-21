package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/netip"
	"net/url"

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
	linodeReservedIPListToolName   = "linode_networking_reserved_ip_list"
	linodeReservedIPDeleteToolName = "linode_networking_reserved_ip_delete"
	reservedIPListPageSizeMin      = 25
	reservedIPListPageSizeMax      = 500
)

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

const endpointReservedIPToolPath = "/networking/reserved/ips"

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

	pageSize, validationMessage := optionalPaginationInt(args, "page_size", reservedIPListPageSizeMin, reservedIPListPageSizeMax)
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
