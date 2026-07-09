package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeAuditHealthTool returns the linode_audit_health query tool.
// It reports the audit subsystem's own status: the JSONL log path and
// footprint, rotated-file count and oldest date, and (when enabled)
// SQLite row count, oldest event, and database size. CapMeta so it is
// available in every profile. Takes no input parameters.
func NewLinodeAuditHealthTool(
	cfg *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_audit_health",
		"Report the audit subsystem's status: log path and disk usage, "+
			"rotated-file count and oldest date, and (when the SQLite sink "+
			"is enabled) row count, oldest event, and database size.",
		toolschemas.Schema("linode.mcp.v1.AuditHealthInput"),
	)

	sqlitePath := resolveAuditSQLitePath(cfg)

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		report, err := audit.CollectHealth(ctx, sqlitePath, audit.ResolveDefaultAuditDir())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to collect audit health: %v", err)), nil
		}

		return MarshalProtoToolResponse(auditHealthProto(&report))
	}

	return tool, profiles.CapMeta, handler
}
