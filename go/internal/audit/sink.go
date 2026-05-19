package audit

// Sink consumes audit events. Phase 1b ships only a NoopSink so the
// capture middleware has a stable target while Phase 2 implements
// the JSONL writer. Sinks are expected to be cheap and non-blocking;
// the capture middleware calls Write inside the tool handler's hot
// path.
//
// A sink that needs heavy work (file IO, OTel export, etc.) should
// buffer to a goroutine and fan out asynchronously. The Phase 2
// JSONL sink uses that pattern; the per-event channel send is the
// only synchronous cost paid by the handler.
//
// Event is passed by pointer because the struct is ~256 bytes;
// gocritic flags the value form for hot-path callers. Sinks MUST NOT
// mutate the event after Write returns; the caller may pass the same
// pointer to multiple sinks via a fan-out Sink wrapper.
type Sink interface {
	Write(event *Event)
}

// NoopSink discards every event. Used until Phase 2 lands the JSONL
// writer; tests also use it to exercise the capture middleware
// without exercising a real sink.
type NoopSink struct{}

// Write implements Sink by doing nothing.
func (NoopSink) Write(_ *Event) {}

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

// Write copies the event into the internal buffer. The copy is
// deliberate: the capture middleware reuses the event variable
// across invocations, so storing the pointer directly would let
// later mutation overwrite earlier captures.
func (s *CapturingSink) Write(event *Event) {
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
