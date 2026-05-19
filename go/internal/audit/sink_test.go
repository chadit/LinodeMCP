package audit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	sink.Write(&evt)
	sink.Write(&evt)

	// The contract is "no observable effect"; the only check we can
	// make is that the event we passed in is unchanged.
	assert.Equal(t, "test_tool", evt.Tool, "noop sink must not mutate the event")
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

	sink.Write(&first)
	sink.Write(&second)
	sink.Write(&third)

	events := sink.Events()
	assert.Len(t, events, 3)
	assert.Equal(t, "first", events[0].Tool)
	assert.Equal(t, "second", events[1].Tool)
	assert.Equal(t, "third", events[2].Tool)
}

// TestCapturingSinkCopiesEvent locks the copy-not-share contract.
// The capture middleware reuses the event variable across
// invocations; storing the pointer directly would let later
// mutation overwrite earlier captures.
func TestCapturingSinkCopiesEvent(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()
	evt := audit.Event{Tool: "original"}

	sink.Write(&evt)

	// Mutate the source event AFTER write. A copy-based sink keeps
	// the original; a share-based sink reflects the mutation.
	evt.Tool = "mutated"

	events := sink.Events()
	require.Len(t, events, 1)
	assert.Equal(t, "original", events[0].Tool,
		"sink must copy event, not retain caller's pointer")
}

// TestCapturingSinkLenReportsCount covers the size accessor.
func TestCapturingSinkLenReportsCount(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()
	assert.Equal(t, 0, sink.Len(), "empty sink starts at zero")

	evt := audit.Event{Tool: "one"}
	sink.Write(&evt)
	assert.Equal(t, 1, sink.Len())
}

// TestNewCapturingSinkStartsEmpty locks the non-nil-but-empty
// contract. Tests should be able to call Events() immediately
// without nil-check guards.
func TestNewCapturingSinkStartsEmpty(t *testing.T) {
	t.Parallel()

	sink := audit.NewCapturingSink()

	assert.NotNil(t, sink.Events(), "Events() must return non-nil even when empty")
	assert.Empty(t, sink.Events())
}
