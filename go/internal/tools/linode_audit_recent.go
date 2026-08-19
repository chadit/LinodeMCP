package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// AuditRecentAnswer reads the most recent audit events from the JSONL
// sink (active log plus rotated files), newest first, applying the filters the
// call names.
func AuditRecentAnswer(
	ctx context.Context,
	request *mcp.CallToolRequest,
	_ *config.Config,
) (*mcp.CallToolResult, error) {
	// audit.ReadRecent takes no context, so cancellation is honored here
	// rather than after the log walk.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("linode_audit_recent canceled: %w", err)
	}

	query, err := buildRecentQuery(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	events, err := audit.ReadRecent(audit.ResolveDefaultAuditDir(), query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.AuditRecentResponse{
		Count:  IDToInt32(len(events)),
		Events: auditEventsProto(events),
	})
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
