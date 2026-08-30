package observability_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// TestTracingWithAnUnparsableEndpointFallsBackToNoop covers the exporter
// construction failure a typo in the collector address produces. The gRPC
// client refuses a target it cannot parse, and that must degrade to tracing
// off: New still succeeds, spans do not record, and Shutdown has nothing to
// flush toward a collector that was never configured.
func TestTracingWithAnUnparsableEndpointFallsBackToNoop(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Tracing: config.TracingConfig{
			Enabled:    true,
			Endpoint:   "collector:%zz",
			Protocol:   "grpc",
			SampleRate: 1.0,
		},
	})
	if err != nil {
		t.Fatalf("New with an unparsable tracing endpoint returned %v, want the instance without tracing", err)
	}

	_, span := obs.Tracer().Start(t.Context(), "probe")
	defer span.End()

	if span.IsRecording() {
		t.Error("span is recording after the exporter failed to build, want a noop tracer")
	}

	if err := obs.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown after a failed exporter build returned %v, want nil", err)
	}
}

// TestTracingWithAZeroSampleRateRecordsNothing pins the sampler wiring: a
// configured endpoint alone does not make spans record, the sample rate
// decides. Zero is the one ratio with a fixed answer, so it is the row that
// proves the ratio sampler is in place rather than always-on sampling.
func TestTracingWithAZeroSampleRateRecordsNothing(t *testing.T) {
	t.Parallel()

	// A server the test owns stands in for the collector, so the exporter
	// never talks to a port on the host that something else may be serving.
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(collector.Close)

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Tracing: config.TracingConfig{
			Enabled:    true,
			Endpoint:   strings.TrimPrefix(collector.URL, "http://"),
			Protocol:   "http",
			Insecure:   true,
			SampleRate: 0,
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		// The test context is already canceled by cleanup time, and a live
		// provider's Shutdown reports that cancellation as its own failure.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()

		if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	_, span := obs.Tracer().Start(t.Context(), "probe")
	defer span.End()

	if span.IsRecording() {
		t.Error("span is recording at sample rate 0, want the ratio sampler to drop it")
	}
}
