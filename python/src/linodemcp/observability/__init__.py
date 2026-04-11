"""Observability package for LinodeMCP.

Provides tracing, metrics, logging, and health check functionality
using OpenTelemetry and Prometheus.
"""

import atexit
import os
import threading
import time
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

import structlog
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.runtime import RuntimeInstrumentor
from opentelemetry.instrumentation.system_metrics import SystemMetricsInstrumentor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

from linodemcp.config import (
    HealthConfig,
    LoggingConfig,
    MetricsConfig,
    ObservabilityConfig,
    TracingConfig,
)

_logger = structlog.get_logger(__name__)

# Global state
_tracer_provider: TracerProvider | None = None
_meter_provider: MeterProvider | None = None
_tracer: trace.Tracer | None = None
_logger_instance: Any = None
_shutdown_funcs: list[Callable[[], None]] = []
_init_state: dict[str, bool] = {"initialized": False}


def init(config: ObservabilityConfig | None = None) -> None:
    """Initialize all observability components."""
    # Idempotency check - don't initialize twice
    if _init_state["initialized"]:
        return

    if config is None:
        config = ObservabilityConfig()

    # Initialize logging
    _init_logging(config.logging)

    # Initialize tracing
    if config.tracing.enabled:
        _init_tracing(config.tracing)

    # Initialize metrics
    if config.metrics.enabled:
        _init_metrics(config.metrics)

    # Initialize health
    if config.health.enabled:
        _init_health(config.health)

    # Register shutdown
    atexit.register(shutdown)

    _init_state["initialized"] = True


def shutdown() -> None:
    """Gracefully shutdown observability components."""
    if _logger_instance:
        _logger_instance.info("shutting down observability")

    for func in reversed(_shutdown_funcs):
        try:
            func()
        except Exception as e:
            if _logger_instance:
                _logger_instance.error("shutdown error", error=str(e))

    if _tracer_provider:
        _tracer_provider.shutdown()

    if _meter_provider:
        _meter_provider.shutdown()


def get_logger() -> Any:
    """Get the configured logger."""
    if _logger_instance is None:
        return structlog.get_logger()
    return _logger_instance


def get_tracer() -> trace.Tracer:
    """Get the configured tracer."""
    if _tracer is None:
        return trace.get_tracer(__name__)
    return _tracer


def _init_logging(config: LoggingConfig) -> None:
    """Configure structlog with JSON or text output."""
    level_map = {
        "debug": 10,
        "info": 20,
        "warning": 30,
        "error": 40,
    }
    log_level = level_map.get(config.level.lower(), 20)

    renderer: Any
    if config.format == "json":
        renderer = structlog.processors.JSONRenderer()
    else:
        renderer = structlog.dev.ConsoleRenderer()

    structlog.configure(
        processors=[
            structlog.contextvars.merge_contextvars,
            structlog.processors.add_log_level,
            structlog.processors.StackInfoRenderer(),
            structlog.processors.TimeStamper(fmt="iso"),
            renderer,
        ],
        wrapper_class=structlog.make_filtering_bound_logger(log_level),
        context_class=dict,
        logger_factory=structlog.PrintLoggerFactory(),
        cache_logger_on_first_use=True,
    )


def _init_tracing(config: TracingConfig) -> None:
    """Initialize OpenTelemetry tracing."""
    global _tracer_provider  # noqa: PLW0603 - Singleton pattern

    try:
        # Apply OTEL environment variables
        otlp_endpoint = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
        endpoint = config.endpoint or otlp_endpoint
        sampler_arg = os.getenv("OTEL_TRACES_SAMPLER_ARG", "1.0")
        sample_rate = config.sample_rate or float(sampler_arg)

        resource = Resource.create({
            "service.name": "linodemcp",
            "service.version": _get_version(),
        })

        # Configure sampler
        sampler = None
        if sample_rate < 1.0:
            from opentelemetry.sdk.trace.sampling import (  # noqa: PLC0415
                ParentBased,
                TraceIdRatioBased,
            )
            sampler = ParentBased(TraceIdRatioBased(sample_rate))

        _tracer_provider = TracerProvider(
            resource=resource,
            sampler=sampler,
        )

        # Add OTLP exporter
        if endpoint:
            exporter = OTLPSpanExporter(
                endpoint=endpoint,
                insecure=config.insecure,
                headers=config.headers,
            )
            _tracer_provider.add_span_processor(
                BatchSpanProcessor(exporter)
            )

        trace.set_tracer_provider(_tracer_provider)

        def _shutdown_tracer() -> None:
            _tracer_provider.shutdown()

        _shutdown_funcs.append(_shutdown_tracer)

        if _logger_instance:
            _logger_instance.info("tracing initialized")
    except Exception as e:
        if _logger_instance:
            _logger_instance.error("failed to initialize tracing", error=str(e))
        # Continue without tracing


