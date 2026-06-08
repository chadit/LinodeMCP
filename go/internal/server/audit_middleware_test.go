package server_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/chadit/LinodeMCP/internal/audit"
	"github.com/chadit/LinodeMCP/internal/server"
)

const helloCallMessage = `{
	"jsonrpc": "2.0",
	"id": 1,
	"method": "tools/call",
	"params": {"name": "hello", "arguments": {"name": "Auditor"}}
}`

// TestAuditMiddlewareWritesEventOnSuccess is the substantive coverage
// for Phase 1b: dispatching a real handler call through the server's
// MCP entry produces one audit event with every key field populated.
// The default profile permits `hello`; the call returns success; the
// CapturingSink stores the event.
func TestAuditMiddlewareWritesEventOnSuccess(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	events := sink.Events()
	if len(events) == 0 {
		t.Fatal("events is empty")
	}

	helloEvent := findEventByTool(events, "hello")
	if helloEvent == nil {
		t.Fatal("helloEvent is nil")
	}

	if helloEvent.ToolCapability != audit.CapabilityMeta {
		t.Errorf("helloEvent.ToolCapability = %v, want %v", helloEvent.ToolCapability, audit.CapabilityMeta)
	}

	if helloEvent.Status != audit.StatusSuccess {
		t.Errorf("helloEvent.Status = %v, want %v", helloEvent.Status, audit.StatusSuccess)
	}

	if helloEvent.LatencyMS < int64(0) {
		t.Errorf("got %v, want >= %v", helloEvent.LatencyMS, int64(0))
	}

	if helloEvent.Error != nil {
		t.Errorf("helloEvent.Error = %v, want nil", helloEvent.Error)
	}

	if !reflect.DeepEqual(helloEvent.Args["name"], "Auditor") {
		t.Errorf("got %v, want %v", helloEvent.Args["name"], "Auditor")
	}
}

// TestSetAuditSinkNilRestoresNoop locks the documented contract for
// SetAuditSink(nil): the server doesn't nil-deref on the next tool
// call. Instead it falls back to NoopSink, which discards events
// silently.
func TestSetAuditSinkNilRestoresNoop(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Install a capturing sink, then clear it. After clearing, the
	// next tool call must not crash and must not feed the previous
	// sink.
	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	srv.SetAuditSink(nil)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	if len(sink.Events()) != 0 {
		t.Errorf("sink.Events() = %v, want empty", sink.Events())
	}
}

// findEventByTool walks the captured events and returns a pointer to
// the first one whose Tool field matches. Returns nil for misses so
// the assertion at the call site can use NotNil for the not-found
// path. Pointer return avoids the gocritic hugeParam complaint that
// returning an Event value would trigger.
func findEventByTool(events []audit.Event, tool string) *audit.Event {
	for i := range events {
		if events[i].Tool == tool {
			return &events[i]
		}
	}

	return nil
}

// helloCallWithPIIArgs sends a hello call with extra PII-named args.
// The hello tool ignores unknown args, but the audit middleware walks
// the raw arguments map and redacts by name regardless of which tool
// owned them; that's how the PII tier exercises here without needing
// a real PII-accepting tool.
const helloCallWithPIIArgs = `{
	"jsonrpc": "2.0",
	"id": 2,
	"method": "tools/call",
	"params": {
		"name": "hello",
		"arguments": {
			"name": "Auditor",
			"phone": "+1-555-0100",
			"tax_id": "TX-99",
			"address_1": "123 Main St",
			"city": "Springfield",
			"country": "us",
			"token": "shh-credential"
		}
	}
}`

// TestAuditMiddlewareRedactsPIIWhenFlagOn confirms the Phase 4c wiring:
// when set_audit_redact_pii(true) is in effect, the captured event's
// args carry [REDACTED] for PII names while non-PII passes through
// and credentials stay scrubbed.
func TestAuditMiddlewareRedactsPIIWhenFlagOn(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	srv.SetAuditRedactPII(true)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallWithPIIArgs))

	events := sink.Events()

	helloEvent := findEventByTool(events, "hello")
	if helloEvent == nil {
		t.Fatal("helloEvent is nil")
	}

	if !audit.IsRedacted(helloEvent.Args["phone"]) {
		t.Error("expected condition to be true")
	}

	if !audit.IsRedacted(helloEvent.Args["tax_id"]) {
		t.Error("expected condition to be true")
	}

	if !audit.IsRedacted(helloEvent.Args["address_1"]) {
		t.Error("expected condition to be true")
	}

	if !audit.IsRedacted(helloEvent.Args["city"]) {
		t.Error("expected condition to be true")
	}

	if !audit.IsRedacted(helloEvent.Args["token"]) {
		t.Error("expected condition to be true")
	}

	if !reflect.DeepEqual(helloEvent.Args["country"], "us") {
		t.Errorf("got %v, want %v", helloEvent.Args["country"], "us")
	}

	if !reflect.DeepEqual(helloEvent.Args["name"], "Auditor") {
		t.Errorf("got %v, want %v", helloEvent.Args["name"], "Auditor")
	}

	if !slices.Contains(helloEvent.ArgsRedacted, "phone") {
		t.Errorf("helloEvent.ArgsRedacted does not contain %v", "phone")
	}

	if !slices.Contains(helloEvent.ArgsRedacted, "token") {
		t.Errorf("helloEvent.ArgsRedacted does not contain %v", "token")
	}
}

// TestAuditMiddlewareLeavesPIIWhenFlagOff is the inverse: with the
// operator's opt-out (set_audit_redact_pii(false), the default for
// tests), PII passes through in cleartext while credentials stay
// scrubbed.
func TestAuditMiddlewareLeavesPIIWhenFlagOff(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	// Explicitly opt out; this is also the Server default for tests.
	srv.SetAuditRedactPII(false)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallWithPIIArgs))

	events := sink.Events()

	helloEvent := findEventByTool(events, "hello")
	if helloEvent == nil {
		t.Fatal("helloEvent is nil")
	}

	if !reflect.DeepEqual(helloEvent.Args["phone"], "+1-555-0100") {
		t.Errorf("got %v, want %v", helloEvent.Args["phone"], "+1-555-0100")
	}

	if !reflect.DeepEqual(helloEvent.Args["tax_id"], "TX-99") {
		t.Errorf("got %v, want %v", helloEvent.Args["tax_id"], "TX-99")
	}

	if !reflect.DeepEqual(helloEvent.Args["address_1"], "123 Main St") {
		t.Errorf("got %v, want %v", helloEvent.Args["address_1"], "123 Main St")
	}

	if !reflect.DeepEqual(helloEvent.Args["city"], "Springfield") {
		t.Errorf("got %v, want %v", helloEvent.Args["city"], "Springfield")
	}

	if !audit.IsRedacted(helloEvent.Args["token"]) {
		t.Error("expected condition to be true")
	}

	if slices.Contains(helloEvent.ArgsRedacted, "phone") {
		t.Errorf("helloEvent.ArgsRedacted should not contain %v", "phone")
	}

	if !slices.Contains(helloEvent.ArgsRedacted, "token") {
		t.Errorf("helloEvent.ArgsRedacted does not contain %v", "token")
	}
}
