package server_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	events := sink.Events()
	require.NotEmpty(t, events, "audit middleware must produce at least one event")

	helloEvent := findEventByTool(events, "hello")
	require.NotNil(t, helloEvent, "audit middleware must record an event for the hello call")

	assert.Equal(t, audit.CapabilityMeta, helloEvent.ToolCapability,
		"hello carries the CapMeta tag")
	assert.Equal(t, audit.StatusSuccess, helloEvent.Status,
		"hello returns a successful result")
	assert.GreaterOrEqual(t, helloEvent.LatencyMS, int64(0),
		"latency populates from Finalize")
	assert.Nil(t, helloEvent.Error, "successful call has nil Error pointer")
	assert.Equal(t, "Auditor", helloEvent.Args["name"],
		"args carry the request's arguments verbatim when not sensitive")
}

// TestSetAuditSinkNilRestoresNoop locks the documented contract for
// SetAuditSink(nil): the server doesn't nil-deref on the next tool
// call. Instead it falls back to NoopSink, which discards events
// silently.
func TestSetAuditSinkNilRestoresNoop(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	require.NoError(t, err)

	// Install a capturing sink, then clear it. After clearing, the
	// next tool call must not crash and must not feed the previous
	// sink.
	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)
	srv.SetAuditSink(nil)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	assert.Empty(t, sink.Events(),
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
