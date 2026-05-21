package audit

import "context"

// Sink consumes audit events. Phase 1b ships only a NoopSink so the
// capture middleware has a stable target while Phase 2 implements
// the JSONL writer. Sinks are expected to be cheap and non-blocking;
// the capture middleware calls Write inside the tool handler's hot
// path.
//
// Write takes a context because the SQLite sink (Phase 3b) needs one
// for database/sql's ExecContext. The middleware passes a
// cancellation-detached context (context.WithoutCancel of the request
// context) so an audit write still lands after the request that
// produced it is canceled. Sinks that don't do context-aware I/O
// (NoopSink, JSONL file append) ignore the parameter.
//
// Event is passed by pointer because the struct is ~256 bytes;
// gocritic flags the value form for hot-path callers. Sinks MUST NOT
// mutate the event after Write returns; the caller may pass the same
// pointer to multiple sinks via the MultiSink fan-out.
type Sink interface {
	Write(ctx context.Context, event *Event)
}

// NoopSink discards every event. Used until Phase 2 lands the JSONL
// writer; tests also use it to exercise the capture middleware
// without exercising a real sink.
type NoopSink struct{}

// Write implements Sink by doing nothing. Both parameters are
// unnamed because a discard sink uses neither.
func (NoopSink) Write(context.Context, *Event) {}

// MultiSink fans an event out to every child sink in order. Used to
// dual-write to JSONL and SQLite when both are enabled. Each child's
// Write is best-effort per its own contract, so a failing child (e.g.
// a SQLite insert error) does not stop the others; the JSONL sink
// stays the durable record.
type MultiSink struct {
	sinks []Sink
}

// NewMultiSink returns a fan-out over the given sinks, written in the
// order provided.
func NewMultiSink(sinks ...Sink) *MultiSink {
	return &MultiSink{sinks: sinks}
}

// Write forwards the event to every child sink.
func (m *MultiSink) Write(ctx context.Context, event *Event) {
	for _, sink := range m.sinks {
		sink.Write(ctx, event)
	}
}

// CapturingSink retains every event for test inspection. NOT for
// production use: it accumulates without bound. The capture
// middleware tests rely on it to assert event-field population at
// the wire boundary.
type CapturingSink struct {
	events []Event
}

// NewCapturingSink returns an empty sink.
func NewCapturingSink() *CapturingSink {
	return &CapturingSink{events: make([]Event, 0)}
}

// Write copies the event into the internal buffer. A done context
// skips the capture, matching the cancellation-respecting contract
// the other sinks honor; tests pass live contexts so it captures
// normally. The copy is deliberate: the capture middleware reuses the
// event variable across invocations, so storing the pointer directly
// would let later mutation overwrite earlier captures.
func (s *CapturingSink) Write(ctx context.Context, event *Event) {
	if ctx.Err() != nil {
		return
	}

	s.events = append(s.events, *event)
}

// Events returns the captured event list. The slice is the sink's
// internal buffer; callers must not mutate it. Returning the live
// slice avoids a copy in the common test path (assertions read it
// once at the end of the test).
func (s *CapturingSink) Events() []Event {
	return s.events
}

// Len reports how many events have been captured. Tests that only
// care about a count use this without materializing the slice.
func (s *CapturingSink) Len() int {
	return len(s.events)
}
