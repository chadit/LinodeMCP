package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
)

// AuditHealthAnswer reports the audit subsystem's own status: the JSONL
// log path and footprint, rotated-file count and oldest date, and (when
// enabled) SQLite row count, oldest event, and database size.
func AuditHealthAnswer(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	cfg *config.Config,
) (*mcp.CallToolResult, error) {
	report, err := audit.CollectHealth(ctx, resolveAuditSQLitePath(cfg), audit.ResolveDefaultAuditDir())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to collect audit health: %v", err)), nil
	}

	return MarshalProtoToolResponse(auditHealthProto(&report))
}
