package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeDomainListTool creates a tool for listing domains.
func NewLinodeDomainListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolRawSchema(
		cfg,
		"linode_domain_list",
		"Lists all domains managed by your Linode account. Can filter by domain name or type (master/slave).",
		"linode.mcp.v1.DomainListInput",
		func(ctx context.Context, client *linode.Client) ([]*linodev1.Domain, error) {
			return client.ListDomainsProto(ctx)
		},
		[]listFilterParam[*linodev1.Domain]{
			containsFilter("domain_contains", "Filter domains by name containing this string (case-insensitive)",
				func(d *linodev1.Domain) string { return d.GetDomain() }),
			fieldFilter("type", "Filter by domain type (master, slave)",
				func(d *linodev1.Domain) string { return d.GetType() }),
		},
		domainListResponse,
	)

	return tool, profiles.CapRead, handler
}

func domainListResponse(items []*linodev1.Domain, count int32, filter *string) *linodev1.DomainListResponse {
	return &linodev1.DomainListResponse{Count: count, Filter: filter, Domains: items}
}

// NewLinodeDomainGetTool creates a tool for getting a single domain.
func NewLinodeDomainGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_get",
		"Gets detailed information about a specific domain by its ID.",
		toolschemas.Schema("linode.mcp.v1.DomainGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeDomainGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	domain, err := client.GetDomainProto(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain %d: %v", domainID, err)), nil
	}

	return MarshalProtoToolResponse(domain)
}

// NewLinodeDomainZoneFileGetTool creates a tool for getting a domain zone file.
func NewLinodeDomainZoneFileGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_zone_file_get",
		"Gets the rendered zone file for a specific domain by its ID.",
		toolschemas.Schema("linode.mcp.v1.DomainZoneFileGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainZoneFileGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeDomainZoneFileGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)

	if domainID <= 0 {
		return mcp.NewToolResultError("domain_id must be a positive integer"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	zoneFile, err := client.GetDomainZoneFileProto(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve zone file for domain %d: %v", domainID, err)), nil
	}

	return MarshalProtoToolResponse(zoneFile)
}

// NewLinodeDomainRecordGetTool creates a tool for getting a single domain record.
func NewLinodeDomainRecordGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_record_get",
		"Gets detailed information about a specific DNS record within a domain.",
		toolschemas.Schema("linode.mcp.v1.DomainRecordGetInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordGetRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeDomainRecordGetRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	recordID := request.GetInt("record_id", 0)

	if domainID <= 0 {
		return mcp.NewToolResultError("domain_id must be a positive integer"), nil
	}

	if recordID <= 0 {
		return mcp.NewToolResultError("record_id must be a positive integer"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	record, err := client.GetDomainRecordProto(ctx, domainID, recordID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain record %d for domain %d: %v", recordID, domainID, err)), nil
	}

	return MarshalProtoToolResponse(record)
}

// NewLinodeDomainRecordListTool creates a tool for listing domain records.
func NewLinodeDomainRecordListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newProtoListToolSubresourceRawSchema(
		cfg,
		"linode_domain_record_list",
		"Lists all DNS records for a specific domain. Can filter by record type or name.",
		"linode.mcp.v1.DomainRecordListInput",
		protoListPathID{
			option: mcp.WithNumber("domain_id", mcp.Required(),
				mcp.Description("The ID of the domain to list records for")),
			parse: domainRecordListDomainIDFromTool,
		},
		func(ctx context.Context, client *linode.Client, domainID int) ([]*linodev1.DomainRecord, error) {
			return client.ListDomainRecordsProto(ctx, domainID)
		},
		[]listFilterParam[*linodev1.DomainRecord]{
			fieldFilter("type",
				"Filter by record type (A, AAAA, NS, MX, CNAME, TXT, SRV, CAA)",
				func(r *linodev1.DomainRecord) string { return r.GetType() }),
			containsFilter("name_contains",
				"Filter records by name containing this string (case-insensitive)",
				func(r *linodev1.DomainRecord) string { return r.GetName() }),
		},
		domainRecordListResponse,
	)

	return tool, profiles.CapRead, handler
}

// domainRecordListDomainIDFromTool validates the domain_id path param the same
// way the non-proto handler did (a zero id returns "domain_id is required").
func domainRecordListDomainIDFromTool(request *mcp.CallToolRequest) (int, string) {
	domainID := request.GetInt("domain_id", 0)
	if domainID == 0 {
		return 0, "domain_id is required"
	}

	return domainID, ""
}

func domainRecordListResponse(items []*linodev1.DomainRecord, count int32, filter *string) *linodev1.DomainRecordListResponse {
	return &linodev1.DomainRecordListResponse{Count: count, Filter: filter, Records: items}
}
