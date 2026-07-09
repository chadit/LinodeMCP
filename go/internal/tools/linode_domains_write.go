package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/protobuf/proto"

	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/linode"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
	"github.com/chadit/LinodeMCP/go/internal/twostage"
)

// NewLinodeDomainImportTool creates a tool for importing a domain zone.
func NewLinodeDomainImportTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_import",
		"Imports a DNS domain zone from a remote nameserver that allows zone transfers. Pass dry_run=true to preview without importing.",
		toolschemas.Schema("linode.mcp.v1.DomainImportInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainImportRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeDomainImportRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domain := request.GetString("domain", "")
	remoteNameserver := request.GetString("remote_nameserver", "")

	if IsDryRun(request) {
		if domain == "" {
			return mcp.NewToolResultError("domain is required"), nil
		}

		if remoteNameserver == "" {
			return mcp.NewToolResultError("remote_nameserver is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_domain_import", httpMethodPost, "/domains/import", nil)
	}

	if result := RequireConfirm(request, "This imports a DNS domain zone. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if domain == "" {
		return mcp.NewToolResultError("domain is required"), nil
	}

	if remoteNameserver == "" {
		return mcp.NewToolResultError("remote_nameserver is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.ImportDomainRequest{
		Domain:           domain,
		RemoteNameserver: remoteNameserver,
	}

	importedDomain, err := client.ImportDomainProto(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to import domain: %v", err)), nil
	}

	response := &linodev1.DomainWriteResponse{
		Message: fmt.Sprintf("Domain '%s' (ID: %d) imported successfully", importedDomain.GetDomain(), importedDomain.GetId()),
		Domain:  importedDomain,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeDomainCloneTool creates a tool for cloning a domain.
func NewLinodeDomainCloneTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_clone",
		"Clones a DNS domain and all associated records to a new domain name.",
		toolschemas.Schema("linode.mcp.v1.DomainCloneInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainCloneRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeDomainCloneRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	domain := request.GetString("domain", "")

	if IsDryRun(request) {
		if domainID <= 0 {
			return mcp.NewToolResultError("domain_id must be a positive integer"), nil
		}

		if domain == "" {
			return mcp.NewToolResultError("domain is required"), nil
		}

		return RunDryRunPreview(ctx, request, cfg, "linode_domain_clone", httpMethodPost,
			fmt.Sprintf("/domains/%d/clone", domainID),
			func(ctx context.Context, c *linode.Client) (any, error) { return c.GetDomain(ctx, domainID) })
	}

	if result := RequireConfirm(request, "This clones a DNS domain. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if domainID <= 0 {
		return mcp.NewToolResultError("domain_id must be a positive integer"), nil
	}

	if domain == "" {
		return mcp.NewToolResultError("domain is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CloneDomainRequest{Domain: domain}

	clonedDomain, err := client.CloneDomainProto(ctx, domainID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to clone domain %d: %v", domainID, err)), nil
	}

	response := &linodev1.DomainWriteResponse{
		Message: fmt.Sprintf("Domain %d cloned as '%s' (ID: %d)", domainID, clonedDomain.GetDomain(), clonedDomain.GetId()),
		Domain:  clonedDomain,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeDomainCreateTool creates a tool for creating a domain.
func NewLinodeDomainCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_create",
		"Creates a new DNS domain. Use type 'master' for domains you control, 'slave' for secondary DNS.",
		toolschemas.Schema("linode.mcp.v1.DomainCreateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeDomainCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domain := request.GetString("domain", "")
	domainType := request.GetString("type", "")
	soaEmail := request.GetString("soa_email", "")
	description := request.GetString("description", "")
	ttlSec := request.GetInt("ttl_sec", 0)

	if IsDryRun(request) {
		if domain == "" {
			return mcp.NewToolResultError("domain is required"), nil
		}

		if domainType == "" {
			return mcp.NewToolResultError("type is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_domain_create", httpMethodPost, "/domains", nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return domainCreateSideEffects(ctx, domainType, domain)
			})
	}

	if result := RequireConfirm(request, "This creates a DNS domain. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if domain == "" {
		return mcp.NewToolResultError("domain is required"), nil
	}

	if domainType == "" {
		return mcp.NewToolResultError("type is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.CreateDomainRequest{
		Domain:      domain,
		Type:        domainType,
		SOAEmail:    soaEmail,
		Description: description,
		TTLSec:      ttlSec,
	}

	createdDomain, err := client.CreateDomainProto(ctx, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create domain: %v", err)), nil
	}

	response := &linodev1.DomainWriteResponse{
		Message: fmt.Sprintf("Domain '%s' (ID: %d) created successfully", createdDomain.GetDomain(), createdDomain.GetId()),
		Domain:  createdDomain,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeDomainUpdateTool creates a tool for updating a domain.
func NewLinodeDomainUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_update",
		"Updates an existing DNS domain. Can modify SOA email, description, TTL, and status. Pass dry_run=true to preview without updating.",
		toolschemas.Schema("linode.mcp.v1.DomainUpdateInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeDomainUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)

	if IsDryRun(request) {
		return handleLinodeDomainUpdateDryRun(ctx, request, cfg, domainID)
	}

	domainName := request.GetString("domain", "")
	soaEmail := request.GetString("soa_email", "")
	description := request.GetString("description", "")
	status := request.GetString("status", "")
	ttlSec := request.GetInt("ttl_sec", 0)

	if result := RequireConfirm(request, "This updates a DNS domain. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateDomainRequest{
		Domain:      domainName,
		SOAEmail:    soaEmail,
		Description: description,
		Status:      status,
		TTLSec:      ttlSec,
	}

	updatedDomain, err := client.UpdateDomainProto(ctx, domainID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify domain %d: %v", domainID, err)), nil
	}

	response := &linodev1.DomainWriteResponse{
		Message: fmt.Sprintf("Domain %d modified successfully", domainID),
		Domain:  updatedDomain,
	}

	return MarshalProtoToolResponse(response)
}

// handleLinodeDomainUpdateDryRun fetches the current domain state and
// returns the dry-run preview without making the PUT call. domain_id
// validation runs here too so a malformed dry-run errors out the same
// way the real call would.
func handleLinodeDomainUpdateDryRun(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config, domainID int) (*mcp.CallToolResult, error) {
	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	domain, err := client.GetDomain(ctx, domainID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch domain %d for dry-run: %v", domainID, err)), nil
	}

	details, walkErr := domainUpdateSideEffects(ctx, domain,
		request.GetString("status", ""),
		request.GetString("soa_email", ""),
		request.GetString("description", ""))
	if walkErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to compute dry-run side effects: %v", walkErr)), nil
	}

	return BuildDryRunResponseDetailed(
		"linode_domain_update",
		request.GetString(paramEnvironment, ""),
		"PUT",
		fmt.Sprintf("/domains/%d", domainID),
		domain,
		&details,
	)
}

// NewLinodeDomainDeleteTool creates a tool for deleting a domain.
func NewLinodeDomainDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_delete",
		"Deletes a DNS domain and all its records. WARNING: This action is irreversible. Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.DomainDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// domainDeleteProto builds the proto-canonical id-echo body for a successful
// domain delete, keeping the proto literal off the handler's struct literal so
// the delete handlers stay below the dupl threshold.
func domainDeleteProto(id int) proto.Message {
	return &linodev1.DomainDeleteResponse{
		Message:  fmt.Sprintf("Domain %d and all its records removed successfully", id),
		DomainId: linodeIDToInt32(id),
	}
}

func handleLinodeDomainDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionWithID(ctx, request, cfg, &DestructiveActionByID{
		ToolName:       "linode_domain_delete",
		IDParam:        "domain_id",
		Method:         httpMethodDelete,
		PathPattern:    "/domains/%d",
		ConfirmMessage: "This operation is destructive and deletes all DNS records. Set confirm=true to proceed.",
		SuccessProto:   domainDeleteProto,
		FetchState:     func(ctx context.Context, c *linode.Client, id int) (any, error) { return c.GetDomain(ctx, id) },
		Execute:        func(ctx context.Context, c *linode.Client, id int) error { return c.DeleteDomain(ctx, id) },
		DependencyWalk: domainDeleteDependencyWalk,
		HashIgnore:     twostage.HashIgnoreFields("Domain"),
	})
}
