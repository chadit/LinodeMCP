//nolint:dupl // Tool implementations have similar structure by design
package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
)

// NewLinodeDomainCreateTool creates a tool for creating a domain.
func NewLinodeDomainCreateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_create",
		mcp.WithDescription("Creates a new DNS domain. Use type 'master' for domains you control, 'slave' for secondary DNS."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithString("domain",
			mcp.Required(),
			mcp.Description("The domain name (e.g., 'example.com')"),
		),
		mcp.WithString("type",
			mcp.Required(),
			mcp.Description("Domain type: 'master' (primary) or 'slave' (secondary)"),
		),
		mcp.WithString("soa_email",
			mcp.Description("Start of Authority email address (required for master domains)"),
		),
		mcp.WithString("description",
			mcp.Description("A description for the domain (optional)"),
		),
		mcp.WithNumber("ttl_sec",
			mcp.Description("Default TTL in seconds for records (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainCreateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainCreateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domain := request.GetString("domain", "")
	domainType := request.GetString("type", "")
	soaEmail := request.GetString("soa_email", "")
	description := request.GetString("description", "")
	ttlSec := request.GetInt("ttl_sec", 0)

	if domain == "" {
		return mcp.NewToolResultError("domain is required"), nil
	}

	if domainType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	selectedEnv, err := selectEnvironment(cfg, environment)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	if err := validateLinodeConfig(selectedEnv); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	client := linode.NewRetryableClientWithDefaults(selectedEnv.Linode.APIURL, selectedEnv.Linode.Token)

	req := linode.CreateDomainRequest{
		Domain:      domain,
		Type:        domainType,
		SOAEmail:    soaEmail,
		Description: description,
		TTLSec:      ttlSec,
	}

	createdDomain, err := client.CreateDomain(ctx, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create domain: %v", err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		Domain  *linode.Domain `json:"domain"`
	}{
		Message: fmt.Sprintf("Domain '%s' (ID: %d) created successfully", createdDomain.Domain, createdDomain.ID),
		Domain:  createdDomain,
	}

	return marshalToolResponse(response)
}

// NewLinodeDomainUpdateTool creates a tool for updating a domain.
func NewLinodeDomainUpdateTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_update",
		mcp.WithDescription("Updates an existing DNS domain. Can modify SOA email, description, TTL, and status."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to update"),
		),
		mcp.WithString("soa_email",
			mcp.Description("New SOA email address (optional)"),
		),
		mcp.WithString("description",
			mcp.Description("New description (optional)"),
		),
		mcp.WithString("status",
			mcp.Description("New status: 'active', 'disabled', or 'edit_mode' (optional)"),
		),
		mcp.WithNumber("ttl_sec",
			mcp.Description("New default TTL in seconds (optional)"),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainUpdateRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainUpdateRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domainID := request.GetInt("domain_id", 0)
	soaEmail := request.GetString("soa_email", "")
	description := request.GetString("description", "")
	status := request.GetString("status", "")
	ttlSec := request.GetInt("ttl_sec", 0)

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

	req := linode.UpdateDomainRequest{
		SOAEmail:    soaEmail,
		Description: description,
		Status:      status,
		TTLSec:      ttlSec,
	}

	updatedDomain, err := client.UpdateDomain(ctx, domainID, req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update domain %d: %v", domainID, err)), nil
	}

	response := struct {
		Message string         `json:"message"`
		Domain  *linode.Domain `json:"domain"`
	}{
		Message: fmt.Sprintf("Domain %d updated successfully", domainID),
		Domain:  updatedDomain,
	}

	return marshalToolResponse(response)
}

// NewLinodeDomainDeleteTool creates a tool for deleting a domain.
func NewLinodeDomainDeleteTool(cfg *config.Config) (mcp.Tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool("linode_domain_delete",
		mcp.WithDescription("Deletes a DNS domain and all its records. WARNING: This action is irreversible."),
		mcp.WithString(paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber("domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to delete"),
		),
		mcp.WithBoolean(paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm deletion. This deletes all DNS records."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainDeleteRequest(ctx, request, cfg)
	}

	return tool, handler
}

func handleLinodeDomainDeleteRequest(ctx context.Context, request mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	environment := request.GetString(paramEnvironment, "")
	domainID := request.GetInt("domain_id", 0)
	confirm := request.GetBool(paramConfirm, false)

	if !confirm {
		return mcp.NewToolResultError("This operation is destructive and deletes all DNS records. Set confirm=true to proceed."), nil
	}

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

	if err := client.DeleteDomain(ctx, domainID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete domain %d: %v", domainID, err)), nil
	}

	response := struct {
		Message  string `json:"message"`
		DomainID int    `json:"domain_id"` //nolint:tagliatelle // snake_case for consistent JSON
	}{
		Message:  fmt.Sprintf("Domain %d and all its records deleted successfully", domainID),
		DomainID: domainID,
	}

	return marshalToolResponse(response)
}
