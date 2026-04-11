package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/chadit/LinodeMCP/internal/config"
)

// metricsState holds all metrics-related state.
type metricsState struct {
	mu              sync.RWMutex
	requestsTotal   metric.Int64Counter
	requestDuration metric.Float64Histogram
	errorsTotal     metric.Int64Counter
	apiRequests     metric.Int64Counter
	apiRequestDur   metric.Float64Histogram
	server          *http.Server
	initialized     bool
}

//nolint:gochecknoglobals // Singleton pattern required for metrics infrastructure
var metrics metricsState

const (
	metricsInitTimeout     = 10 * time.Second
	metricsReadMemInterval = 15 * time.Second

	// Histogram bucket boundaries for request duration (in seconds).
	requestDurationBucket1  = 0.001
	requestDurationBucket2  = 0.005
	requestDurationBucket3  = 0.01
	requestDurationBucket4  = 0.025
	requestDurationBucket5  = 0.05
	requestDurationBucket6  = 0.1
	requestDurationBucket7  = 0.25
	requestDurationBucket8  = 0.5
	requestDurationBucket9  = 1
	requestDurationBucket10 = 2.5
	requestDurationBucket11 = 5
	requestDurationBucket12 = 10

	// Histogram bucket boundaries for API request duration (in seconds).
	apiDurationBucket1  = 0.01
	apiDurationBucket2  = 0.05
	apiDurationBucket3  = 0.1
	apiDurationBucket4  = 0.25
	apiDurationBucket5  = 0.5
	apiDurationBucket6  = 1
	apiDurationBucket7  = 2.5
	apiDurationBucket8  = 5
	apiDurationBucket9  = 10
	apiDurationBucket10 = 30
)

// initMetrics sets up OpenTelemetry metrics with Prometheus and OTLP export.
func initMetrics(cfg *config.MetricsConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), metricsInitTimeout)
	defer cancel()

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	if metrics.initialized {
		return nil
	}

	// Build resource with service info
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("linodemcp"),
			semconv.ServiceVersion(getVersion()),
		),
		resource.WithHost(),
		resource.WithOS(),
		resource.WithProcess(),
	)
	if err != nil {
		return fmt.Errorf("failed to create resource: %w", err)
	}

	// Create readers for Prometheus and OTLP
	var readers []sdkmetric.Reader

	// Prometheus exporter for scraping
	if cfg.Prometheus.Enabled {
		promReader, err := newPrometheusReader(cfg.Prometheus.Port, cfg.Prometheus.Path)
		if err != nil {
			return fmt.Errorf("failed to create prometheus reader: %w", err)
		}

		readers = append(readers, promReader)
	}

	// OTLP exporter for pushing metrics - skip if not implemented
	if cfg.OTLP.Enabled {
		_, err := newOTLPReader(ctx, cfg.OTLP)
		if err != nil {
			Logger().Debug("OTLP metrics not implemented, using Prometheus only", "error", err)
		}
	}

	// Create meter provider with all readers
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}

	for _, reader := range readers {
		opts = append(opts, sdkmetric.WithReader(reader))
	}

	meterProvider := sdkmetric.NewMeterProvider(opts...)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Register shutdown
	registerShutdown(func(ctx context.Context) error {
		return meterProvider.Shutdown(ctx)
	})

	// Start runtime metrics collection (Go runtime: goroutines, memory, GC)
	if cfg.Runtime {
		if err := runtime.Start(
			runtime.WithMeterProvider(meterProvider),
			runtime.WithMinimumReadMemStatsInterval(metricsReadMemInterval),
		); err != nil {
			Logger().Error("failed to start runtime metrics", "error", err)

			return nil
		}

		Logger().Info("runtime metrics enabled")
	}

	// Start host metrics collection (CPU, memory, network)
	if cfg.Host {
		if err := host.Start(host.WithMeterProvider(meterProvider)); err != nil {
			Logger().Error("failed to start host metrics", "error", err)

			return nil
		}

		Logger().Info("host metrics enabled")
	}

	// Create custom metrics
	if err := createCustomMetrics(meterProvider); err != nil {
		return fmt.Errorf("failed to create custom metrics: %w", err)
	}

	metrics.initialized = true

	return nil
}

