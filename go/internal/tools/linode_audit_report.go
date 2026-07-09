package tools

import (
	"context"
	"fmt"
	"path"
	"slices"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/toolschemas"
)

// NewLinodeAuditReportTool returns the linode_audit_report query tool.
// It runs a user-defined named report from audit.reports against the
// active event store. CapMeta so it is available in every profile. The
// report definition is resolved from cfg at call time, so editing the
// report file takes effect on the next call.
func NewLinodeAuditReportTool(
	cfg *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewToolWithRawSchema(
		"linode_audit_report",
		"Run a named custom audit report from config (audit.reports). "+
			"Reads SQLite when enabled, else the JSONL log. Returns a "+
			"summary of counts or a list of matching events depending on "+
			"the report's output mode.",
		toolschemas.Schema("linode.mcp.v1.AuditReportInput"),
	)

	sqlitePath := resolveAuditSQLitePath(cfg)

	handler := func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("report name is required"), nil
		}

		report, ok := cfg.Audit.Reports[name]
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown report: %q", name)), nil
		}

		result, err := runReport(ctx, sqlitePath, audit.ResolveDefaultAuditDir(), name, &report, time.Now())
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to run report: %v", err)), nil
		}

		return MarshalProtoToolResponse(result)
	}

	return tool, profiles.CapMeta, handler
}

// runReport loads the windowed events for the report, applies the
// post-filter (the _in lists, environment and profile globs that
// RecentQuery doesn't carry), and emits summary or list output. Now is
// the reference time for since_offset; injected so tests can pin it.
func runReport(
	ctx context.Context,
	sqlitePath, dir, name string,
	report *config.ReportConfig,
	now time.Time,
) (*linodev1.AuditReportResponse, error) {
	query, err := buildReportLoadQuery(&report.Filter, now)
	if err != nil {
		return nil, err
	}

	events, err := audit.ExportEvents(ctx, sqlitePath, dir, query)
	if err != nil {
		return nil, fmt.Errorf("load report events: %w", err)
	}

	predicate := compileReportPredicate(&report.Filter)
	filtered := events[:0]

	for idx := range events {
		if predicate(&events[idx]) {
			filtered = append(filtered, events[idx])
		}
	}

	result := &linodev1.AuditReportResponse{
		Name:        name,
		Output:      report.Output,
		TotalEvents: linodeIDToInt32(len(filtered)),
	}

	if report.Output == config.ReportOutputSummary {
		groupBy, gbErr := audit.ValidateGroupBy(report.GroupBy)
		if gbErr != nil {
			return nil, fmt.Errorf("validate report group_by: %w", gbErr)
		}

		result.Rows = auditSummaryRowsProto(audit.Summarize(filtered, groupBy))

		return result, nil
	}

	// list output
	if report.Limit > 0 && len(filtered) > report.Limit {
		filtered = filtered[:report.Limit]
		result.TotalEvents = linodeIDToInt32(len(filtered))
	}

	protoEvents, err := auditEventsProto(filtered)
	if err != nil {
		return nil, err
	}

	result.Events = protoEvents

	return result, nil
}

// buildReportLoadQuery translates the filter fields RecentQuery carries
// (tool glob, scalar capability/status, since/until) into a load-time
// query. IncludeMeta is true: the report grammar (capability /
// capability_in) controls meta inclusion explicitly, not the
// tool-layer default. since_offset takes precedence over the absolute
// since when both are set.
func buildReportLoadQuery(filter *config.ReportFilter, now time.Time) (*audit.RecentQuery, error) {
	since, err := resolveReportSince(filter, now)
	if err != nil {
		return nil, err
	}

	until, err := resolveReportUntil(filter)
	if err != nil {
		return nil, err
	}

	return &audit.RecentQuery{
		Limit:       audit.MaxExportRecords,
		Since:       since,
		Until:       until,
		Tool:        filter.Tool,
		Capability:  audit.Capability(filter.Capability),
		Status:      audit.Status(filter.Status),
		IncludeMeta: true,
	}, nil
}

// resolveReportSince computes the load-time since bound: since_offset
// (now - duration) when set, else the absolute since RFC3339, else zero
// (no bound). The 4a validator already caught bad values, so parse
// errors here would be unexpected.
func resolveReportSince(filter *config.ReportFilter, now time.Time) (time.Time, error) {
	if filter.SinceOffset != "" {
		dur, err := time.ParseDuration(filter.SinceOffset)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse since_offset %q: %w", filter.SinceOffset, err)
		}

		return now.Add(-dur), nil
	}

	if filter.Since != "" {
		parsed, err := time.Parse(time.RFC3339, filter.Since)
		if err != nil {
			return time.Time{}, fmt.Errorf("parse since %q: %w", filter.Since, err)
		}

		return parsed, nil
	}

	return time.Time{}, nil
}

// resolveReportUntil parses the absolute until bound, returning the
// zero time when absent.
func resolveReportUntil(filter *config.ReportFilter) (time.Time, error) {
	if filter.Until == "" {
		return time.Time{}, nil
	}

	parsed, err := time.Parse(time.RFC3339, filter.Until)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse until %q: %w", filter.Until, err)
	}

	return parsed, nil
}

// compileReportPredicate builds the per-event predicate for the filter
// fields RecentQuery does not carry: the _in lists for capability and
// status, plus the environment and profile globs. tool/since/until/
// scalar capability and status are handled at load time.
func compileReportPredicate(filter *config.ReportFilter) func(*audit.Event) bool {
	return func(event *audit.Event) bool {
		if len(filter.CapabilityIn) > 0 && !slices.Contains(filter.CapabilityIn, string(event.ToolCapability)) {
			return false
		}

		if len(filter.StatusIn) > 0 && !slices.Contains(filter.StatusIn, string(event.Status)) {
			return false
		}

		if filter.Environment != "" {
			if ok, _ := path.Match(filter.Environment, event.Environment); !ok {
				return false
			}
		}

		if filter.Profile != "" {
			if ok, _ := path.Match(filter.Profile, event.Profile); !ok {
				return false
			}
		}

		return true
	}
}
