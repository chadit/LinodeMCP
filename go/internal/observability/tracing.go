package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/chadit/LinodeMCP/go/internal/config"
)

const (
	tracingInitTimeout        = 10 * time.Second
	tracingBatchTimeout       = 5 * time.Second
	tracingMaxExportBatchSize = 512
)

// initTracing constructs the tracer provider and stores the tracer on the
// instance. Honors OTEL_* environment variables via the SDK's auto-config
// when no endpoint is set in the local config.
func (o *Observability) initTracing(cfg config.TracingConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), tracingInitTimeout)
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

	exporter, err := buildTraceExporter(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate < 1.0 && cfg.SampleRate >= 0 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))
	}

	o.traceProvider = sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithBatchTimeout(tracingBatchTimeout),
			sdktrace.WithMaxExportBatchSize(tracingMaxExportBatchSize),
		),
	)

	otel.SetTracerProvider(o.traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	o.tracer = o.traceProvider.Tracer("github.com/chadit/LinodeMCP")

	o.registerShutdown(func(shutdownCtx context.Context) error {
		return o.traceProvider.Shutdown(shutdownCtx)
	})

	if cfg.Endpoint == "" {
		// No endpoint -> reset to noop so we don't try to export to nowhere.
		// The provider above is registered for completeness; flipping the
		// tracer here means the rest of the app sees a no-op.
		o.tracer = noop.NewTracerProvider().Tracer("linodemcp")
		otel.SetTracerProvider(noop.NewTracerProvider())
	}

	return nil
}

// buildTraceExporter selects the right OTLP exporter for the configured
// protocol. Defaults to gRPC when only an endpoint is provided.
func buildTraceExporter(ctx context.Context, cfg config.TracingConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
		}

		exp, err := otlptracehttp.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("otlptracehttp.New: %w", err)
		}

		return exp, nil
	default:
		opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}

		exp, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("otlptracegrpc.New: %w", err)
		}

		return exp, nil
	}
}

// version returns the application version from build info or env.
func version() string {
	if v := os.Getenv("LINODEMCP_VERSION"); v != "" {
		return v
	}

	return "dev"
}

// Span attribute constructors. Pure functions, no observability state needed.

// ToolAttr creates an attribute for the tool name.
func ToolAttr(name string) attribute.KeyValue {
	return attribute.String("mcp.tool.name", name)
}

// ToolArgumentAttr creates an attribute for a tool argument.
func ToolArgumentAttr(name, value string) attribute.KeyValue {
	return attribute.String("mcp.tool.argument."+name, value)
}

// ToolResultAttr creates an attribute for the tool result size.
func ToolResultAttr(size int) attribute.KeyValue {
	return attribute.Int("mcp.tool.result.size", size)
}

// LinodeEndpointAttr creates an attribute for the Linode API endpoint.
func LinodeEndpointAttr(endpoint string) attribute.KeyValue {
	return attribute.String("linode.api.endpoint", endpoint)
}

// LinodeMethodAttr creates an attribute for the Linode API method.
func LinodeMethodAttr(method string) attribute.KeyValue {
	return attribute.String("linode.api.method", method)
}

// EnvironmentAttr creates an attribute for the environment name.
func EnvironmentAttr(name string) attribute.KeyValue {
	return attribute.String("linode.environment", name)
}
