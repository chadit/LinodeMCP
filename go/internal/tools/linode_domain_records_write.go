package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeDomainRecordCreateTool creates a tool for creating a domain record.
func NewLinodeDomainRecordCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_record_create",
		mcp.WithDescription("Creates a new DNS record within a domain. Supports A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, and PTR record types."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to add the record to"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Record type: A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, or PTR"),
		),
		mcp.WithString("name",
			mcp.Description("The hostname or subdomain (e.g., 'www', 'mail'). Leave empty for root domain."),
		),
		mcp.WithString("target",
			mcp.Required(),
			mcp.Description("The target value (e.g., IP address for A records, hostname for CNAME)"),
		),
		mcp.WithNumber("priority",
			mcp.Description("Priority for MX and SRV records (optional)"),
		),
		mcp.WithNumber("weight",
			mcp.Description("Weight for SRV records (optional)"),
		),
		mcp.WithNumber("port",
			mcp.Description("Port for SRV records (optional)"),
		),
		mcp.WithString("service",
			mcp.Description("Service name for SRV records (e.g., '_http')"),
		),
		mcp.WithString("protocol",
			mcp.Description("Protocol for SRV records (e.g., '_tcp', '_udp')"),
		),
		mcp.WithNumber("ttl_sec",
			mcp.Description("TTL in seconds (optional, uses domain default if not specified)"),
		),
		mcp.WithString("tag",
			mcp.Description("Tag for CAA records: 'issue', 'issuewild', or 'iodef' (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainRecordCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	domainID := request.GetInt("domain_id", 0)
	recordType := request.GetString("type", "")
	name := request.GetString("name", "")
	target := request.GetString("target", "")
	priority := request.GetInt("priority", 0)
	weight := request.GetInt("weight", 0)
	port := request.GetInt("port", 0)
	service := request.GetString("service", "")
	protocol := request.GetString("protocol", "")
	ttlSec := request.GetInt("ttl_sec", 0)
	tag := request.GetString("tag", "")

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	if recordType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	if err := validateDNSRecordName(name); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateDNSRecordTarget(recordType, target); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.CreateDomainRecordRequest{
		Type:     recordType,
		Name:     name,
		Target:   target,
		Priority: priority,
		Weight:   weight,
		Port:     port,
		Service:  service,
		Protocol: protocol,
		TTLSec:   ttlSec,
		Tag:      tag,
	}

	record, err := client.CreateDomainRecord(ctx, domainID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create domain record: %v", err)), nil
	}

	response := struct {
		Message  string               `json:"message"`
		DomainID int                  `json:"domain_id"` //nolint:tagliatelle // snake_case for consistent JSON
		Record   *linode.DomainRecord `json:"record"`
	}{
		Message:  fmt.Sprintf("%s record (ID: %d) created successfully", record.Type, record.ID),
		DomainID: domainID,
		Record:   record,
	}

	return marshalToolResponse(response)
}

// NewLinodeDomainRecordUpdateTool creates a tool for updating a domain record.
func NewLinodeDomainRecordUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_record_update",
		mcp.WithDescription("Updates an existing DNS record. Note: Record type cannot be changed."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain containing the record"),
		),
		mcp.WithNumber("record_id",
			mcp.Required(),
			mcp.Description("The ID of the record to update"),
		),
		mcp.WithString("name",
			mcp.Description("New hostname or subdomain (optional)"),
		),
		mcp.WithString("target",
			mcp.Description("New target value (optional)"),
		),
		mcp.WithNumber("priority",
			mcp.Description("New priority for MX and SRV records (optional)"),
		),
		mcp.WithNumber("weight",
			mcp.Description("New weight for SRV records (optional)"),
		),
		mcp.WithNumber("port",
			mcp.Description("New port for SRV records (optional)"),
		),
		mcp.WithNumber("ttl_sec",
			mcp.Description("New TTL in seconds (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordUpdateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainRecordUpdateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	domainID := request.GetInt("domain_id", 0)
	recordID := request.GetInt("record_id", 0)
	name := request.GetString("name", "")
	target := request.GetString("target", "")
	priority := request.GetInt("priority", 0)
	weight := request.GetInt("weight", 0)
	port := request.GetInt("port", 0)
	ttlSec := request.GetInt("ttl_sec", 0)

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	if recordID == 0 {
		return mcp.NewToolResultError("record_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.UpdateDomainRecordRequest{
		Name:     name,
		Target:   target,
		Priority: priority,
		Weight:   weight,
		Port:     port,
		TTLSec:   ttlSec,
	}

	record, err := client.UpdateDomainRecord(ctx, domainID, recordID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update record %d: %v", recordID, err)), nil
	}

	response := struct {
		Message  string               `json:"message"`
		DomainID int                  `json:"domain_id"` //nolint:tagliatelle // snake_case for consistent JSON
		Record   *linode.DomainRecord `json:"record"`
	}{
		Message:  fmt.Sprintf("Record %d updated successfully", recordID),
		DomainID: domainID,
		Record:   record,
	}

	return marshalToolResponse(response)
}

// NewLinodeDomainRecordDeleteTool creates a tool for deleting a domain record.
func NewLinodeDomainRecordDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_record_delete",
		mcp.WithDescription("Deletes a DNS record from a domain."),
		mcp.WithString("environment",
			mcp.Description("Linode environment to use (optional, defaults to 'default')"),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain containing the record"),
		),
		mcp.WithNumber("record_id",
			mcp.Required(),
			mcp.Description("The ID of the record to delete"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainRecordDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString("environment", "")
	domainID := request.GetInt("domain_id", 0)
	recordID := request.GetInt("record_id", 0)

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	if recordID == 0 {
		return mcp.NewToolResultError("record_id is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	if err := client.DeleteDomainRecord(ctx, domainID, recordID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete record %d: %v", recordID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		DomainID int    `json:"domain_id"` //nolint:tagliatelle // snake_case for consistent JSON
		RecordID int    `json:"record_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:  fmt.Sprintf("Record %d deleted successfully from domain %d", recordID, domainID),
		DomainID: domainID,
		RecordID: recordID,
	}

	return marshalToolResponse(response)
}
