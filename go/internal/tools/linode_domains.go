//nolint:dupl // Tool implementations have similar structure by design
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
	tool := mcp.NewTool("linode_domains_list",
		mcp.WithDescription("Lists all domains managed by your Linode account. Can filter by domain name or type (master/slave)."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("domain_contains",
			mcp.Description("Filter domains by name containing this string (case-insensitive)"),
		),
		mcp.WithString("type",
			mcp.Description("Filter by domain type (master, slave)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domainContains := request.GetString("domain_contains", "")
	typeFilter := request.GetString("type", "")

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	domains, err := client.ListDomains(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domains: %v", err)), nil
	}

	if domainContains != "" {
		domains = filterDomainsByName(domains, domainContains)
	}

	if typeFilter != "" {
		domains = filterDomainsByType(domains, typeFilter)
	}

	return formatDomainsResponse(domains, domainContains, typeFilter)
}

func filterDomainsByName(domains []linode.Domain, domainContains string) []linode.Domain {
	filtered := make([]linode.Domain, 0, len(domains))

	domainContains = strings.ToLower(domainContains)

	for _, domain := range domains {
		if strings.Contains(strings.ToLower(domain.Domain), domainContains) {
			filtered = append(filtered, domain)
		}
	}

	return filtered
}

func filterDomainsByType(domains []linode.Domain, typeFilter string) []linode.Domain {
	filtered := make([]linode.Domain, 0, len(domains))

	typeFilter = strings.ToLower(typeFilter)

	for _, domain := range domains {
		if strings.ToLower(domain.Type) == typeFilter {
			filtered = append(filtered, domain)
		}
	}

	return filtered
}

func formatDomainsResponse(domains []linode.Domain, domainContains, typeFilter string) (*mcp.CallToolResult, error) {
	response := struct {
		Count   int             `json:"count"`
		Filter  string          `json:"filter,omitempty"`
		Domains []linode.Domain `json:"domains"`
	}{
		Count:   len(domains),
		Domains: domains,
	}

	var filters []string
	if domainContains != "" {
		filters = append(filters, "domain_contains="+domainContains)
	}

	if typeFilter != "" {
		filters = append(filters, "type="+typeFilter)
	}

	if len(filters) > 0 {
		response.Filter = strings.Join(filters, ", ")
	}

	return marshalToolResponse(response)
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
		return handleLinodeDomainGetRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainGetRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domainID := request.GetInt("domain_id", 0)

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

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
		return handleLinodeDomainRecordsListRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainRecordsListRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domainID := request.GetInt("domain_id", 0)
	typeFilter := request.GetString("type", "")
	nameContains := request.GetString("name_contains", "")

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	records, err := client.ListDomainRecords(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to retrieve domain records for domain %d: %v", domainID, err)), nil
	}

	if typeFilter != "" {
		records = filterDomainRecordsByType(records, typeFilter)
	}

	if nameContains != "" {
		records = filterDomainRecordsByName(records, nameContains)
	}

	return formatDomainRecordsResponse(records, domainID, typeFilter, nameContains)
}

func filterDomainRecordsByType(records []linode.DomainRecord, typeFilter string) []linode.DomainRecord {
	filtered := make([]linode.DomainRecord, 0, len(records))

	typeFilter = strings.ToUpper(typeFilter)

	for _, record := range records {
		if strings.ToUpper(record.Type) == typeFilter {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

func filterDomainRecordsByName(records []linode.DomainRecord, nameContains string) []linode.DomainRecord {
	filtered := make([]linode.DomainRecord, 0, len(records))

	nameContains = strings.ToLower(nameContains)

	for _, record := range records {
		if strings.Contains(strings.ToLower(record.Name), nameContains) {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

func formatDomainRecordsResponse(records []linode.DomainRecord, domainID int, typeFilter, nameContains string) (*mcp.CallToolResult, error) {
	response := struct {
		Count    int                   `json:"count"`
		DomainID int                   `json:"domain_id"` //nolint:tagliatelle // snake_case for consistent JSON
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
