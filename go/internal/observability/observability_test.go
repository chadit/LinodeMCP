package observability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// newTestObservability constructs a fresh instance with all subsystems off
// (so tests don't open ports or talk to OTLP collectors).
func newTestObservability(t *testing.T) *observability.Observability {
	t.Helper()

	obs, err := observability.New(&config.ObservabilityConfig{
		Tracing: config.TracingConfig{Enabled: false},
		Metrics: config.MetricsConfig{Enabled: false},
		Health:  config.HealthConfig{Enabled: false},
		Logging: config.LoggingConfig{Level: "info", Format: "json"},
	})
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	t.Cleanup(func() {
		_ = obs.Shutdown(t.Context())
	})

	return obs
}

func TestNewWithDisabledComponents(t *testing.T) {
	t.Parallel()
	_ = newTestObservability(t)
}

func TestNewNilConfig(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(nil)
	if err != nil {
		t.Fatalf("New(nil) failed: %v", err)
	}

	t.Cleanup(func() { _ = obs.Shutdown(t.Context()) })
}

func TestLogger(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	logger := obs.Logger()

	if logger == nil {
		t.Fatal("Logger() should never return nil")
	}

	logger.Info("test log message")
}

func TestTracer(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	if obs.Tracer() == nil {
		t.Error("Tracer() should never return nil")
	}
}

func TestRecordRequest(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	ctx := t.Context()

	// Counters are nil when metrics disabled; calls must not panic.
	obs.RecordRequest(ctx, "test_tool", "execute", "success", 0.1)
	obs.RecordRequest(ctx, "test_tool", "execute", "error", 0.2)
}

func TestRecordError(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	obs.RecordError(t.Context(), "test_tool", "test_error")
}

func TestRecordAPIRequest(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	obs.RecordAPIRequest(t.Context(), "/v4/linode/instances", "GET", 200, 0.05)
}

func TestToolExecution(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	ctx := t.Context()

	if err := obs.ToolExecution(ctx, "test_tool", func(_ context.Context) error {
		return nil
	}); err != nil {
		t.Errorf("ToolExecution() with successful function should return nil: %v", err)
	}

	testErr := errors.New("test error")

	err := obs.ToolExecution(ctx, "test_tool", func(_ context.Context) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("ToolExecution() should return original error, got %v", err)
	}
}

func TestAPICall(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)
	ctx := t.Context()

	if err := obs.APICall(ctx, "/v4/linode/instances", "GET", func(_ context.Context) error {
		return nil
	}); err != nil {
		t.Errorf("APICall() with successful function should return nil: %v", err)
	}

	testErr := errors.New("test error")

	err := obs.APICall(ctx, "/v4/linode/instances", "GET", func(_ context.Context) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("APICall() should return original error, got %v", err)
	}
}

func TestWithEnvironment(t *testing.T) {
	t.Parallel()

	if observability.WithEnvironment(t.Context(), "test-env") == nil {
		t.Error("WithEnvironment() should return valid context")
	}
}

func TestWithToolArgument(t *testing.T) {
	t.Parallel()

	if observability.WithToolArgument(t.Context(), "test_arg", "test_value") == nil {
		t.Error("WithToolArgument() should return valid context")
	}
}

func TestWithToolResultSize(t *testing.T) {
	t.Parallel()

	if observability.WithToolResultSize(t.Context(), 1024) == nil {
		t.Error("WithToolResultSize() should return valid context")
	}
}

func TestRecordEvent(t *testing.T) {
	t.Parallel()

	observability.RecordEvent(t.Context(), "test_event")
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(nil)
	if err != nil {
		t.Fatalf("New(nil) failed: %v", err)
	}

	if err := obs.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() should succeed, got %v", err)
	}

	// Second call is a no-op.
	if err := obs.Shutdown(t.Context()); err != nil {
		t.Errorf("Shutdown() second call should be a no-op, got %v", err)
	}
}

func TestShutdownWithTimeout(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(nil)
	if err != nil {
		t.Fatalf("New(nil) failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	_ = obs.Shutdown(ctx)
}
