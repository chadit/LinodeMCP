package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

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
	tool := mcp.NewToolWithRawSchema(
		"linode_audit_recent",
		"Return the most recent audit events (what tools were called, with what "+
			"outcome), newest first. Reads the on-disk JSONL audit log. Optional "+
			"filters: limit, since, until, tool (glob), capability, status, include_meta.",
		toolschemas.Schema("linode.mcp.v1.AuditRecentInput"),
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

		protoEvents, err := auditEventsProto(events)
		if err != nil {
			return nil, err
		}

		return MarshalProtoToolResponse(&linodev1.AuditRecentResponse{
			Count:  linodeIDToInt32(len(events)),
			Events: protoEvents,
		})
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
