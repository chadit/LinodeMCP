package tools_test

import (
	"encoding/json"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

	linodev1 "github.com/chadit/LinodeMCP/go/internal/genpb/linode/mcp/v1"
	"github.com/chadit/LinodeMCP/go/internal/tools"
)

// TestMarshalProtoJSONWidensInt64Fields verifies 64-bit integer fields emit
// as JSON numbers, including inside a nested message. A string emission would
// fail the typed decode below, because encoding/json refuses to unmarshal a
// quoted string into an int64.
func TestMarshalProtoJSONWidensInt64Fields(t *testing.T) {
	t.Parallel()

	msg := &linodev1.AuditHealthResponse{
		JsonlPath:       "/tmp/audit",
		ActiveLogExists: true,
		DiskBytes:       40960,
		DroppedEvents:   0,
		Sqlite: &linodev1.AuditHealthSQLite{
			Path:              "/tmp/audit.db",
			EventCount:        1200,
			OldestEventUnixNs: 1782734400000000000,
			DbBytes:           262144,
		},
	}

	data, err := tools.MarshalProtoJSON(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded struct {
		DiskBytes     int64 `json:"disk_bytes"`
		DroppedEvents int64 `json:"dropped_events"`
		Sqlite        struct {
			EventCount        int64 `json:"event_count"`
			OldestEventUnixNs int64 `json:"oldest_event_unix_ns"`
			DBBytes           int64 `json:"db_bytes"`
		} `json:"sqlite"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.DiskBytes != 40960 {
		t.Errorf("decoded.DiskBytes = %v, want %v", decoded.DiskBytes, 40960)
	}

	if decoded.DroppedEvents != 0 {
		t.Errorf("decoded.DroppedEvents = %v, want %v", decoded.DroppedEvents, 0)
	}

	if decoded.Sqlite.OldestEventUnixNs != 1782734400000000000 {
		t.Errorf("decoded.Sqlite.OldestEventUnixNs = %v, want %v", decoded.Sqlite.OldestEventUnixNs, int64(1782734400000000000))
	}

	if strings.Contains(string(data), `"40960"`) {
		t.Errorf("output still quotes disk_bytes: %s", data)
	}
}

// TestMarshalProtoJSONLeavesStructStringsAlone verifies the widening pass is
// descriptor-driven: a digit-only string inside a free-form Struct arg (here
// a redacted tool argument that happens to share a 64-bit field's name) stays
// a string while the typed 64-bit fields around it become numbers.
func TestMarshalProtoJSONLeavesStructStringsAlone(t *testing.T) {
	t.Parallel()

	msg := &linodev1.AuditEvent{
		Ts:                   "2026-06-29T12:00:00.5Z",
		TsUnixNs:             1782734400500000000,
		EventId:              "01JYyyyyyyyyyyyyyyyyyyyyyy",
		Tool:                 "linode_instance_list",
		LatencyMs:            250,
		CredentialGeneration: 2,
		Args: map[string]*structpb.Value{
			"disk_bytes": structpb.NewStringValue("40960"),
		},
	}

	data, err := tools.MarshalProtoJSON(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded struct {
		TSUnixNs             int64          `json:"ts_unix_ns"`
		LatencyMs            int64          `json:"latency_ms"`
		CredentialGeneration uint64         `json:"credential_generation"`
		Args                 map[string]any `json:"args"`
	}

	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.TSUnixNs != 1782734400500000000 {
		t.Errorf("decoded.TSUnixNs = %v, want %v", decoded.TSUnixNs, int64(1782734400500000000))
	}

	if decoded.LatencyMs != 250 {
		t.Errorf("decoded.LatencyMs = %v, want %v", decoded.LatencyMs, 250)
	}

	if decoded.CredentialGeneration != 2 {
		t.Errorf("decoded.CredentialGeneration = %v, want %v", decoded.CredentialGeneration, 2)
	}

	arg, ok := decoded.Args["disk_bytes"].(string)
	if !ok {
		t.Fatalf("args disk_bytes decoded as %T, want string", decoded.Args["disk_bytes"])
	}

	if arg != "40960" {
		t.Errorf("arg = %v, want %v", arg, "40960")
	}
}
