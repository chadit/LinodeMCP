// Package observability provides tracing, metrics, logging, and health check
// functionality for LinodeMCP using OpenTelemetry and Prometheus.
//
// Construct an *Observability with New, defer Shutdown, and inject the value
// into anything that needs it. The package holds no global state so multiple
// instances can coexist (production, test harnesses, multi-tenant).
package observability

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/chadit/LinodeMCP/internal/config"
)

// Observability bundles tracing, metrics, logging, and health endpoints.
// All fields are owned by the instance; nothing leaks to package globals.
type Observability struct {
	logger *slog.Logger
	tracer trace.Tracer

	traceProvider *sdktrace.TracerProvider
	meterProvider *sdkmetric.MeterProvider

	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorsTotal     metric.Int64Counter
	apiRequests     metric.Int64Counter
	apiRequestDur   metric.Float64Histogram
	metricsServer   *http.Server

	healthMu     sync.RWMutex
	healthChecks map[string]HealthCheck
	healthServer *http.Server

	shutdownMu    sync.Mutex
	shutdownFuncs []func(context.Context) error
}

// New constructs and starts an Observability stack.
// A nil cfg uses sensible defaults (info-level JSON logs, all subsystems off).
func New(cfg *config.ObservabilityConfig) (*Observability, error) {
	if cfg == nil {
		cfg = &config.ObservabilityConfig{
			Logging: config.LoggingConfig{Level: "info", Format: "json"},
		}
	}

	obs := &Observability{
		logger:       defaultLogger(),
		tracer:       noop.NewTracerProvider().Tracer("linodemcp"),
		healthChecks: make(map[string]HealthCheck),
	}

	obs.initLogging(cfg.Logging)
	obs.logger.Info("initializing observability")

	if cfg.Tracing.Enabled {
		if err := obs.initTracing(cfg.Tracing); err != nil {
			obs.logger.Error("tracing initialization failed", "error", err)
		}

		if obs.traceProvider != nil {
			obs.logger.Info("tracing initialized")
		}
	}

	if cfg.Metrics.Enabled {
		if err := obs.initMetrics(&cfg.Metrics); err != nil {
			obs.logger.Error("metrics initialization failed", "error", err)
		}

		if obs.meterProvider != nil {
			obs.logger.Info("metrics initialized")
		}
	}

	obs.initHealth(cfg.Health)

	if cfg.Health.Enabled {
		obs.logger.Info("health checks initialized")
	}

	obs.logger.Info("observability initialization complete")

	return obs, nil
}

// Logger returns the configured slog logger.
func (o *Observability) Logger() *slog.Logger { return o.logger }

// Tracer returns the configured OpenTelemetry tracer.
func (o *Observability) Tracer() trace.Tracer { return o.tracer }

// Shutdown runs all registered shutdown hooks in LIFO order.
// Safe to call once per instance; subsequent calls are no-ops.
func (o *Observability) Shutdown(ctx context.Context) error {
	o.shutdownMu.Lock()
	defer o.shutdownMu.Unlock()

	if len(o.shutdownFuncs) == 0 {
		return nil
	}

	o.logger.Info("shutting down observability")

	var lastErr error

	for i := len(o.shutdownFuncs) - 1; i >= 0; i-- {
		if err := o.shutdownFuncs[i](ctx); err != nil {
			lastErr = err

			o.logger.Error("shutdown error", "error", err)
		}
	}

	o.shutdownFuncs = nil

	return lastErr
}

// registerShutdown queues a shutdown hook. Caller must hold no locks the hook
// will reacquire.
func (o *Observability) registerShutdown(fn func(context.Context) error) {
	o.shutdownMu.Lock()
	defer o.shutdownMu.Unlock()

	o.shutdownFuncs = append(o.shutdownFuncs, fn)
}

// defaultLogger returns a stderr text logger used until initLogging configures
// the real one. Keeps Logger() non-nil during early init.
func defaultLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
}
