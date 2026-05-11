package observability

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
func (o *Observability) initMetrics(cfg *config.MetricsConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), metricsInitTimeout)
	defer cancel()

	res, err := resource.New(
		ctx,
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

	var readers []sdkmetric.Reader

	if cfg.Prometheus.Enabled {
		readers = append(readers, o.newPrometheusReader(cfg.Prometheus.Port, cfg.Prometheus.Path))
	}

	if cfg.OTLP.Enabled {
		_, err := newOTLPReader(ctx, cfg.OTLP)
		if err != nil {
			o.logger.Debug("OTLP metrics not implemented, using Prometheus only", "error", err)
		}
	}

	opts := []sdkmetric.Option{sdkmetric.WithResource(res)}
	for _, reader := range readers {
		opts = append(opts, sdkmetric.WithReader(reader))
	}

	o.meterProvider = sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(o.meterProvider)

	o.registerShutdown(func(ctx context.Context) error {
		return o.meterProvider.Shutdown(ctx)
	})

	if cfg.Runtime {
		if err := runtime.Start(
			runtime.WithMeterProvider(o.meterProvider),
			runtime.WithMinimumReadMemStatsInterval(metricsReadMemInterval),
		); err != nil {
			o.logger.Error("failed to start runtime metrics", "error", err)

			return nil
		}

		o.logger.Info("runtime metrics enabled")
	}

	if cfg.Host {
		if err := host.Start(host.WithMeterProvider(o.meterProvider)); err != nil {
			o.logger.Error("failed to start host metrics", "error", err)

			return nil
		}

		o.logger.Info("host metrics enabled")
	}

	if err := o.createCustomMetrics(o.meterProvider); err != nil {
		return fmt.Errorf("failed to create custom metrics: %w", err)
	}

	return nil
}

// newPrometheusReader creates a Prometheus exporter and starts the scrape
// HTTP server. Caller must hold no metrics-related locks.
func (o *Observability) newPrometheusReader(port int, path string) *sdkmetric.ManualReader {
	reader := sdkmetric.NewManualReader()

	if o.metricsServer != nil {
		return reader
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	))

	addr := fmt.Sprintf(":%d", port)
	o.metricsServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		o.logger.Info("starting metrics server", "address", addr, "path", path)

		if err := o.metricsServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			o.logger.Error("metrics server failed", "error", err)
		}
	}()

	o.registerShutdown(func(shutdownCtx context.Context) error {
		if o.metricsServer != nil {
			return o.metricsServer.Shutdown(shutdownCtx)
		}

		return nil
	})

	return reader
}

// newOTLPReader creates an OTLP metrics reader.
// Parameters are intentionally unused as this is a stub implementation.
func newOTLPReader(context.Context, config.OTLPMetricsConfig) (*sdkmetric.PeriodicReader, error) {
	// OTLP metrics exporter to be implemented when needed
	// For now, rely on Prometheus only
	return nil, errOTLPNotImplemented
}

// createCustomMetrics creates LinodeMCP-specific metrics on the instance.
func (o *Observability) createCustomMetrics(mp metric.MeterProvider) error {
	meter := mp.Meter("github.com/chadit/LinodeMCP")

	var err error

	o.requestsTotal, err = meter.Int64Counter(
		"linodemcp.requests.total",
		metric.WithDescription("Total number of MCP requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create requests counter: %w", err)
	}

	o.requestDuration, err = meter.Float64Histogram(
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

	o.errorsTotal, err = meter.Int64Counter(
		"linodemcp.errors.total",
		metric.WithDescription("Total number of MCP errors"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create errors counter: %w", err)
	}

	o.apiRequests, err = meter.Int64Counter(
		"linodemcp.api.requests.total",
		metric.WithDescription("Total number of Linode API requests"),
		metric.WithUnit("1"),
	)
	if err != nil {
		return fmt.Errorf("failed to create API requests counter: %w", err)
	}

	o.apiRequestDur, err = meter.Float64Histogram(
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
func (o *Observability) RecordRequest(ctx context.Context, tool, method, status string, duration float64) {
	if o.requestsTotal == nil {
		return
	}

	o.requestsTotal.Add(
		ctx, 1,
		metric.WithAttributes(
			attribute.String("tool", tool),
			attribute.String("method", method),
			attribute.String("status", status),
		),
	)

	if duration > 0 {
		o.requestDuration.Record(
			ctx, duration,
			metric.WithAttributes(
				attribute.String("tool", tool),
				attribute.String("method", method),
			),
		)
	}
}

// RecordError records an MCP error.
func (o *Observability) RecordError(ctx context.Context, tool, errorType string) {
	if o.errorsTotal == nil {
		return
	}

	o.errorsTotal.Add(
		ctx, 1,
		metric.WithAttributes(
			attribute.String("tool", tool),
			attribute.String("error_type", errorType),
		),
	)
}

// RecordAPIRequest records a completed Linode API request.
func (o *Observability) RecordAPIRequest(ctx context.Context, endpoint, method string, status int, duration float64) {
	if o.apiRequests == nil {
		return
	}

	o.apiRequests.Add(
		ctx, 1,
		metric.WithAttributes(
			attribute.String("endpoint", endpoint),
			attribute.String("method", method),
			attribute.Int("status_code", status),
		),
	)

	if duration > 0 {
		o.apiRequestDur.Record(
			ctx, duration,
			metric.WithAttributes(
				attribute.String("endpoint", endpoint),
				attribute.String("method", method),
			),
		)
	}
}
