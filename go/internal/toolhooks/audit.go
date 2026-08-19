package toolhooks

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// The five audit answers, each forwarding to the reader that assembles it.
//
// The readers live beside the audit stores' other shared code rather than here
// because Python's do: its hooks forward the same way, so a change to what an
// audit tool answers is one edit per language in one place.

// LinodeAuditHealthAnswer reports the audit subsystem's own status: log path
// and footprint, rotated-file count and oldest date, and the SQLite section
// when that sink is enabled.
func LinodeAuditHealthAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.AuditHealthAnswer(ctx, request, cfg))
}

// LinodeAuditRecentAnswer reads the most recent audit events from the JSONL
// sink, newest first, applying the filters the call names.
func LinodeAuditRecentAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.AuditRecentAnswer(ctx, request, cfg))
}

// LinodeAuditSummaryAnswer counts audit events bucketed by the requested
// columns over a time window.
func LinodeAuditSummaryAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.AuditSummaryAnswer(ctx, request, cfg))
}

// LinodeAuditExportAnswer dumps a filtered window of audit events to a temp
// file and answers with its path.
func LinodeAuditExportAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.AuditExportAnswer(ctx, request, cfg))
}

// LinodeAuditReportAnswer runs a named report from the configured catalog
// against the active event store.
func LinodeAuditReportAnswer(
	ctx context.Context, request *mcp.CallToolRequest, cfg *config.Config,
) (*mcp.CallToolResult, error) {
	return answeredBy(tools.AuditReportAnswer(ctx, request, cfg))
}
