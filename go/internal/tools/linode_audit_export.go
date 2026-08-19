package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/config"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// AuditExportAnswer dumps a filtered window of audit events to a temp
// file in JSON, CSV, or NDJSON and answers with the path.
//
// It reads the SQLite store when that sink is enabled, falling back to the
// JSONL log otherwise, and is bounded by max_records so an unbounded range
// never reaches memory.
func AuditExportAnswer(
	ctx context.Context,
	request *mcp.CallToolRequest,
	cfg *config.Config,
) (*mcp.CallToolResult, error) {
	format := request.GetString("format", "")
	ext, _ := exportFileExtension(format)

	query, err := buildExportQuery(request)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	events, err := audit.ExportEvents(ctx, resolveAuditSQLitePath(cfg), audit.ResolveDefaultAuditDir(), query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to read audit log: %v", err)), nil
	}

	path, err := writeExportFile(events, format, ext)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to write export file: %v", err)), nil
	}

	return MarshalProtoToolResponse(&linodev1.AuditExportResponse{
		Path:        path,
		Format:      format,
		RecordCount: IDToInt32(len(events)),
	})
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
