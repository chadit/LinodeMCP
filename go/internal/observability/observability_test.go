package observability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/chadit/LinodeMCP/internal/config"
	"github.com/chadit/LinodeMCP/internal/observability"
)

func TestInitWithDisabledComponents(t *testing.T) {
	t.Parallel()

	cfg := config.ObservabilityConfig{
		Tracing: config.TracingConfig{
			Enabled: false,
		},
		Metrics: config.MetricsConfig{
			Enabled: false,
		},
		Health: config.HealthConfig{
			Enabled: false,
		},
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	err := observability.Init(&cfg)
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
}

func TestInitNilConfig(t *testing.T) {
	t.Parallel()

	// Init with nil config should use defaults
	err := observability.Init(nil)
	if err != nil {
		t.Fatalf("Init(nil) failed: %v", err)
	}
}

func TestLogger(t *testing.T) {
	t.Parallel()

	// Logger should always return a valid logger
	logger := observability.Logger()
	if logger == nil {
		t.Error("Logger() should never return nil")
	}

	// Log something to verify it works
	logger.Info("test log message")
}

func TestTracer(t *testing.T) {
	t.Parallel()

	// Tracer should always return a valid tracer
	tracer := observability.Tracer()
	if tracer == nil {
		t.Error("Tracer() should never return nil")
	}
}

func TestRecordRequest(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Should not panic when metrics not initialized
	observability.RecordRequest(ctx, "test_tool", "execute", "success", 0.1)
	observability.RecordRequest(ctx, "test_tool", "execute", "error", 0.2)
}

func TestRecordError(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Should not panic when metrics not initialized
	observability.RecordError(ctx, "test_tool", "test_error")
}

func TestRecordAPIRequest(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Should not panic when metrics not initialized
	observability.RecordAPIRequest(ctx, "/v4/linode/instances", "GET", 200, 0.05)
}

func TestToolExecution(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	err := observability.ToolExecution(ctx, "test_tool", func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("ToolExecution() with successful function should return nil: %v", err)
	}

	testErr := errors.New("test error")

	err = observability.ToolExecution(ctx, "test_tool", func(_ context.Context) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("ToolExecution() should return original error")
	}
}

func TestAPICall(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	err := observability.APICall(ctx, "/v4/linode/instances", "GET", func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("APICall() with successful function should return nil: %v", err)
	}

	testErr := errors.New("test error")

	err = observability.APICall(ctx, "/v4/linode/instances", "GET", func(_ context.Context) error {
		return testErr
	})
	if !errors.Is(err, testErr) {
		t.Errorf("APICall() should return original error")
	}
}

func TestWithEnvironment(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	newCtx := observability.WithEnvironment(ctx, "test-env")
	if newCtx == nil {
		t.Error("WithEnvironment() should return valid context")
	}
}

func TestWithToolArgument(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	newCtx := observability.WithToolArgument(ctx, "test_arg", "test_value")
	if newCtx == nil {
		t.Error("WithToolArgument() should return valid context")
	}
}

func TestWithToolResultSize(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	newCtx := observability.WithToolResultSize(ctx, 1024)
	if newCtx == nil {
		t.Error("WithToolResultSize() should return valid context")
	}
}

func TestRecordEvent(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	// Should not panic
	observability.RecordEvent(ctx, "test_event")
}

func TestShutdown(t *testing.T) {
	t.Parallel()

	// Shutdown with background context should succeed
	ctx := t.Context()
	err := observability.Shutdown(ctx)
	// May return nil or context errors depending on state
	_ = err
}

func TestShutdownWithTimeout(t *testing.T) {
	t.Parallel()

	// Shutdown with timeout should handle gracefully
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	err := observability.Shutdown(ctx)
	// May return nil or context errors depending on state
	_ = err
}
