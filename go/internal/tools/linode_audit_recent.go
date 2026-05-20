package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// auditRecentResponse is the wire shape of the linode_audit_recent
// result. Count is the number of events returned (after filtering and
// the limit), Events is the newest-first list.
type auditRecentResponse struct {
	Count  int           `json:"count"`
	Events []audit.Event `json:"events"`
}

// NewLinodeAuditRecentTool returns the linode_audit_recent query tool.
// It reads the most recent audit events from the JSONL sink (active
// log plus rotated files), newest first, applying optional filters.
//
// Capability is CapMeta so the tool is available in every profile,
// including read-only ones: inspecting what the assistant did should
// never require write access. Meta events (the audit and profile-
// builder tools' own calls) are excluded unless include_meta is true,
// so the default view shows Linode activity rather than the
// assistant's bookkeeping.
func NewLinodeAuditRecentTool(
	_ *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_audit_recent",
		mcp.WithDescription(
			"Return the most recent audit events (what tools were called, with what "+
				"outcome), newest first. Reads the on-disk JSONL audit log. Optional "+
				"filters: limit, since, until, tool (glob), capability, status, include_meta.",
		),
		mcp.WithNumber(
			"limit",
			mcp.Description("Max events to return. Default 20, capped at 200."),
		),
		mcp.WithString(
			"since",
			mcp.Description("Only events at or after this RFC 3339 timestamp (e.g. 2026-05-19T00:00:00Z)."),
		),
		mcp.WithString(
			"until",
			mcp.Description("Only events at or before this RFC 3339 timestamp."),
		),
		mcp.WithString(
			"tool",
			mcp.Description(`Only events whose tool name matches this glob (e.g. "linode_instance_*").`),
		),
		mcp.WithString(
			"capability",
			mcp.Description("Only events with this capability: read, write, destroy, admin, or meta."),
		),
		mcp.WithString(
			"status",
			mcp.Description("Only events with this status: success, error, or refused."),
		),
		mcp.WithBoolean(
			"include_meta",
			mcp.Description("Include audit/profile meta-tool events. Default false (they are noise for activity review)."),
		),
	)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		query, err := buildRecentQuery(&request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		events, err := audit.ReadRecent(audit.ResolveDefaultAuditDir(), query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
		}

		body, err := json.Marshal(auditRecentResponse{Count: len(events), Events: events})
		if err != nil {
			return nil, fmt.Errorf("marshal audit recent response: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// buildRecentQuery translates the request parameters into a
// RecentQuery. Returns an error for a malformed since/until timestamp
// so the caller can surface it rather than silently ignoring the
// filter.
func buildRecentQuery(request *mcp.CallToolRequest) (*audit.RecentQuery, error) {
	query := &audit.RecentQuery{
		Limit:       request.GetInt("limit", 0),
		Tool:        request.GetString("tool", ""),
		Capability:  audit.Capability(request.GetString("capability", "")),
		Status:      audit.Status(request.GetString("status", "")),
		IncludeMeta: request.GetBool("include_meta", false),
	}

	since, err := parseOptionalTime(request.GetString("since", ""))
	if err != nil {
		return nil, fmt.Errorf("invalid 'since' timestamp: %w", err)
	}

	query.Since = since

	until, err := parseOptionalTime(request.GetString("until", ""))
	if err != nil {
		return nil, fmt.Errorf("invalid 'until' timestamp: %w", err)
	}

	query.Until = until

	return query, nil
}

// parseOptionalTime parses an RFC 3339 timestamp, returning the zero
// time for an empty string (meaning "no bound"). A non-empty but
// unparseable value is an error.
func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("expected RFC 3339, got %q: %w", value, err)
	}

	return parsed, nil
}
