package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeDomainsListTool creates a tool for listing domains.
func NewLinodeDomainsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	return newListTool(cfg,
		"linode_domains_list",
		"Lists all domains managed by your Linode account. Can filter by domain name or type (master/slave).",
		func(ctx context.Context, client *linode.RetryableClient) ([]linode.Domain, error) {
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
}

// NewLinodeDomainGetTool creates a tool for getting a single domain.
func NewLinodeDomainGetTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_get",
		mcp.WithDescription("Gets detailed information about a specific domain by its ID."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to retrieve"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainGetRequest(ctx, &request, cfg)
	}

	return tool, handler
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

	return marshalToolResponse(domain)
}

// NewLinodeDomainRecordsListTool creates a tool for listing domain records.
func NewLinodeDomainRecordsListTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_records_list",
		mcp.WithDescription("Lists all DNS records for a specific domain. Can filter by record type or name."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to list records for"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by record type (A, AAAA, NS, MX, CNAME, TXT, SRV, CAA)"),
		),
		mcp.WithString("name_contains",
			mcp.Description("Filter records by name containing this string (case-insensitive)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordsListRequest(ctx, &request, cfg)
	}

	return tool, handler
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
		records = filterByField(records, typeFilter, func(r linode.DomainRecord) string { return r.Type })
	}

	if nameContains != "" {
		records = filterByContains(records, nameContains, func(r linode.DomainRecord) string { return r.Name })
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

	return marshalToolResponse(response)
}
