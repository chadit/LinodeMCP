package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/audit"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// The outcome the capture middleware records without running a handler.
//
// A refusal is written by the shutdown gate, and nothing here reached it before:
// the success path's own assertions would look the same if a rebuild dropped it.
// The other two outcomes are read where the status is chosen, in
// audit.Outcome, because no shipped handler answers an error at all: every
// route and every meta tool answers an error RESULT, so the middleware's error
// arm is the honest handling of a return its signature declares rather than one
// a call can produce today.

// listToolsCallMessage is a second meta call, sent at the gate so the refusal
// case names a tool of its own rather than the one the success cases use.
const listToolsCallMessage = `{
	"jsonrpc": "2.0",
	"id": 1,
	"method": "tools/call",
	"params": {"name": "linode_profile_list_tools", "arguments": {}}
}`

// TestAuditMiddlewareRecordsARefusalAfterShutdown covers the gate: a call that
// arrives once the server is draining is refused, and the refusal is audited
// with no latency because nothing ran.
func TestAuditMiddlewareRecordsARefusalAfterShutdown(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sink := audit.NewCapturingSink()
	srv.SetAuditSink(sink)

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	if shutdownErr := srv.Shutdown(ctx); shutdownErr != nil {
		t.Fatalf("unexpected error: %v", shutdownErr)
	}

	_ = srv.HandleMessage(t.Context(), []byte(listToolsCallMessage))

	refused := findEventByTool(sink.Events(), "linode_profile_list_tools")
	if refused == nil {
		t.Fatal("refused is nil")
	}

	if refused.Status != string(audit.StatusRefused) {
		t.Errorf("refused.Status = %v, want %v", refused.Status, audit.StatusRefused)
	}

	if refused.Error == nil {
		t.Fatal("refused.Error is nil")
	}

	if refused.LatencyMs != 0 {
		t.Errorf("refused.LatencyMs = %v, want zero, since the handler never ran", refused.LatencyMs)
	}
}
