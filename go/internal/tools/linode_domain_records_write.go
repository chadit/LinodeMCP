package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/linode"
	"github.com/chadit/LinodeMCP/internal/profiles"
	"github.com/chadit/LinodeMCP/internal/twostage"
)

// NewLinodeDomainRecordCreateTool creates a tool for creating a domain record.
func NewLinodeDomainRecordCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_record_create",
		mcp.WithDescription("Creates a new DNS record within a domain. Supports A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, and PTR record types."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain to add the record to"),
		),
		mcp.WithString(
			"type",
			mcp.Required(),
			mcp.Description("Record type: A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, or PTR"),
		),
		mcp.WithString(
			"name",
			mcp.Description("The hostname or subdomain (e.g., 'www', 'mail'). Leave empty for root domain."),
		),
		mcp.WithString(
			"target",
			mcp.Required(),
			mcp.Description("The target value (e.g., IP address for A records, hostname for CNAME)"),
		),
		mcp.WithNumber(
			"priority",
			mcp.Description("Priority for MX and SRV records (optional)"),
		),
		mcp.WithNumber(
			"weight",
			mcp.Description("Weight for SRV records (optional)"),
		),
		mcp.WithNumber(
			"port",
			mcp.Description("Port for SRV records (optional)"),
		),
		mcp.WithString(
			"service",
			mcp.Description("Service name for SRV records (e.g., '_http')"),
		),
		mcp.WithString(
			"protocol",
			mcp.Description("Protocol for SRV records (e.g., '_tcp', '_udp')"),
		),
		mcp.WithNumber(
			"ttl_sec",
			mcp.Description("TTL in seconds (optional, uses domain default if not specified)"),
		),
		mcp.WithString(
			"tag",
			mcp.Description("Tag for CAA records: 'issue', 'issuewild', or 'iodef' (optional)"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm DNS record creation. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordCreateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

// validateDomainRecordCreateArgs validates the create args, returning an
// error message or "". Shared by the real create path and the dry-run
// preview so both reject the same malformed inputs.
func validateDomainRecordCreateArgs(domainID int, recordType, name, target string) string {
	if domainID == 0 {
		return "domain_id is required"
	}

	if recordType == "" {
		return "type is required"
	}

	if err := validateDNSRecordName(name); err != nil {
		return err.Error()
	}

	if err := validateDNSRecordTarget(recordType, target); err != nil {
		return err.Error()
	}

	return ""
}

func handleLinodeDomainRecordCreateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
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

	if IsDryRun(request) {
		if msg := validateDomainRecordCreateArgs(domainID, recordType, name, target); msg != "" {
			return mcp.NewToolResultError(msg), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_domain_record_create", httpMethodPost,
			fmt.Sprintf("/domains/%d/records", domainID), nil,
			func(ctx context.Context, _ *linode.Client, _ any) (DryRunDetails, error) {
				return domainRecordCreateSideEffects(ctx, recordType, name, target, domainID)
			})
	}

	if result := RequireConfirm(request, "This creates a DNS record. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if msg := validateDomainRecordCreateArgs(domainID, recordType, name, target); msg != "" {
		return mcp.NewToolResultError(msg), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

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

	record, err := client.CreateDomainRecord(ctx, domainID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create domain record: %v", err)), nil
	}

	response := struct {
		Message  string               `json:"message"`
		DomainID int                  `json:"domain_id"`
		Record   *linode.DomainRecord `json:"record"`
	}{
		Message:  fmt.Sprintf("%s record (ID: %d) created successfully", record.Type, record.ID),
		DomainID: domainID,
		Record:   record,
	}

	return MarshalToolResponse(response)
}

// NewLinodeDomainRecordUpdateTool creates a tool for updating a domain record.
func NewLinodeDomainRecordUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_domain_record_update",
		mcp.WithDescription("Updates an existing DNS record. Note: Record type cannot be changed."),
		mcp.WithString(
			paramEnvironment,
			mcp.Description(paramEnvironmentDesc),
		),
		mcp.WithNumber(
			"domain_id",
			mcp.Required(),
			mcp.Description("The ID of the domain containing the record"),
		),
		mcp.WithNumber(
			"record_id",
			mcp.Required(),
			mcp.Description("The ID of the record to update"),
		),
		mcp.WithString(
			"name",
			mcp.Description("New hostname or subdomain (optional)"),
		),
		mcp.WithString(
			"target",
			mcp.Description("New target value (optional)"),
		),
		mcp.WithNumber(
			"priority",
			mcp.Description("New priority for MX and SRV records (optional)"),
		),
		mcp.WithNumber(
			"weight",
			mcp.Description("New weight for SRV records (optional)"),
		),
		mcp.WithNumber(
			"port",
			mcp.Description("New port for SRV records (optional)"),
		),
		mcp.WithNumber(
			"ttl_sec",
			mcp.Description("New TTL in seconds (optional)"),
		),
		mcp.WithBoolean(
			paramConfirm,
			mcp.Required(),
			mcp.Description("Must be set to true to confirm DNS record update. Ignored when dry_run=true."),
		),
		mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordUpdateRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapWrite, handler
}

func handleLinodeDomainRecordUpdateRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	domainID := request.GetInt("domain_id", 0)
	recordID := request.GetInt("record_id", 0)
	name := request.GetString("name", "")
	target := request.GetString("target", "")
	priority := request.GetInt("priority", 0)
	weight := request.GetInt("weight", 0)
	port := request.GetInt("port", 0)
	ttlSec := request.GetInt("ttl_sec", 0)

	if IsDryRun(request) {
		if domainID == 0 {
			return mcp.NewToolResultError("domain_id is required"), nil
		}

		if recordID == 0 {
			return mcp.NewToolResultError("record_id is required"), nil
		}

		return RunDryRunPreviewDetailed(ctx, request, cfg, "linode_domain_record_update", "PUT",
			fmt.Sprintf("/domains/%d/records/%d", domainID, recordID),
			func(ctx context.Context, c *linode.Client) (any, error) {
				return c.GetDomainRecord(ctx, domainID, recordID)
			},
			func(ctx context.Context, _ *linode.Client, state any) (DryRunDetails, error) {
				return domainRecordUpdateSideEffects(ctx, state, name, target)
			})
	}

	if result := RequireConfirm(request, "This updates a DNS record. Set confirm=true to proceed."); result != nil {
		return result, nil
	}

	if domainID == 0 {
		return mcp.NewToolResultError("domain_id is required"), nil
	}

	if recordID == 0 {
		return mcp.NewToolResultError("record_id is required"), nil
	}

	client, err := prepareClient(request, cfg)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	req := linode.UpdateDomainRecordRequest{
		Name:     name,
		Target:   target,
		Priority: priority,
		Weight:   weight,
		Port:     port,
		TTLSec:   ttlSec,
	}

	record, err := client.UpdateDomainRecord(ctx, domainID, recordID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify record %d: %v", recordID, err)), nil
	}

	response := struct {
		Message  string               `json:"message"`
		DomainID int                  `json:"domain_id"`
		Record   *linode.DomainRecord `json:"record"`
	}{
		Message:  fmt.Sprintf("Record %d modified successfully", recordID),
		DomainID: domainID,
		Record:   record,
	}

	return MarshalToolResponse(response)
}

