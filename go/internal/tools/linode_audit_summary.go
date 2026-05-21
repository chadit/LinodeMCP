package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// auditSummaryResponse is the wire shape of the linode_audit_summary
// result: the per-bucket rows plus the total event count across them.
type auditSummaryResponse struct {
	TotalEvents int                `json:"total_events"`
	Rows        []audit.SummaryRow `json:"rows"`
}

// NewLinodeAuditSummaryTool returns the linode_audit_summary query
// tool. It counts audit events bucketed by the requested columns over
// a time window. CapMeta so it is available in every profile.
//
// Reads the SQLite store when the SQLite sink is enabled (faster,
// indexed window scan), falling back to the JSONL log otherwise. The
// counts are identical either way.
func NewLinodeAuditSummaryTool(
	cfg *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_audit_summary",
		mcp.WithDescription(
			"Count audit events grouped by tool and status (or other columns) "+
				"over a time window. Useful for questions like 'how many destroys "+
				"in the last 24h'. Reads SQLite when enabled, else the JSONL log.",
		),
		mcp.WithString(
			"since",
			mcp.Description("Only count events at or after this RFC 3339 timestamp."),
		),
		mcp.WithArray(
			"group_by",
			mcp.Description(
				"Columns to group by. Allowed: tool, status, capability, "+
					"profile, environment. Defaults to [tool, status].",
			),
			mcp.Items(schemaStringItem()),
		),
		mcp.WithBoolean(
			"include_meta",
			mcp.Description("Include audit/profile meta-tool events. Default false."),
		),
	)

	sqlitePath := resolveAuditSQLitePath(cfg)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		query, err := buildSummaryQuery(&request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		events, err := audit.LoadWindow(ctx, sqlitePath, audit.ResolveDefaultAuditDir(), query.Since, query.IncludeMeta)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
		}

		rows := audit.Summarize(events, query.GroupBy)

		body, err := json.Marshal(auditSummaryResponse{TotalEvents: len(events), Rows: rows})
		if err != nil {
			return nil, fmt.Errorf("marshal audit summary response: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
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
