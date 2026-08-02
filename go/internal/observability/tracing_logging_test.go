package observability_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/observability"
)

// errToolFailed stands in for a tool execution failure so the recording path
// has a non-nil error to classify.
var errToolFailed = errors.New("tool execution failed")

// TestSpanAttributeConstructors pins the wire names every exported attribute
// helper emits. Renaming one silently breaks dashboards and trace queries that
// filter on the old key, so the names are the assertion.
func TestSpanAttributeConstructors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attr    attribute.KeyValue
		name    string
		wantKey string
		wantVal string
	}{
		{
			name:    "tool name",
			attr:    observability.ToolAttr("linode_instance_list"),
			wantKey: "mcp.tool.name",
			wantVal: "linode_instance_list",
		},
		{
			name:    "tool argument keys carry the argument name",
			attr:    observability.ToolArgumentAttr("region", "us-east"),
			wantKey: "mcp.tool.argument.region",
			wantVal: "us-east",
		},
		{
			name:    "linode endpoint",
			attr:    observability.LinodeEndpointAttr("/regions"),
			wantKey: "linode.api.endpoint",
			wantVal: "/regions",
		},
		{
			name:    "linode method",
			attr:    observability.LinodeMethodAttr("GET"),
			wantKey: "linode.api.method",
			wantVal: "GET",
		},
		{
			name:    "environment",
			attr:    observability.EnvironmentAttr("prod"),
			wantKey: "linode.environment",
			wantVal: "prod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := string(tt.attr.Key); got != tt.wantKey {
				t.Errorf("attribute key = %q, want %q", got, tt.wantKey)
			}

			if got := tt.attr.Value.AsString(); got != tt.wantVal {
				t.Errorf("attribute value = %q, want %q", got, tt.wantVal)
			}
		})
	}
}

// TestToolResultAttrCarriesAnIntSize covers the one attribute helper that is
// not string-valued: the result size has to stay numeric so backends can
// aggregate it rather than treating every size as a distinct label.
func TestToolResultAttrCarriesAnIntSize(t *testing.T) {
	t.Parallel()

	attr := observability.ToolResultAttr(4096)

	if got := string(attr.Key); got != "mcp.tool.result.size" {
		t.Errorf("attribute key = %q, want %q", got, "mcp.tool.result.size")
	}

	if got := attr.Value.Type(); got != attribute.INT64 {
		t.Errorf("attribute type = %v, want %v so backends can aggregate it", got, attribute.INT64)
	}

	if got := attr.Value.AsInt64(); got != 4096 {
		t.Errorf("attribute value = %d, want %d", got, 4096)
	}
}

// TestWithContextLeavesAnUnspannedLoggerAlone covers the log-correlation
// helper's cheap path. With no recorded span there are no IDs to attach, so
// the helper must hand back the same logger rather than logging empty trace
// fields. The spanned half lives in TestTracingWithAnEndpointRecords, which is
// where a recording span actually exists.
func TestWithContextLeavesAnUnspannedLoggerAlone(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	if got := observability.WithContext(t.Context(), logger); got != logger {
		t.Error("WithContext returned a derived logger for a context with no span, want the original")
	}
}

// TestWithContextDefaultsANilLogger keeps the helper safe at call sites that
// have no logger of their own: a nil argument falls back to the slog default
// instead of panicking.
func TestWithContextDefaultsANilLogger(t *testing.T) {
	t.Parallel()

	if got := observability.WithContext(t.Context(), nil); got == nil {
		t.Error("WithContext(nil logger) returned nil, want the slog default")
	}
}

// TestNewLoggerReturnsTheInstanceLoggerWithoutASpan pairs with WithContext on
// the instance side: with no active span there is no trace ID to add, so the
// instance logger comes back as is.
func TestNewLoggerReturnsTheInstanceLoggerWithoutASpan(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)

	if got := obs.NewLogger(t.Context()); got != obs.Logger() {
		t.Error("NewLogger derived a logger for a context with no span, want the instance logger")
	}
}