// newPrometheusReader creates a Prometheus exporter and starts the HTTP server.
func newPrometheusReader(port int, path string) (*sdkmetric.ManualReader, error) {
	reader := sdkmetric.NewManualReader()

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	if metrics.server == nil {
		mux := http.NewServeMux()
		mux.Handle(path, promhttp.HandlerFor(
			prometheus.DefaultGatherer,
			promhttp.HandlerOpts{
				EnableOpenMetrics: true,
			},
		))

		addr := fmt.Sprintf(":%d", port)
		metrics.server = &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}

		go func() {
			Logger().Info("starting metrics server", "address", addr, "path", path)

			if err := metrics.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				Logger().Error("metrics server failed", "error", err)
			}
		}()

		registerShutdown(func(shutdownCtx context.Context) error {
			if metrics.server != nil {
				return metrics.server.Shutdown(shutdownCtx)
			}

			return nil
		})
	}

	return reader, nil
}

// newOTLPReader creates an OTLP metrics reader.
// Parameters are intentionally unused as this is a stub implementation.
func newOTLPReader(context.Context, config.OTLPMetricsConfig) (*sdkmetric.PeriodicReader, error) {
	// OTLP metrics exporter to be implemented when needed
	// For now, rely on Prometheus only
	return nil, errOTLPNotImplemented
}

// createCustomMetrics creates LinodeMCP-specific metrics.
func createCustomMetrics(mp metric.MeterProvider) error {
	meter := mp.Meter("github.com/chadit/LinodeMCP")

	var err error

	// Request counter
	metrics.requestsTotal, err = meter.Int64Counter(
		"linodemcp.requests.total",
		metric.WithDescription("Total number of MCP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create requests counter: %w", err)
	}

	// Request duration histogram
	metrics.requestDuration, err = meter.Float64Histogram(
		"linodemcp.request.duration.seconds",
		metric.WithDescription("Duration of MCP requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			requestDurationBucket1, requestDurationBucket2, requestDurationBucket3,
			requestDurationBucket4, requestDurationBucket5, requestDurationBucket6,
			requestDurationBucket7, requestDurationBucket8, requestDurationBucket9,
			requestDurationBucket10, requestDurationBucket11, requestDurationBucket12,
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create duration histogram: %w", err)
	}

	// Error counter
	metrics.errorsTotal, err = meter.Int64Counter(
		"linodemcp.errors.total",
		metric.WithDescription("Total number of MCP errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create errors counter: %w", err)
	}

	// API request counter
	metrics.apiRequests, err = meter.Int64Counter(
		"linodemcp.api.requests.total",
		metric.WithDescription("Total number of Linode API requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create API requests counter: %w", err)
	}

	// API request duration histogram
	metrics.apiRequestDur, err = meter.Float64Histogram(
		"linodemcp.api.request.duration.seconds",
		metric.WithDescription("Duration of Linode API requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			apiDurationBucket1, apiDurationBucket2, apiDurationBucket3,
			apiDurationBucket4, apiDurationBucket5, apiDurationBucket6,
			apiDurationBucket7, apiDurationBucket8, apiDurationBucket9,
			apiDurationBucket10,
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create API duration histogram: %w", err)
	}

	return nil
}

// RecordRequest records a completed MCP request.
func RecordRequest(ctx context.Context, tool, method, status string, duration float64) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	if metrics.requestsTotal == nil {
		return
	}

	metrics.requestsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("tool", tool),
			attribute.String("method", method),
			attribute.String("status", status),
		),
	)

	if duration > 0 {
		metrics.requestDuration.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("tool", tool),
				attribute.String("method", method),
			),
		)
	}
}

// RecordError records an MCP error.
func RecordError(ctx context.Context, tool, errorType string) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	if metrics.errorsTotal == nil {
		return
	}

	metrics.errorsTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("tool", tool),
			attribute.String("error_type", errorType),
		),
	)
}

// RecordAPIRequest records a completed Linode API request.
func RecordAPIRequest(ctx context.Context, endpoint, method string, status int, duration float64) {
	metrics.mu.RLock()
	defer metrics.mu.RUnlock()

	if metrics.apiRequests == nil {
		return
	}

	metrics.apiRequests.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("endpoint", endpoint),
			attribute.String("method", method),
			attribute.Int("status_code", status),
		),
	)

	if duration > 0 {
		metrics.apiRequestDur.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String("endpoint", endpoint),
				attribute.String("method", method),
			),
		)
	}
}
