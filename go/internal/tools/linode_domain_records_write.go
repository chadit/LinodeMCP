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

// NewLinodeDomainRecordCreateTool creates a tool for creating a domain record.
func NewLinodeDomainRecordCreateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_record_create",
		"Creates a new DNS record within a domain. Supports A, AAAA, NS, MX, CNAME, TXT, SRV, CAA, and PTR record types.",
		toolschemas.Schema("linode.mcp.v1.DomainRecordCreateInput"),
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

	record, err := client.CreateDomainRecordProto(ctx, domainID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create domain record: %v", err)), nil
	}

	response := &linodev1.DomainRecordWriteResponse{
		Message: fmt.Sprintf("%s record (ID: %d) created successfully", record.GetType(), record.GetId()),
		Record:  record,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeDomainRecordUpdateTool creates a tool for updating a domain record.
func NewLinodeDomainRecordUpdateTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_record_update",
		"Updates an existing DNS record. Note: Record type cannot be changed.",
		toolschemas.Schema("linode.mcp.v1.DomainRecordUpdateInput"),
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

	record, err := client.UpdateDomainRecordProto(ctx, domainID, recordID, &req)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to modify record %d: %v", recordID, err)), nil
	}

	response := &linodev1.DomainRecordWriteResponse{
		Message: fmt.Sprintf("Record %d modified successfully", recordID),
		Record:  record,
	}

	return MarshalProtoToolResponse(response)
}

// NewLinodeDomainRecordDeleteTool creates a tool for deleting a domain record.
func NewLinodeDomainRecordDeleteTool(cfg *config.Config) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_domain_record_delete",
		"Deletes a DNS record from a domain. Pass dry_run=true to preview without deleting."+twoStageNote,
		toolschemas.Schema("linode.mcp.v1.DomainRecordDeleteInput"),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return handleLinodeDomainRecordDeleteRequest(ctx, &request, cfg)
	}

	return tool, profiles.CapDestroy, handler
}

// domainRecordDeleteProto builds the proto-canonical id-echo body for a
// successful domain-record delete, keeping the proto literal off the handler's
// struct literal so the delete handlers stay below the dupl threshold.
func domainRecordDeleteProto(domainID, recordID int) proto.Message {
	return &linodev1.DomainRecordDeleteResponse{
		Message:  fmt.Sprintf("Record %d removed successfully from domain %d", recordID, domainID),
		DomainId: linodeIDToInt32(domainID),
		RecordId: linodeIDToInt32(recordID),
	}
}

func handleLinodeDomainRecordDeleteRequest(ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config) (*mcp.CallToolResult, error) {
	return RunDestructiveActionByTwoIDs(ctx, request, cfg, &DestructiveActionByTwoIDs{
		ToolName:       "linode_domain_record_delete",
		OuterIDParam:   "domain_id",
		InnerIDParam:   "record_id",
		Method:         httpMethodDelete,
		PathPattern:    "/domains/%d/records/%d",
		ConfirmMessage: "This deletes a DNS record and is irreversible. Set confirm=true to proceed.",
		SuccessProto:   domainRecordDeleteProto,
		FetchState: func(ctx context.Context, c *linode.Client, domainID, recordID int) (any, error) {
			return c.GetDomainRecord(ctx, domainID, recordID)
		},
		Execute: func(ctx context.Context, c *linode.Client, domainID, recordID int) error {
			return c.DeleteDomainRecord(ctx, domainID, recordID)
		},
		HashIgnore: twostage.HashIgnoreFields("DomainRecord"),
	})
}
