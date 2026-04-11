package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ToolExecution records the execution of an MCP tool with tracing and metrics.
func ToolExecution(ctx context.Context, toolName string, executeFn func(ctx context.Context) error) error {
	tracer := Tracer()

	ctx, span := tracer.Start(ctx, "mcp.tool.execute",
		trace.WithAttributes(
			ToolAttr(toolName),
		),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	start := time.Now()
	err := executeFn(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		RecordError(ctx, toolName, "execution_error")
		RecordRequest(ctx, toolName, "execute", "error", duration)

		return err
	}

	span.SetStatus(codes.Ok, "")
	RecordRequest(ctx, toolName, "execute", "success", duration)

	return err
}

// APICall records a Linode API call with tracing and metrics.
func APICall(ctx context.Context, endpoint, method string, apiFn func(ctx context.Context) error) error {
	tracer := Tracer()

	ctx, span := tracer.Start(ctx, "linode.api.call",
		trace.WithAttributes(
			LinodeEndpointAttr(endpoint),
			LinodeMethodAttr(method),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	start := time.Now()
	err := apiFn(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		RecordAPIRequest(ctx, endpoint, method, 0, duration)

		return err
	}

	span.SetStatus(codes.Ok, "")
	RecordAPIRequest(ctx, endpoint, method, 0, duration)

	return err
}

// WithEnvironment adds the environment attribute to the context.
func WithEnvironment(ctx context.Context, env string) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(EnvironmentAttr(env))
	}

	return ctx
}

// WithToolArgument adds a tool argument as a span attribute.
func WithToolArgument(ctx context.Context, name, value string) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(ToolArgumentAttr(name, value))
	}

	return ctx
}

// WithToolResultSize adds the result size as a span attribute.
func WithToolResultSize(ctx context.Context, size int) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(ToolResultAttr(size))
	}

	return ctx
}

// RecordEvent records an event in the current span.
func RecordEvent(ctx context.Context, eventName string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(eventName, trace.WithAttributes(attrs...))
	}
}