def _init_metrics(config: MetricsConfig) -> None:
    """Initialize OpenTelemetry metrics."""
    global _meter_provider  # noqa: PLW0603 - Singleton pattern

    try:
        resource = Resource.create({
            "service.name": "linodemcp",
            "service.version": _get_version(),
        })

        _meter_provider = MeterProvider(resource=resource)

        # Start runtime metrics (threads, memory)
        if config.runtime:
            try:
                RuntimeInstrumentor().instrument()
            except Exception as e:
                if _logger_instance:
                    _logger_instance.error(
                        "failed to start runtime metrics", error=str(e)
                    )

        # Start host/system metrics (CPU, memory, network)
        if config.host:
            try:
                SystemMetricsInstrumentor().instrument()
            except Exception as e:
                if _logger_instance:
                    _logger_instance.error(
                        "failed to start host metrics", error=str(e)
                    )

        metrics.set_meter_provider(_meter_provider)

        def _shutdown_meter() -> None:
            _meter_provider.shutdown()

        _shutdown_funcs.append(_shutdown_meter)

        if _logger_instance:
            _logger_instance.info("metrics initialized")
    except Exception as e:
        if _logger_instance:
            _logger_instance.error("failed to initialize metrics", error=str(e))
        # Continue without metrics


def _init_health(config: HealthConfig) -> None:
    """Initialize health check endpoints.

    Starts an HTTP server with /live, /ready, and /healthz endpoints.
    """
    # Use closure to capture health_server reference
    health_server_ref: list[Any] = [None]

    try:
        class HealthHandler(BaseHTTPRequestHandler):
            """HTTP handler for health check endpoints."""

            def log_message(self, fmt: str, *args: Any) -> None:
                """Suppress default logging."""

            def do_GET(self) -> None:
                """Handle GET requests for health endpoints."""
                live_path = config.path + "/live"
                ready_path = config.path + "/ready"
                healthz_path = config.path + "/healthz"

                if self.path in [live_path, ready_path, healthz_path]:
                    self.send_response(200)
                    self.send_header("Content-Type", "application/json")
                    self.end_headers()
                    timestamp = str(time.time())
                    response = '{"status": "healthy", "timestamp": "' + timestamp + '"}'
                    self.wfile.write(response.encode())
                else:
                    self.send_response(404)
                    self.end_headers()

        health_server_ref[0] = HTTPServer(("127.0.0.1", config.port), HealthHandler)

        # Run server in background thread
        server_thread = threading.Thread(
            target=health_server_ref[0].serve_forever,
            daemon=True,
        )
        server_thread.start()

        if _logger_instance:
            _logger_instance.info(
                "health server started",
                port=config.port,
                path=config.path,
            )

        def _shutdown_health() -> None:
            if health_server_ref[0]:
                health_server_ref[0].shutdown()

        _shutdown_funcs.append(_shutdown_health)
    except Exception as e:
        if _logger_instance:
            _logger_instance.error("failed to start health server", error=str(e))
        # Continue without health endpoint

def _get_version() -> str:
    """Get application version."""
    return os.getenv("LINODEMCP_VERSION", "dev")


@contextmanager
def tool_execution(tool_name: str) -> Iterator[Any]:
    """Context manager for tool execution tracing."""
    tracer = get_tracer()
    with tracer.start_as_current_span(
        "mcp.tool.execute",
        attributes={"mcp.tool.name": tool_name},
    ) as span:
        try:
            yield span
            span.set_status(trace.StatusCode.OK)
        except Exception as e:
            span.set_status(trace.StatusCode.ERROR, str(e))
            span.record_exception(e)
            raise


@contextmanager
def api_call(endpoint: str, method: str) -> Iterator[Any]:
    """Context manager for API call tracing."""
    tracer = get_tracer()
    with tracer.start_as_current_span(
        "linode.api.call",
        attributes={
            "linode.api.endpoint": endpoint,
            "linode.api.method": method,
        },
    ) as span:
        try:
            yield span
            span.set_status(trace.StatusCode.OK)
        except Exception as e:
            span.set_status(trace.StatusCode.ERROR, str(e))
            span.record_exception(e)
            raise
