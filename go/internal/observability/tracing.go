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

	"github.com/chadit/LinodeMCP/internal/config"
)

const (
	tracingInitTimeout        = 10 * time.Second
	tracingBatchTimeout       = 5 * time.Second
	tracingMaxExportBatchSize = 512
)

// initTracing sets up OpenTelemetry tracing with OTLP export.
// Configuration is primarily via OTEL_* environment variables.
func initTracing(cfg config.TracingConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), tracingInitTimeout)
	defer cancel()

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

	// Create exporter based on protocol
	var exporter sdktrace.SpanExporter

	switch cfg.Protocol {
	case "http", "http/protobuf":
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
		}

		exporter, err = otlptracehttp.New(ctx, opts...)
	case "grpc", "grpc/protobuf":
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}

		if len(cfg.Headers) > 0 {
			opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
		}

		exporter, err = otlptracegrpc.New(ctx, opts...)
	default:
		// Try to detect from endpoint
		if cfg.Endpoint != "" {
			// Default to gRPC
			opts := []otlptracegrpc.Option{
				otlptracegrpc.WithEndpoint(cfg.Endpoint),
			}
			if cfg.Insecure {
				opts = append(opts, otlptracegrpc.WithInsecure())
			}

			if len(cfg.Headers) > 0 {
				opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
			}

			exporter, err = otlptracegrpc.New(ctx, opts...)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to create exporter: %w", err)
	}

	// Configure sampler based on sample rate
	sampler := sdktrace.AlwaysSample()
	if cfg.SampleRate < 1.0 && cfg.SampleRate >= 0 {
		sampler = sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRate),
		)
	}

	// Create trace provider
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(tracingBatchTimeout),
			sdktrace.WithMaxExportBatchSize(tracingMaxExportBatchSize),
		),
	)

	// Set global tracer provider
	otel.SetTracerProvider(traceProvider)

	// Set global propagator (tracecontext + baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Register shutdown
	registerShutdown(func(ctx context.Context) error {
		return traceProvider.Shutdown(ctx)
	})

	// Also respect OTEL_EXPORTER_OTLP_* environment variables
	// by using the SDK's automatic configuration if endpoint not set
	if cfg.Endpoint == "" {
		// Reset to noop if no configuration provided
		otel.SetTracerProvider(noop.NewTracerProvider())
	}

	return nil
}

// getVersion returns the application version from build info or env.
func getVersion() string {
	if v := os.Getenv("LINODEMCP_VERSION"); v != "" {
		return v
	}

	return "dev"
}

// SpanAttributes contains common attribute builders for LinodeMCP spans.
type SpanAttributes struct{}

// Tool creates an attribute for the tool name.
func (SpanAttributes) Tool(name string) attribute.KeyValue {
	return attribute.String("mcp.tool.name", name)
}

// ToolArgument creates an attribute for a tool argument.
func (SpanAttributes) ToolArgument(name, value string) attribute.KeyValue {
	return attribute.String("mcp.tool.argument."+name, value)
}

// ToolResult creates an attribute for the tool result size.
func (SpanAttributes) ToolResult(size int) attribute.KeyValue {
	return attribute.Int("mcp.tool.result.size", size)
}

// LinodeEndpoint creates an attribute for the Linode API endpoint.
func (SpanAttributes) LinodeEndpoint(endpoint string) attribute.KeyValue {
	return attribute.String("linode.api.endpoint", endpoint)
}

// LinodeMethod creates an attribute for the Linode API method.
func (SpanAttributes) LinodeMethod(method string) attribute.KeyValue {
	return attribute.String("linode.api.method", method)
}

// Environment creates an attribute for the environment name.
func (SpanAttributes) Environment(name string) attribute.KeyValue {
	return attribute.String("linode.environment", name)
}

// spanAttrs provides common span attribute builders.
//
//nolint:gochecknoglobals // Singleton pattern for span attribute builders
var spanAttrs = SpanAttributes{}

// ToolAttr creates an attribute for the tool name.
func ToolAttr(name string) attribute.KeyValue {
	return spanAttrs.Tool(name)
}

// ToolArgumentAttr creates an attribute for a tool argument.
func ToolArgumentAttr(name, value string) attribute.KeyValue {
	return spanAttrs.ToolArgument(name, value)
}

// ToolResultAttr creates an attribute for the tool result size.
func ToolResultAttr(size int) attribute.KeyValue {
	return spanAttrs.ToolResult(size)
}

// LinodeEndpointAttr creates an attribute for the Linode API endpoint.
func LinodeEndpointAttr(endpoint string) attribute.KeyValue {
	return spanAttrs.LinodeEndpoint(endpoint)
}

// LinodeMethodAttr creates an attribute for the Linode API method.
func LinodeMethodAttr(method string) attribute.KeyValue {
	return spanAttrs.LinodeMethod(method)
}

// EnvironmentAttr creates an attribute for the environment name.
func EnvironmentAttr(name string) attribute.KeyValue {
	return spanAttrs.Environment(name)
}
