package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/profiles"
)

// auditExportResponse is the wire shape of the linode_audit_export
// result: the temp-file path the model surfaces to the user, the
// format written, and the number of records exported.
type auditExportResponse struct {
	Path        string `json:"path"`
	Format      string `json:"format"`
	RecordCount int    `json:"record_count"`
}

// NewLinodeAuditExportTool returns the linode_audit_export query tool.
// It dumps a filtered window of audit events to a temp file in JSON,
// CSV, or NDJSON and returns the path. CapMeta so it is available in
// every profile.
//
// Reads the SQLite store when the SQLite sink is enabled, falling back
// to the JSONL log otherwise. Bounded by max_records to avoid pulling
// an unbounded range into memory.
func NewLinodeAuditExportTool(
	cfg *config.Config,
) (mcp.Tool, profiles.Capability, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)) {
	tool := mcp.NewTool(
		"linode_audit_export",
		mcp.WithDescription(
			"Export a range of audit events to a temp file and return its path. "+
				"Reads SQLite when enabled, else the JSONL log. Optional filters: "+
				"since, until, tool (glob), max_records, include_meta.",
		),
		mcp.WithString(
			"format",
			mcp.Required(),
			mcp.Description("Output format: json, csv, or ndjson."),
		),
		mcp.WithString(
			"since",
			mcp.Description("Only events at or after this RFC 3339 timestamp."),
		),
		mcp.WithString(
			"until",
			mcp.Description("Only events at or before this RFC 3339 timestamp."),
		),
		mcp.WithString(
			"tool",
			mcp.Description(`Only events whose tool name matches this glob (e.g. "linode_instance_*").`),
		),
		mcp.WithNumber(
			"max_records",
			mcp.Description("Max events to export. Default 10000, capped at 100000."),
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

		format := request.GetString("format", "")

		ext, ok := exportFileExtension(format)
		if !ok {
			return mcp.NewToolResultError(fmt.Sprintf("unknown format %q: expected json, csv, or ndjson", format)), nil
		}

		query, err := buildExportQuery(&request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		events, err := audit.ExportEvents(ctx, sqlitePath, audit.ResolveDefaultAuditDir(), query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
		}

		path, err := writeExportFile(events, format, ext)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to write export file: %v", err)), nil
		}

		body, err := json.Marshal(auditExportResponse{Path: path, Format: format, RecordCount: len(events)})
		if err != nil {
			return nil, fmt.Errorf("marshal audit export response: %w", err)
		}

		return mcp.NewToolResultText(string(body)), nil
	}

	return tool, profiles.CapMeta, handler
}

// buildExportQuery translates request parameters into a RecentQuery
// whose Limit carries the resolved max_records cap. Returns an error
// for a malformed since/until timestamp.
func buildExportQuery(request *mcp.CallToolRequest) (*audit.RecentQuery, error) {
	query := &audit.RecentQuery{
		Limit:       resolveMaxRecords(request.GetInt("max_records", 0)),
		Tool:        request.GetString("tool", ""),
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

// resolveMaxRecords applies the default and hard cap to a requested
// max_records value. Zero or negative means "use the default".
func resolveMaxRecords(requested int) int {
	if requested <= 0 {
		return audit.DefaultExportMaxRecords
	}

	if requested > audit.MaxExportRecords {
		return audit.MaxExportRecords
	}

	return requested
}

// exportFileExtension maps a format name to its file extension,
// reporting false for an unknown format.
func exportFileExtension(format string) (string, bool) {
	switch format {
	case audit.ExportFormatJSON:
		return "json", true
	case audit.ExportFormatCSV:
		return "csv", true
	case audit.ExportFormatNDJSON:
		return "ndjson", true
	default:
		return "", false
	}
}

// writeExportFile creates a temp file with the format's extension,
// encodes events into it, and returns the path. The file is left in
// place for the user to read; the OS reclaims the temp directory on
// its own schedule.
func writeExportFile(events []audit.Event, format, ext string) (string, error) {
	file, err := os.CreateTemp("", "linode-audit-export-*."+ext)
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}

	if err := audit.EncodeEvents(file, events, format); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())

		return "", fmt.Errorf("encode export: %w", err)
	}

	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close export file: %w", err)
	}

	return file.Name(), nil
}
