package tools

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
)

// auditEventProto converts one audit event to its proto element. The args map
// round-trips through structpb (it arrives JSON-native from the JSONL sink);
// the timestamp keeps the RFC 3339 nanosecond form the legacy json.Marshal
// emitted for time.Time so stored and reported values stay comparable.
func auditEventProto(event *audit.Event) (*linodev1.AuditEvent, error) {
	args, err := structpb.NewStruct(event.Args)
	if err != nil {
		return nil, fmt.Errorf("convert audit event args for %s: %w", event.EventID, err)
	}

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
	}, nil
}

// auditEventsProto converts a slice of audit events to proto elements,
// preserving order.
func auditEventsProto(events []audit.Event) ([]*linodev1.AuditEvent, error) {
	out := make([]*linodev1.AuditEvent, 0, len(events))

	for idx := range events {
		converted, err := auditEventProto(&events[idx])
		if err != nil {
			return nil, err
		}

		out = append(out, converted)
	}

	return out, nil
}

// auditSummaryRowsProto converts summary rows to proto elements, preserving
// order.
func auditSummaryRowsProto(rows []audit.SummaryRow) []*linodev1.AuditSummaryRow {
	out := make([]*linodev1.AuditSummaryRow, 0, len(rows))

	for idx := range rows {
		out = append(out, &linodev1.AuditSummaryRow{
			Groups: rows[idx].Groups,
			Count:  linodeIDToInt32(rows[idx].Count),
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
		RotatedFileCount:  linodeIDToInt32(report.RotatedFileCount),
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
