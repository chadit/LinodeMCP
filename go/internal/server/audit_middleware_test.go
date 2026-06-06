package server_test

import (
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
	requireNoError(t, err)

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	events := sink.Events()
	requireNotEmpty(t, events, "audit middleware must produce at least one event")

	helloEvent := findEventByTool(events, "hello")
	requireNotNil(t, helloEvent, "audit middleware must record an event for the hello call")

	assertEqual(t, audit.CapabilityMeta, helloEvent.ToolCapability,
		"hello carries the CapMeta tag")
	assertEqual(t, audit.StatusSuccess, helloEvent.Status,
		"hello returns a successful result")
	assertGreaterOrEqual(t, helloEvent.LatencyMS, int64(0),
		"latency populates from Finalize")
	assertNil(t, helloEvent.Error, "successful call has nil Error pointer")
	assertEqual(t, "Auditor", helloEvent.Args["name"],
		"args carry the request's arguments verbatim when not sensitive")
}

// TestSetAuditSinkNilRestoresNoop locks the documented contract for
// SetAuditSink(nil): the server doesn't nil-deref on the next tool
// call. Instead it falls back to NoopSink, which discards events
// silently.
func TestSetAuditSinkNilRestoresNoop(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	requireNoError(t, err)

	// Install a capturing sink, then clear it. After clearing, the
	// next tool call must not crash and must not feed the previous
	// sink.
	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	srv.SetAuditSink(nil)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	assertEmpty(t, sink.Events(),
		"previously-installed sink must not receive events after SetAuditSink(nil)")
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
	requireNoError(t, err)

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	srv.SetAuditRedactPII(true)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallWithPIIArgs))

	events := sink.Events()
	helloEvent := findEventByTool(events, "hello")
	requireNotNil(t, helloEvent)

	assertTrue(t, audit.IsRedacted(helloEvent.Args["phone"]),
		"phone is PII and must be redacted when redact_pii=true")
	assertTrue(t, audit.IsRedacted(helloEvent.Args["tax_id"]))
	assertTrue(t, audit.IsRedacted(helloEvent.Args["address_1"]))
	assertTrue(t, audit.IsRedacted(helloEvent.Args["city"]))
	assertTrue(t, audit.IsRedacted(helloEvent.Args["token"]),
		"credential is always redacted regardless of redact_pii")
	assertEqual(t, "us", helloEvent.Args["country"],
		"country is a non-sensitive filter, must pass through")
	assertEqual(t, "Auditor", helloEvent.Args["name"],
		"non-PII non-credential args pass through")
	assertContains(t, helloEvent.ArgsRedacted, "phone")
	assertContains(t, helloEvent.ArgsRedacted, "token")
}

// TestAuditMiddlewareLeavesPIIWhenFlagOff is the inverse: with the
// operator's opt-out (set_audit_redact_pii(false), the default for
// tests), PII passes through in cleartext while credentials stay
// scrubbed.
func TestAuditMiddlewareLeavesPIIWhenFlagOff(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	requireNoError(t, err)

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	// Explicitly opt out; this is also the Server default for tests.
	srv.SetAuditRedactPII(false)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallWithPIIArgs))

	events := sink.Events()
	helloEvent := findEventByTool(events, "hello")
	requireNotNil(t, helloEvent)

	assertEqual(t, "+1-555-0100", helloEvent.Args["phone"],
		"PII passes through when redact_pii=false")
	assertEqual(t, "TX-99", helloEvent.Args["tax_id"])
	assertEqual(t, "123 Main St", helloEvent.Args["address_1"])
	assertEqual(t, "Springfield", helloEvent.Args["city"])
	assertTrue(t, audit.IsRedacted(helloEvent.Args["token"]),
		"credential is redacted even with redact_pii=false")
	assertNotContains(t, helloEvent.ArgsRedacted, "phone",
		"PII names absent from ArgsRedacted when flag is off")
	assertContains(t, helloEvent.ArgsRedacted, "token")
}
