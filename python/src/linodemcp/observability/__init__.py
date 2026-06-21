"""Observability for LinodeMCP.

Construct an Observability instance, hold onto it, call shutdown() when done.
The module holds no global state so multiple instances can coexist (production,
test harnesses, multi-tenant). Stage 6 originally shipped a singleton/init()
pattern; that's been removed.
"""

import os
import sys
import threading
import time
from collections.abc import Callable, Generator
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, HTTPServer
from typing import Any

import structlog
from opentelemetry import metrics, trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.exporter.prometheus import PrometheusMetricReader
from opentelemetry.instrumentation.system_metrics import SystemMetricsInstrumentor
from opentelemetry.sdk.metrics import MeterProvider
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from prometheus_client import CONTENT_TYPE_LATEST, CollectorRegistry, generate_latest

from linodemcp.config import (
    HealthConfig,
    LoggingConfig,
    MetricsConfig,
    ObservabilityConfig,
    TracingConfig,
)


class Observability:
    """Bundles tracing, metrics, logging, and health endpoints.

    All state lives on the instance. Pass it explicitly to anything that needs
    observability rather than reaching for module globals.
    """

    def __init__(self, config: ObservabilityConfig | None = None) -> None:
        if config is None:
            config = ObservabilityConfig()

        self._tracer_provider: TracerProvider | None = None
        self._meter_provider: MeterProvider | None = None
        self._tracer: trace.Tracer = trace.get_tracer("linodemcp")
        self._shutdown_funcs: list[Callable[[], None]] = []
        self._health_server: HTTPServer | None = None
        self._metrics_server: HTTPServer | None = None
        self._requests_total: metrics.Counter | None = None
        self._request_duration: metrics.Histogram | None = None
        self._errors_total: metrics.Counter | None = None
        self._api_requests: metrics.Counter | None = None
        self._api_request_duration: metrics.Histogram | None = None

        self._init_logging(config.logging)
        self.logger = structlog.get_logger("linodemcp")

        if config.tracing.enabled:
            self._init_tracing(config.tracing)

        if config.metrics.enabled:
            self._init_metrics(config.metrics)

        if config.health.enabled:
            self._init_health(config.health)

    @property
    def tracer(self) -> trace.Tracer:
        """The configured OpenTelemetry tracer."""
        return self._tracer

    def shutdown(self) -> None:
        """Run all registered shutdown hooks in LIFO order. Safe to call twice."""
        if (
            not self._shutdown_funcs
            and self._tracer_provider is None
            and self._meter_provider is None
        ):
            return

        if self.logger:
            self.logger.info("shutting down observability")

        for func in reversed(self._shutdown_funcs):
            try:
                func()
            except Exception as exc:
                if self.logger:
                    self.logger.exception("shutdown error", error=str(exc))

        if self._tracer_provider is not None:
            self._tracer_provider.shutdown()
            self._tracer_provider = None

        if self._meter_provider is not None:
            self._meter_provider.shutdown()
            self._meter_provider = None

        self._shutdown_funcs.clear()

    @contextmanager
    def tool_execution(self, tool_name: str) -> Generator[Any]:
        """Trace a tool execution. Records exceptions and sets span status."""
        with self._tracer.start_as_current_span(
            "mcp.tool.execute",
            attributes={"mcp.tool.name": tool_name},
        ) as span:
            try:
                yield span
                span.set_status(trace.StatusCode.OK)
            except Exception as exc:
                span.set_status(trace.StatusCode.ERROR, str(exc))
                span.record_exception(exc)
                raise

    @contextmanager
    def api_call(self, endpoint: str, method: str) -> Generator[Any]:
        """Trace a Linode API call."""
        with self._tracer.start_as_current_span(
            "linode.api.call",
            attributes={
                "linode.api.endpoint": endpoint,
                "linode.api.method": method,
            },
        ) as span:
            try:
                yield span
                span.set_status(trace.StatusCode.OK)
            except Exception as exc:
                span.set_status(trace.StatusCode.ERROR, str(exc))
                span.record_exception(exc)
                raise

    def _init_logging(self, config: LoggingConfig) -> None:
        level_map = {"debug": 10, "info": 20, "warning": 30, "error": 40}
        log_level = level_map.get(config.level.lower(), 20)

        renderer: Any = (
            structlog.processors.JSONRenderer()
            if config.format == "json"
            else structlog.dev.ConsoleRenderer()
        )

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
            # MCP stdio reserves stdout for the JSON-RPC stream; logs must
            # go to stderr or they corrupt the protocol the client reads.
            logger_factory=structlog.PrintLoggerFactory(file=sys.stderr),
            cache_logger_on_first_use=True,
        )

    def _init_tracing(self, config: TracingConfig) -> None:
        try:
            otlp_endpoint = os.getenv(
                "OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317"
            )
            endpoint = config.endpoint or otlp_endpoint
            sampler_arg = os.getenv("OTEL_TRACES_SAMPLER_ARG", "1.0")
            sample_rate = config.sample_rate or float(sampler_arg)

            resource = Resource.create(
                {"service.name": "linodemcp", "service.version": _get_version()}
            )

            sampler = None
            if sample_rate < 1.0:
                from opentelemetry.sdk.trace.sampling import (  # noqa: PLC0415
                    ParentBased,
                    TraceIdRatioBased,
                )

                sampler = ParentBased(TraceIdRatioBased(sample_rate))

            self._tracer_provider = TracerProvider(resource=resource, sampler=sampler)

            if endpoint:
                exporter = OTLPSpanExporter(
                    endpoint=endpoint,
                    insecure=config.insecure,
                    headers=config.headers,
                )
                self._tracer_provider.add_span_processor(BatchSpanProcessor(exporter))

            trace.set_tracer_provider(self._tracer_provider)
            self._tracer = trace.get_tracer("linodemcp")
        except Exception as exc:
            if self.logger:
                self.logger.exception("failed to initialize tracing", error=str(exc))

    def _init_metrics(self, config: MetricsConfig) -> None:
        try:
            resource = Resource.create(
                {"service.name": "linodemcp", "service.version": _get_version()}
            )

            readers: list[PrometheusMetricReader] = []
            registry: CollectorRegistry | None = None

            if config.prometheus.enabled and config.prometheus.port > 0:
                # A per-instance registry (not the global default) keeps
                # multiple Observability instances, and parallel tests, from
                # colliding on duplicate collector registration. The reader
                # bridges the instruments to the scrape endpoint; without it
                # the meter provider records into nothing exposed.
                registry = CollectorRegistry()
                readers.append(PrometheusMetricReader(registry=registry))

            self._meter_provider = MeterProvider(
                resource=resource, metric_readers=readers
            )
            metrics.set_meter_provider(self._meter_provider)

            meter = self._meter_provider.get_meter("github.com/chadit/LinodeMCP")
            self._requests_total = meter.create_counter(
                "linodemcp.requests.total",
                unit="1",
                description="Total number of MCP requests",
            )
            self._request_duration = meter.create_histogram(
                "linodemcp.request.duration.seconds",
                unit="s",
                description="Duration of MCP requests in seconds",
            )
            self._errors_total = meter.create_counter(
                "linodemcp.errors.total",
                unit="1",
                description="Total number of MCP errors",
            )
            self._api_requests = meter.create_counter(
                "linodemcp.api.requests.total",
                unit="1",
                description="Total number of Linode API requests",
            )
            self._api_request_duration = meter.create_histogram(
                "linodemcp.api.request.duration.seconds",
                unit="s",
                description="Duration of Linode API requests in seconds",
            )

            if registry is not None:
                self._start_metrics_server(
                    config.prometheus.host,
                    config.prometheus.port,
                    config.prometheus.path,
                    registry,
                )

            if config.host:
                try:
                    SystemMetricsInstrumentor().instrument()
                except Exception as exc:
                    if self.logger:
                        self.logger.exception(
                            "failed to start host metrics", error=str(exc)
                        )
        except Exception as exc:
            if self.logger:
                self.logger.exception("failed to initialize metrics", error=str(exc))

    def _start_metrics_server(
        self, host: str, port: int, path: str, registry: CollectorRegistry
    ) -> None:
        metrics_path = path

        class MetricsHandler(BaseHTTPRequestHandler):
            def log_message(self, format: str, *args: Any) -> None:  # noqa: A002 - signature must match BaseHTTPRequestHandler.log_message
                """Suppress default request logging."""

            def do_GET(self) -> None:
                if self.path != metrics_path:
                    self.send_response(404)
                    self.end_headers()
                    return

                output = generate_latest(registry)
                self.send_response(200)
                self.send_header("Content-Type", CONTENT_TYPE_LATEST)
                self.end_headers()
                self.wfile.write(output)

        self._metrics_server = HTTPServer((host, port), MetricsHandler)

        thread = threading.Thread(
            target=self._metrics_server.serve_forever, daemon=True
        )
        thread.start()

        if self.logger:
            self.logger.info("metrics server started", port=port, path=path)

        def _shutdown_metrics() -> None:
            if self._metrics_server is not None:
                self._metrics_server.shutdown()
                self._metrics_server = None

        self._shutdown_funcs.append(_shutdown_metrics)

    def record_tool_call(self, tool: str, duration_seconds: float, error: bool) -> None:
        """Record metrics for a tool dispatch that already ran.

        The request total and duration always move; an execution-error count
        moves only when error is True. The non-recording observability path
        leaves the instruments None, so this is a no-op until metrics init.
        """
        if self._requests_total is None or self._request_duration is None:
            return

        status = "error" if error else "success"
        self._requests_total.add(
            1, {"tool": tool, "method": "execute", "status": status}
        )

        if duration_seconds > 0:
            self._request_duration.record(
                duration_seconds, {"tool": tool, "method": "execute"}
            )

        if error and self._errors_total is not None:
            self._errors_total.add(1, {"tool": tool, "error_type": "execution_error"})

    def record_api_request(
        self, endpoint: str, method: str, status: int, duration_seconds: float
    ) -> None:
        """Record metrics for a completed Linode API request."""
        if self._api_requests is None or self._api_request_duration is None:
            return

        self._api_requests.add(
            1, {"endpoint": endpoint, "method": method, "status_code": status}
        )

        if duration_seconds > 0:
            self._api_request_duration.record(
                duration_seconds, {"endpoint": endpoint, "method": method}
            )

    def _init_health(self, config: HealthConfig) -> None:
        try:

            class HealthHandler(BaseHTTPRequestHandler):
                def log_message(self, format: str, *args: Any) -> None:  # noqa: A002 - signature must match BaseHTTPRequestHandler.log_message
                    """Suppress default request logging."""

                def do_GET(self) -> None:
                    paths = {
                        config.path + "/live",
                        config.path + "/ready",
                        config.path + "/healthz",
                    }
                    if self.path in paths:
                        self.send_response(200)
                        self.send_header("Content-Type", "application/json")
                        self.end_headers()
                        body = (
                            '{"status": "healthy", "timestamp": "'
                            + str(time.time())
                            + '"}'
                        )
                        self.wfile.write(body.encode())
                    else:
                        self.send_response(404)
                        self.end_headers()

            self._health_server = HTTPServer((config.host, config.port), HealthHandler)

            thread = threading.Thread(
                target=self._health_server.serve_forever, daemon=True
            )
            thread.start()

            if self.logger:
                self.logger.info(
                    "health server started", port=config.port, path=config.path
                )

            def _shutdown_health() -> None:
                if self._health_server is not None:
                    self._health_server.shutdown()
                    self._health_server = None

            self._shutdown_funcs.append(_shutdown_health)
        except Exception as exc:
            if self.logger:
                self.logger.exception("failed to start health server", error=str(exc))


def _get_version() -> str:
    """Application version, env-overridable for tests and packaging."""
    return os.getenv("LINODEMCP_VERSION", "dev")
