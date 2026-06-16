package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
)

// NewLinodeDomainListTool creates a tool for listing domains.
func NewLinodeDomainListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newListTool(
		cfg,
		"linode_domain_list",
		"Lists all domains managed by your Linode account. Can filter by domain name or type (master/slave).",
		func(ctx context.Context, client *linode.Client) ([]linode.Domain, error) {
			return client.ListDomains(ctx)
		},
		[]listFilterParam[linode.Domain]{
			containsFilter("domain_contains", "Filter domains by name containing this string (case-insensitive)",
				func(d linode.Domain) string { return d.Domain }),
			fieldFilter("type", "Filter by domain type (master, slave)",
				func(d linode.Domain) string { return d.Type }),
		},
		"domains",
	)

	return tool, profiles.CapRead, handler
}

// NewLinodeDomainGetTool creates a tool for getting a single domain.
func NewLinodeDomainGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_get",
		mcp.WithDescription("Gets detailed information about a specific domain by its ID."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to retrieve"),
		),
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

	domain, err := client.GetDomain(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain %d: %v", domainID, err)), nil
	}

	return MarshalToolResponse(domain)
}

// NewLinodeDomainZoneFileGetTool creates a tool for getting a domain zone file.
func NewLinodeDomainZoneFileGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_zone_file_get",
		mcp.WithDescription("Gets the rendered zone file for a specific domain by its ID."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain whose zone file should be retrieved"),
		),
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

	zoneFile, err := client.GetDomainZoneFile(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve zone file for domain %d: %v", domainID, err)), nil
	}

	return MarshalToolResponse(zoneFile)
}

// NewLinodeDomainRecordGetTool creates a tool for getting a single domain record.
func NewLinodeDomainRecordGetTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_record_get",
		mcp.WithDescription("Gets detailed information about a specific DNS record within a domain."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain that owns the record"),
		),
		mcp.WithNumber(
			"record_id",
			mcp.Required(),
			mcp.Description("The ID of the domain record to retrieve"),
		),
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

	record, err := client.GetDomainRecord(ctx, domainID, recordID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain record %d for domain %d: %v", recordID, domainID, err)), nil
	}

	return MarshalToolResponse(record)
}

// NewLinodeDomainRecordListTool creates a tool for listing domain records.
func NewLinodeDomainRecordListTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_record_list",
		mcp.WithDescription("Lists all DNS records for a specific domain. Can filter by record type or name."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to list records for"),
		),
		mcp.WithString(
			"type",
			mcp.Description("Filter by record type (A, AAAA, NS, MX, CNAME, TXT, SRV, CAA)"),
		),
		mcp.WithString(
			"name_contains",
			mcp.Description("Filter records by name containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordsListRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapRead, handler
}

func handleLinodeDomainRecordsListRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	typeFilter := request.GetString("type", "")
	nameContains := request.GetString("name_contains", "")

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	records, err := client.ListDomainRecords(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain records for domain %d: %v", domainID, err)), nil
	}

	if typeFilter != "" {
		records = FilterByField(records, typeFilter, func(r linode.DomainRecord) string { return r.Type })
	}

	if nameContains != "" {
		records = FilterByContains(records, nameContains, func(r linode.DomainRecord) string { return r.Name })
	}

	return formatDomainRecordsResponse(records, domainID, typeFilter, nameContains)
}

func formatDomainRecordsResponse(records []linode.DomainRecord, domainID int, typeFilter, nameContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count    int                   `json:"count"`
		DomainID int                   `json:"domain_id"`
		Filter   string                `json:"filter,omitempty"`
		Records  []linode.DomainRecord `json:"records"`
	}{
		Count:    len(records),
		DomainID: domainID,
		Records:  records,
	}

	var filters []string
	if typeFilter != "" {
		filters = append(filters, "type="+typeFilter)
	}

	if nameContains != "" {
		filters = append(filters, "name_contains="+nameContains)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return MarshalToolResponse(response)
}
