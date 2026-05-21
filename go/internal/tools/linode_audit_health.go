package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// NewLinodeAuditHealthTool returns the linode_audit_health query tool.
// It reports the audit subsystem's own status: the JSONL log path and
// footprint, rotated-file count and oldest date, and (when enabled)
// SQLite row count, oldest event, and database size. CapMeta so it is
// available in every profile. Takes no input parameters.
func NewLinodeAuditHealthTool(
	cfg *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_audit_health",
		mcp.WithDescription(
			"Report the audit subsystem's status: log path and disk usage, "+
				"rotated-file count and oldest date, and (when the SQLite sink "+
				"is enabled) row count, oldest event, and database size.",
		),
	)

	sqlitePath := resolveAuditSQLitePath(cfg)

	handler := func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		report, err := audit.CollectHealth(ctx, sqlitePath, audit.ResolveDefaultAuditDir())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to collect audit health: %v", err)), nil
		}

		body, err := json.Marshal(report)
		if err != nil {
			return nil, fmt.Errorf("marshal audit health report: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}
