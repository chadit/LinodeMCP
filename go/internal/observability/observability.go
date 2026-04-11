// Package observability provides tracing, metrics, logging, and health check
// functionality for LinodeMCP using OpenTelemetry and Prometheus.
package observability

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/chadit/LinodeMCP/internal/config"
)

// state holds all observability package state.
// This avoids scattered global variables.
//
//nolint:gochecknoglobals // Singleton pattern required for observability infrastructure
var state struct {
	mu            sync.RWMutex
	initialized   atomic.Bool
	logger        atomic.Value // stores *slog.Logger
	tracer        atomic.Value // stores trace.Tracer
	shutdownFuncs []func(context.Context) error
}

// Logger returns the configured slog logger.
func Logger() *slog.Logger {
	logger := state.logger.Load()
	if logger == nil {
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	}

	return logger.(*slog.Logger) //nolint:forcetypeassert // State is always initialized with *slog.Logger
}

// Tracer returns the configured OpenTelemetry tracer.
func Tracer() trace.Tracer {
	t := state.tracer.Load()
	if t == nil {
		return noop.NewTracerProvider().Tracer("linodemcp")
	}

	return t.(trace.Tracer) //nolint:forcetypeassert // State is always initialized with trace.Tracer
}

// Init initializes all observability components (tracing, metrics, logging, health).
// It respects OTEL_* environment variables for configuration.
func Init(cfg *config.ObservabilityConfig) error {
	if state.initialized.Load() {
		return errAlreadyInitialized
	}

	// Use defaults if config is nil
	if cfg == nil {
		cfg = &config.ObservabilityConfig{
			Logging: config.LoggingConfig{
				Level:  "info",
				Format: "json",
			},
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// Initialize logging first so we can log during setup
	initLogging(cfg.Logging)

	Logger().Info("initializing observability")

	// Initialize tracing
	if cfg.Tracing.Enabled {
		if err := initTracing(cfg.Tracing); err != nil {
			Logger().Error("tracing initialization failed", "error", err)
			// Continue without tracing
			return nil
		}

		state.tracer.Store(otel.Tracer("github.com/chadit/LinodeMCP"))
		Logger().Info("tracing initialized")
	}

	// Initialize metrics
	if cfg.Metrics.Enabled {
		if err := initMetrics(&cfg.Metrics); err != nil {
			Logger().Error("metrics initialization failed", "error", err)
			// Continue without metrics
			return nil
		}

		Logger().Info("metrics initialized")
	}

	// Initialize health checks
	initHealth(cfg.Health)
	Logger().Info("health checks initialized")

	state.initialized.Store(true)
	Logger().Info("observability initialization complete")

	return nil
}

// Shutdown gracefully shuts down all observability components.
func Shutdown(ctx context.Context) error {
	if !state.initialized.Load() {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	Logger().Info("shutting down observability")

	var lastErr error

	for i := len(state.shutdownFuncs) - 1; i >= 0; i-- {
		if err := state.shutdownFuncs[i](ctx); err != nil {
			lastErr = err
			Logger().Error("shutdown error", "error", err)
		}
	}

	return lastErr
}

// registerShutdown registers a shutdown function to be called on Shutdown().
// Must be called with state.mu held.
func registerShutdown(fn func(context.Context) error) {
	state.shutdownFuncs = append(state.shutdownFuncs, fn)
}