// NewLinodeDomainRecordDeleteTool creates a tool for deleting a domain record.
func NewLinodeDomainRecordDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool, handler := newToolWithHandler(
		cfg,
		"linode_domain_record_delete",
		"Deletes a DNS record from a domain. Pass dry_run=true to preview without deleting."+twoStageNote,
		[]mcp.ToolOption{
			mcp.WithNumber("domain_id", mcp.Required(),
				mcp.Description("The ID of the domain containing the record")),
			mcp.WithNumber("record_id", mcp.Required(),
				mcp.Description("The ID of the record to delete")),
			mcp.WithBoolean(paramConfirm, mcp.Required(),
				mcp.Description("Must be set to true to confirm DNS record deletion. This action is irreversible. Ignored when dry_run=true.")),
			mcp.WithBoolean(paramDryRun, mcp.Description(paramDryRunDesc)),
			mcp.WithString(paramMode, mcp.Description(paramModeDesc)),
			mcp.WithString(paramPlanID, mcp.Description(paramPlanIDDesc)),
		},
		handleLinodeDomainRecordDeleteRequest,
	)

	return tool, profiles.CapDestroy, handler
}

func handleLinodeDomainRecordDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_domain_record_delete",
		OuterIDParam:   "domain_id",
		InnerIDParam:   "record_id",
		Method:         httpMethodDelete,
		PathPattern:    "/domains/%d/records/%d",
		ConfirmMessage: "This deletes a DNS record and is irreversible. Set confirm=true to proceed.",
		SuccessFormat:  "Record %d removed successfully from domain %d",
		FetchState: func(ctx context.Context, c *linode.Client, domainID, recordID int) (any, error) {
			return c.GetDomainRecord(ctx, domainID, recordID)
		},
		Execute: func(ctx context.Context, c *linode.Client, domainID, recordID int) error {
			return c.DeleteDomainRecord(ctx, domainID, recordID)
		},
		HashIgnore: twostage.HashIgnoreFields("DomainRecord"),
	})
}
