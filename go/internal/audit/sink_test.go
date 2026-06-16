package audit_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/go/internal/audit"
)

// TestNoopSinkSatisfiesInterface locks the type-level contract:
// NoopSink implements Sink, can be invoked through the interface,
// and accepts pointer arguments without observable side effect.
// Compile-time + runtime check together so the contract is
// exercised, not just declared.
func TestNoopSinkSatisfiesInterface(t *testing.T) {
	t.Parallel()

	var sink audit.Sink = audit.NoopSink{}

	evt := audit.Event{Tool: "test_tool"}
	sink.Write(t.Context(), &evt)
	sink.Write(t.Context(), &evt)

	// The contract is "no observable effect"; the only check we can
	// make is that the event we passed in is unchanged.
	if evt.Tool != "test_tool" {
		t.Errorf("evt.Tool = %v, want %v", evt.Tool, "test_tool")
	}
}

// TestMultiSinkFansOutToEveryChild verifies the fan-out delivers
// each event to all child sinks in order.
func TestMultiSinkFansOutToEveryChild(t *testing.T) {
	t.Parallel()

	first := audit.NewCapturingSink()
	second := audit.NewCapturingSink()
	multi := audit.NewMultiSink(first, second)

	evt := audit.Event{Tool: "fanned_out"}
	multi.Write(t.Context(), &evt)

	if first.Len() != 1 {
		t.Fatalf("first.Len() = %v, want %v", first.Len(), 1)
	}

	if second.Len() != 1 {
		t.Fatalf("second.Len() = %v, want %v", second.Len(), 1)
	}

	if first.Events()[0].Tool != tcFannedOut {
		t.Errorf("first.Events()[0].Tool = %v, want %v", first.Events()[0].Tool, tcFannedOut)
	}

	if second.Events()[0].Tool != tcFannedOut {
		t.Errorf("second.Events()[0].Tool = %v, want %v", second.Events()[0].Tool, tcFannedOut)
	}
}

// TestMultiSinkEmptyIsNoop verifies a fan-out with no children does
// not panic.
func TestMultiSinkEmptyIsNoop(t *testing.T) {
	t.Parallel()

	multi := audit.NewMultiSink()
	evt := audit.Event{Tool: "nowhere"}

	func() { multi.Write(t.Context(), &evt) }()
}

// TestCapturingSinkRetainsWriteOrder confirms the test-only sink
// preserves insertion order. Tests that assert "event for tool X
// was the third call" rely on this.
func TestCapturingSinkRetainsWriteOrder(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()

	first := audit.Event{Tool: "first"}
	second := audit.Event{Tool: "second"}
	third := audit.Event{Tool: "third"}

	sink.Write(t.Context(), &first)
	sink.Write(t.Context(), &second)
	sink.Write(t.Context(), &third)

	events := sink.Events()
	if len(events) != 3 {
		t.Errorf("len(events) = %d, want %d", len(events), 3)
	}

	if events[0].Tool != "first" {
		t.Errorf("events[0].Tool = %v, want %v", events[0].Tool, "first")
	}

	if events[1].Tool != "second" {
		t.Errorf("events[1].Tool = %v, want %v", events[1].Tool, "second")
	}

	if events[2].Tool != "third" {
		t.Errorf("events[2].Tool = %v, want %v", events[2].Tool, "third")
	}
}

// TestCapturingSinkCopiesEvent locks the copy-not-share contract.
// The capture middleware reuses the event variable across
// invocations; storing the pointer directly would let later
// mutation overwrite earlier captures.
func TestCapturingSinkCopiesEvent(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()
	evt := audit.Event{Tool: "original"}

	sink.Write(t.Context(), &evt)

	// Mutate the source event AFTER write. A copy-based sink keeps
	// the original; a share-based sink reflects the mutation.
	evt.Tool = "mutated"

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want %d", len(events), 1)
	}

	if events[0].Tool != "original" {
		t.Errorf("events[0].Tool = %v, want %v", events[0].Tool, "original")
	}
}

// TestCapturingSinkLenReportsCount covers the size accessor.
func TestCapturingSinkLenReportsCount(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()
	if sink.Len() != 0 {
		t.Errorf("sink.Len() = %v, want %v", sink.Len(), 0)
	}

	evt := audit.Event{Tool: "one"}
	sink.Write(t.Context(), &evt)

	if sink.Len() != 1 {
		t.Errorf("sink.Len() = %v, want %v", sink.Len(), 1)
	}
}

// TestNewCapturingSinkStartsEmpty locks the non-nil-but-empty
// contract. Tests should be able to call Events() immediately
// without nil-check guards.
func TestNewCapturingSinkStartsEmpty(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()

	if sink.Events() == nil {
		t.Error("sink.Events() is nil")
	}

	if len(sink.Events()) != 0 {
		t.Errorf("sink.Events() = %v, want empty", sink.Events())
	}
}
