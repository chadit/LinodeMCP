package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ToolExecution wraps tool execution with tracing and metric recording.
func (o *Observability) ToolExecution(ctx context.Context, toolName string, executeFn func(ctx context.Context) error) error {
	ctx, span := o.tracer.Start(
		ctx, "mcp.tool.execute",
		trace.WithAttributes(ToolAttr(toolName)),
		trace.WithSpanKind(trace.SpanKindServer),
	)
	defer span.End()

	start := time.Now()
	err := executeFn(ctx)
	duration := time.Since(start).Seconds()

	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		o.RecordError(ctx, toolName, "execution_error")
		o.RecordRequest(ctx, toolName, "execute", "error", duration)

		return err
	}

	span.SetStatus(codes.Ok, "")
	o.RecordRequest(ctx, toolName, "execute", "success", duration)

	return nil
}

// APICall wraps a Linode API call with tracing and metric recording.
func (o *Observability) APICall(ctx context.Context, endpoint, method string, apiFn func(ctx context.Context) error) error {
	ctx, span := o.tracer.Start(
		ctx, "linode.api.call",
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
		o.RecordAPIRequest(ctx, endpoint, method, 0, duration)

		return err
	}

	span.SetStatus(codes.Ok, "")
	o.RecordAPIRequest(ctx, endpoint, method, 0, duration)

	return nil
}

// Span helpers below are pure context-mutation utilities. They operate only
// on the span carried by ctx, so no observability state is needed.

// WithEnvironment annotates the current span with the environment name.
func WithEnvironment(ctx context.Context, env string) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(EnvironmentAttr(env))
	}

	return ctx
}

// WithToolArgument annotates the current span with a tool argument.
func WithToolArgument(ctx context.Context, name, value string) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(ToolArgumentAttr(name, value))
	}

	return ctx
}

// WithToolResultSize annotates the current span with the tool result size.
func WithToolResultSize(ctx context.Context, size int) context.Context {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.SetAttributes(ToolResultAttr(size))
	}

	return ctx
}

// RecordEvent adds an event to the current span.
func RecordEvent(ctx context.Context, eventName string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if span.IsRecording() {
		span.AddEvent(eventName, trace.WithAttributes(attrs...))
	}
}
