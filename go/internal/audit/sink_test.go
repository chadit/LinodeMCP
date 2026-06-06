package audit_test

import (
	"testing"

	"github.com/chadit/LinodeMCP/internal/audit"
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
	checkEqual(t, "test_tool", evt.Tool, "noop sink must not mutate the event")
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

	mustEqual(t, 1, first.Len(), "first child must receive the event")
	mustEqual(t, 1, second.Len(), "second child must receive the event")
	checkEqual(t, "fanned_out", first.Events()[0].Tool)
	checkEqual(t, "fanned_out", second.Events()[0].Tool)
}

// TestMultiSinkEmptyIsNoop verifies a fan-out with no children does
// not panic.
func TestMultiSinkEmptyIsNoop(t *testing.T) {
	t.Parallel()

	multi := audit.NewMultiSink()
	evt := audit.Event{Tool: "nowhere"}

	mustNotPanics(t, func() { multi.Write(t.Context(), &evt) },
		"empty MultiSink must be a safe no-op")
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
	checkLen(t, events, 3)
	checkEqual(t, "first", events[0].Tool)
	checkEqual(t, "second", events[1].Tool)
	checkEqual(t, "third", events[2].Tool)
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
	mustLen(t, events, 1)
	checkEqual(t, "original", events[0].Tool,
		"sink must copy event, not retain caller's pointer")
}

// TestCapturingSinkLenReportsCount covers the size accessor.
func TestCapturingSinkLenReportsCount(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()
	checkEqual(t, 0, sink.Len(), "empty sink starts at zero")

	evt := audit.Event{Tool: "one"}
	sink.Write(t.Context(), &evt)
	checkEqual(t, 1, sink.Len())
}

// TestNewCapturingSinkStartsEmpty locks the non-nil-but-empty
// contract. Tests should be able to call Events() immediately
// without nil-check guards.
func TestNewCapturingSinkStartsEmpty(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()

	checkNotNil(t, sink.Events(), "Events() must return non-nil even when empty")
	checkEmpty(t, sink.Events())
}
