package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/chadit/LinodeMCP/internal/config"
)

// initLogging builds the logger and stores it on the instance. Also wires
// slog.SetDefault so callers that reach for slog.Default() see the same
// configuration without having to thread *Observability through.
func (o *Observability) initLogging(cfg config.LoggingConfig) {
	opts := &slog.HandlerOptions{Level: parseLogLevel(cfg.Level)}

	handler := slog.Handler(slog.NewTextHandler(os.Stderr, opts))
	if strings.EqualFold(cfg.Format, "json") {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	o.logger = slog.New(handler)
	slog.SetDefault(o.logger)
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

// NewLogger returns a logger derived from the instance logger with trace
// context attached when ctx carries an active span.
func (o *Observability) NewLogger(ctx context.Context) *slog.Logger {
	if span := trace.SpanFromContext(ctx); span.SpanContext().HasTraceID() {
		return o.logger.With("trace_id", span.SpanContext().TraceID().String())
	}

	return o.logger
}

// WithContext attaches trace and span IDs from ctx to the supplied logger.
// Pure helper, no observability state required.
func WithContext(ctx context.Context, log *slog.Logger) *slog.Logger {
	if log == nil {
		log = slog.Default()
	}

	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().HasTraceID() {
		return log
	}

	return log.With(
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	)
}
