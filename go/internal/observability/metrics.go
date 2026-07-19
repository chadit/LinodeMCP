package observability

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/chadit/LinodeMCP/go/internal/config"
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

// initMetrics sets up OpenTelemetry metrics with Prometheus export.
func (o *Observability) initMetrics(cfg *config.MetricsConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), metricsInitTimeout)
	defer cancel()

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("linodemcp"),
			semconv.ServiceVersion(version()),
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
		reader, err := o.newPrometheusReader(ctx, cfg.Prometheus.Host, cfg.Prometheus.Port, cfg.Prometheus.Path)
		if err != nil {
			return fmt.Errorf("failed to create prometheus reader: %w", err)
		}

		readers = append(readers, reader)
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

// newPrometheusReader creates an OpenTelemetry Prometheus exporter backed by a
// dedicated registry and starts the scrape HTTP server against that same
// registry. The exporter IS the SDK reader: registering it on the meter
// provider is what bridges the linodemcp.* instruments to the /metrics
// endpoint. An earlier version handed back a bare ManualReader and served the
// global default registry, so the instruments recorded into a reader nobody
// read and the endpoint only ever showed go_*/process_*. A per-instance
// registry (rather than the global default) also keeps multiple Observability
// instances, and parallel tests, from colliding on duplicate collector
// registration. The Go runtime and process collectors are registered
// explicitly so the endpoint still carries go_* and process_* alongside the
// application metrics. Caller must hold no metrics-related locks.
func (o *Observability) newPrometheusReader(ctx context.Context, bindHost string, port int, path string) (sdkmetric.Reader, error) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	exporter, err := otelprom.New(otelprom.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	if o.metricsServer != nil {
		return exporter, nil
	}

	mux := http.NewServeMux()
	mux.Handle(path, promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	))

	if bindHost == "" {
		bindHost = config.DefaultBindHost
	}

	addr := net.JoinHostPort(bindHost, strconv.Itoa(port))

	// Bind the listener synchronously so the endpoint is reachable the moment
	// New returns. Serving in a goroutine before binding left a startup window
	// where an immediate scrape raced the listener, and a bind failure (port
	// in use) only surfaced as a log line from a detached goroutine instead of
	// an init error the caller can see.
	var listenCfg net.ListenConfig

	listener, err := listenCfg.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bind metrics server on %s: %w", addr, err)
	}

	o.metricsServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	o.logger.Info("starting metrics server", "address", addr, "path", path)

	go func() {
		if serveErr := o.metricsServer.Serve(listener); !errors.Is(serveErr, http.ErrServerClosed) {
			o.logger.Error("metrics server failed", "error", serveErr)
		}
	}()

	o.registerShutdown(func(shutdownCtx context.Context) error {
		if o.metricsServer != nil {
			return o.metricsServer.Shutdown(shutdownCtx)
		}

		return nil
	})

	return exporter, nil
}

// RequestDurationBoundaries returns the pinned bucket boundaries for
// linodemcp.request.duration.seconds. Every language declares the same
// values; testdata/observability/duration_buckets.json is the shared
// fixture each language's tests assert against, because the exported
// Prometheus _bucket series must match across implementations.
func RequestDurationBoundaries() []float64 {
	return []float64{
		requestDurationBucket1, requestDurationBucket2, requestDurationBucket3,
		requestDurationBucket4, requestDurationBucket5, requestDurationBucket6,
		requestDurationBucket7, requestDurationBucket8, requestDurationBucket9,
		requestDurationBucket10, requestDurationBucket11, requestDurationBucket12,
	}
}

// APIRequestDurationBoundaries returns the pinned bucket boundaries for
// linodemcp.api.request.duration.seconds; same cross-language contract as
// RequestDurationBoundaries.
func APIRequestDurationBoundaries() []float64 {
	return []float64{
		apiDurationBucket1, apiDurationBucket2, apiDurationBucket3,
		apiDurationBucket4, apiDurationBucket5, apiDurationBucket6,
		apiDurationBucket7, apiDurationBucket8, apiDurationBucket9,
		apiDurationBucket10,
	}
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
		metric.WithExplicitBucketBoundaries(RequestDurationBoundaries()...),
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
		metric.WithExplicitBucketBoundaries(APIRequestDurationBoundaries()...),
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
