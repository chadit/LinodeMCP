package tools

import (
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// auditEventProto converts one audit event to its proto element. The timestamp
// keeps the RFC 3339 nanosecond form the legacy json.Marshal emitted for
// time.Time so stored and reported values stay comparable.
//
// The args map cannot fail to convert: it arrives from the JSONL sink through
// encoding/json, whose decoder produces only the kinds structpb represents. A
// map that somehow did fail reports no args rather than failing the whole read,
// since one unrepresentable value is not a reason to answer nothing.
func auditEventProto(event *audit.Event) *linodev1.AuditEvent {
	args, _ := structpb.NewStruct(event.Args)

	return &linodev1.AuditEvent{
		Ts:                   event.TS.Format(time.RFC3339Nano),
		TsUnixNs:             event.TSUnixNS,
		EventId:              event.EventID,
		Tool:                 event.Tool,
		ToolCapability:       string(event.ToolCapability),
		Environment:          event.Environment,
		Profile:              event.Profile,
		Mode:                 string(event.Mode),
		PlanId:               event.PlanID,
		Args:                 args.GetFields(),
		ArgsRedacted:         event.ArgsRedacted,
		Status:               string(event.Status),
		LatencyMs:            event.LatencyMS,
		ResultSummary:        event.ResultSummary,
		Error:                event.Error,
		LinodemcpVersion:     event.LinodemcpVersion,
		SessionId:            event.SessionID,
		CredentialGeneration: event.CredentialGeneration,
	}
}

// auditEventsProto converts a slice of audit events to proto elements,
// preserving order.
func auditEventsProto(events []audit.Event) []*linodev1.AuditEvent {
	out := make([]*linodev1.AuditEvent, 0, len(events))

	for idx := range events {
		out = append(out, auditEventProto(&events[idx]))
	}

	return out
}

// auditSummaryRowsProto converts summary rows to proto elements, preserving
// order.
func auditSummaryRowsProto(rows []audit.SummaryRow) []*linodev1.AuditSummaryRow {
	out := make([]*linodev1.AuditSummaryRow, 0, len(rows))

	for idx := range rows {
		out = append(out, &linodev1.AuditSummaryRow{
			Groups: rows[idx].Groups,
			Count:  IDToInt32(rows[idx].Count),
		})
	}

	return out
}

// auditHealthProto converts the collected health report to its proto body.
// The SQLite section stays unset when the sink is disabled, so the canonical
// output omits it rather than emitting null.
func auditHealthProto(report *audit.HealthReport) *linodev1.AuditHealthResponse {
	out := &linodev1.AuditHealthResponse{
		JsonlPath:         report.JSONLPath,
		ActiveLogExists:   report.ActiveLogExists,
		RotatedFileCount:  IDToInt32(report.RotatedFileCount),
		OldestRotatedDate: report.OldestRotatedDate,
		DiskBytes:         report.DiskBytes,
		DroppedEvents:     report.DroppedEvents,
	}

	if report.SQLite != nil {
		out.Sqlite = &linodev1.AuditHealthSQLite{
			Path:              report.SQLite.Path,
			EventCount:        report.SQLite.EventCount,
			OldestEventUnixNs: report.SQLite.OldestEventUnixNS,
			DbBytes:           report.SQLite.DBBytes,
		}
	}

	return out
}
