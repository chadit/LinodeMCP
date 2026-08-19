package tools

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// AuditSummaryAnswer counts audit events bucketed by the requested
// columns over a time window.
//
// It reads the SQLite store when that sink is enabled (an indexed window scan),
// falling back to the JSONL log otherwise. The counts are identical either way.
func AuditSummaryAnswer(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
) (*mcp.CallToolResult, error) {
	query, err := buildSummaryQuery(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	events, err := audit.LoadWindow(
		ctx, resolveAuditSQLitePath(cfg), audit.ResolveDefaultAuditDir(), query.Since, query.IncludeMeta,
	)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
	}

	rows := audit.Summarize(events, query.GroupBy)

	return MarshalProtoToolResponse(&linodev1.AuditSummaryResponse{
		TotalEvents: IDToInt32(len(events)),
		Rows:        auditSummaryRowsProto(rows),
	})
}

// buildSummaryQuery translates request parameters into a validated
// SummaryQuery. Returns an error for a malformed since timestamp or an
// unknown group_by column.
func buildSummaryQuery(request *mcp.CallToolRequest) (*audit.SummaryQuery, error) {
	since, err := parseOptionalTime(request.GetString("since", ""))
	if err != nil {
		return nil, fmt.Errorf("invalid 'since' timestamp: %w", err)
	}

	groupBy, err := audit.ValidateGroupBy(request.GetStringSlice("group_by", nil))
	if err != nil {
		return nil, fmt.Errorf("invalid group_by: %w", err)
	}

	return &audit.SummaryQuery{
		Since:       since,
		GroupBy:     groupBy,
		IncludeMeta: request.GetBool("include_meta", false),
	}, nil
}

// resolveAuditSQLitePath returns the SQLite database path when the
// SQLite sink is enabled (the configured path, or audit.db beside the
// JSONL log), or empty string when SQLite is off (signaling the JSONL
// fallback to audit.LoadWindow).
func resolveAuditSQLitePath(cfg *config.Config) string {
	if cfg == nil || !cfg.Audit.SQLite.Enabled {
		return ""
	}

	if cfg.Audit.SQLite.Path != "" {
		return cfg.Audit.SQLite.Path
	}

	return filepath.Join(audit.ResolveDefaultAuditDir(), "audit.db")
}
