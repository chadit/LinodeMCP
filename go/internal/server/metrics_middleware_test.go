package server_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/server"
)

// recordedToolCall captures one RecordToolCall invocation for assertions.
type recordedToolCall struct {
	tool string
	err  error
}

// fakeMetricsRecorder satisfies server.MetricsRecorder and records what the
// dispatch chokepoint hands it. Guarded by a mutex so the race detector is
// satisfied even though HandleMessage dispatches synchronously.
type fakeMetricsRecorder struct {
	mu        sync.Mutex
	toolCalls []recordedToolCall
}

func (f *fakeMetricsRecorder) RecordToolCall(_ context.Context, toolName string, _ time.Duration, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.toolCalls = append(f.toolCalls, recordedToolCall{tool: toolName, err: err})
}

func (*fakeMetricsRecorder) RecordAPIRequest(context.Context, string, string, int, float64) {}

func (f *fakeMetricsRecorder) calls() []recordedToolCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Clone(f.toolCalls)
}

// TestMetricsRecorderWrappedAroundDispatch is the regression guard for the
// metrics-recording wiring: ToolExecution used to be dead code because the
// Server held no recorder. Dispatching a real tool call must now drive the
// injected recorder, otherwise the request metrics never move.
func TestMetricsRecorderWrappedAroundDispatch(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recorder := &fakeMetricsRecorder{}
	srv.SetMetricsRecorder(recorder)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	calls := recorder.calls()
	if len(calls) == 0 {
		t.Fatal("recorder captured no tool calls; dispatch is not wired to the metrics recorder")
	}

	helloCall := findToolCall(calls, "hello")
	if helloCall == nil {
		t.Fatalf("no RecordToolCall for hello; got %+v", calls)
	}

	if helloCall.err != nil {
		t.Errorf("hello RecordToolCall err = %v, want nil", helloCall.err)
	}
}

// TestSetMetricsRecorderNilRestoresNoop locks the documented contract:
// SetMetricsRecorder(nil) falls back to the no-op recorder rather than
// nil-derefing on the next dispatch.
func TestSetMetricsRecorderNilRestoresNoop(t *testing.T) {
	t.Parallel()

	srv, err := server.New(fullAccessConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recorder := &fakeMetricsRecorder{}
	srv.SetMetricsRecorder(recorder)
	srv.SetMetricsRecorder(nil)

	_ = srv.HandleMessage(t.Context(), []byte(helloCallMessage))

	if got := len(recorder.calls()); got != 0 {
		t.Errorf("recorder received %d calls after being replaced by nil; want 0", got)
	}
}

func findToolCall(calls []recordedToolCall, tool string) *recordedToolCall {
	for i := range calls {
		if calls[i].tool == tool {
			return &calls[i]
		}
	}

	return nil
}
