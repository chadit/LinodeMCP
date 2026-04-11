package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/chadit/LinodeMCP/internal/config"
)

// initLogging configures the global slog logger with JSON or text output.
func initLogging(cfg config.LoggingConfig) {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.Level),
	}

	if strings.EqualFold(cfg.Format, "json") {
		handler := slog.NewJSONHandler(os.Stderr, opts)
		state.logger.Store(slog.New(handler))
		slog.SetDefault(Logger())

		return
	}

	handler := slog.NewTextHandler(os.Stderr, opts)
	state.logger.Store(slog.New(handler))
	slog.SetDefault(Logger())
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a new logger with context-aware fields.
func NewLogger(ctx context.Context) *slog.Logger {
	base := Logger()

	// Add trace ID if available
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		traceID := span.SpanContext().TraceID().String()

		return base.With("trace_id", traceID)
	}

	return base
}

// WithContext returns a logger with trace context from the span.
func WithContext(ctx context.Context, log *slog.Logger) *slog.Logger {
	if log == nil {
		log = Logger()
	}

	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		traceID := span.SpanContext().TraceID().String()
		spanID := span.SpanContext().SpanID().String()

		return log.With("trace_id", traceID, "span_id", spanID)
	}

	return log
}