// TestLoggingConfigSelectsLevelAndFormat drives the logger construction arms.
// The level decides what the handler lets through, so an unknown level has to
// land on info rather than silently dropping everything.
func TestLoggingConfigSelectsLevelAndFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		level       string
		format      string
		wantEnabled slog.Level
		wantDropped slog.Level
	}{
		{
			name:        "debug lets debug through",
			level:       "debug",
			format:      logFormatJSON,
			wantEnabled: slog.LevelDebug,
			wantDropped: slog.LevelDebug - 1,
		},
		{
			name:        "info drops debug",
			level:       "info",
			format:      logFormatJSON,
			wantEnabled: slog.LevelInfo,
			wantDropped: slog.LevelDebug,
		},
		{
			name:        "warn drops info",
			level:       "warn",
			format:      "text",
			wantEnabled: slog.LevelWarn,
			wantDropped: slog.LevelInfo,
		},
		{
			name:        "warning is accepted as a spelling of warn",
			level:       "warning",
			format:      "text",
			wantEnabled: slog.LevelWarn,
			wantDropped: slog.LevelInfo,
		},
		{
			name:        "error drops warn",
			level:       logLevelError,
			format:      logFormatJSON,
			wantEnabled: slog.LevelError,
			wantDropped: slog.LevelWarn,
		},
		{
			name:        "unknown level falls back to info",
			level:       "chatty",
			format:      logFormatJSON,
			wantEnabled: slog.LevelInfo,
			wantDropped: slog.LevelDebug,
		},
		{
			name:        "empty level falls back to info",
			level:       "",
			format:      "",
			wantEnabled: slog.LevelInfo,
			wantDropped: slog.LevelDebug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			obs, err := observability.New(&config.ObservabilityConfig{
				Logging: config.LoggingConfig{Level: tt.level, Format: tt.format},
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			t.Cleanup(func() {
				if shutdownErr := obs.Shutdown(t.Context()); shutdownErr != nil {
					t.Errorf("Shutdown: %v", shutdownErr)
				}
			})

			if !obs.Logger().Enabled(t.Context(), tt.wantEnabled) {
				t.Errorf("level %q rejects %v, want it enabled", tt.level, tt.wantEnabled)
			}

			if obs.Logger().Enabled(t.Context(), tt.wantDropped) {
				t.Errorf("level %q accepts %v, want it dropped", tt.level, tt.wantDropped)
			}
		})
	}
}

// TestRecordToolCallAcceptsBothOutcomes drives the dispatch-side recorder.
// It returns nothing by design, so the contract worth pinning is that neither
// outcome panics or blocks with metrics switched off.
func TestRecordToolCallAcceptsBothOutcomes(t *testing.T) {
	t.Parallel()

	obs := newTestObservability(t)

	obs.RecordToolCall(t.Context(), "linode_instance_list", 25*time.Millisecond, nil)
	obs.RecordToolCall(t.Context(), "linode_instance_list", 25*time.Millisecond, errToolFailed)
}

// TestTracingWithoutAnEndpointStaysNoop covers the deliberate no-export path:
// tracing on but no endpoint configured must leave a tracer that records
// nothing rather than reaching for an implicit default collector.
func TestTracingWithoutAnEndpointStaysNoop(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Tracing: config.TracingConfig{Enabled: true, SampleRate: 1.0},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()

		if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	_, span := obs.Tracer().Start(t.Context(), "probe")
	defer span.End()

	if span.IsRecording() {
		t.Error("span is recording with no tracing endpoint configured, want a noop tracer")
	}
}

// TestTracingWithAnEndpointRecords is the other half: once an endpoint is
// configured the tracer must produce recording spans, otherwise the exporter
// would never see anything to send.
func TestTracingWithAnEndpointRecords(t *testing.T) {
	t.Parallel()

	obs, err := observability.New(&config.ObservabilityConfig{
		Logging: config.LoggingConfig{Level: logLevelError, Format: logFormatJSON},
		Tracing: config.TracingConfig{
			Enabled:    true,
			Endpoint:   "127.0.0.1:4318",
			Protocol:   "http",
			Insecure:   true,
			SampleRate: 1.0,
			Headers:    map[string]string{"x-probe": "1"},
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
		defer cancel()

		if shutdownErr := obs.Shutdown(shutdownCtx); shutdownErr != nil {
			t.Errorf("Shutdown: %v", shutdownErr)
		}
	})

	ctx, span := obs.Tracer().Start(t.Context(), "probe")
	defer span.End()

	if !span.IsRecording() {
		t.Fatal("span is not recording with tracing configured, want a live tracer")
	}

	// The span-annotating helpers only act on a recording span, so this is the
	// context in which their non-trivial arm runs.
	observability.WithEnvironment(ctx, "prod")
	observability.WithToolArgument(ctx, "region", "us-east")
	observability.WithToolResultSize(ctx, 1024)
	observability.RecordEvent(ctx, "probe.event", observability.ToolAttr("linode_instance_list"))

	// A recording span carries a trace ID, so the log-correlation helpers have
	// something to attach and must hand back a derived logger.
	base := slog.Default()

	if got := observability.WithContext(ctx, base); got == base {
		t.Error("WithContext returned the original logger for a recording span, want trace fields attached")
	}

	if got := obs.NewLogger(ctx); got == obs.Logger() {
		t.Error("NewLogger returned the instance logger for a recording span, want a trace_id attached")
	}
}
