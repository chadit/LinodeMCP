"""Tests for the observability package."""

import sys
from unittest.mock import MagicMock

import pytest

# Mock opentelemetry imports before importing observability module
sys.modules["opentelemetry"] = MagicMock()
sys.modules["opentelemetry.metrics"] = MagicMock()
sys.modules["opentelemetry.trace"] = MagicMock()
sys.modules["opentelemetry.exporter"] = MagicMock()
sys.modules["opentelemetry.exporter.otlp"] = MagicMock()
sys.modules["opentelemetry.exporter.otlp.proto"] = MagicMock()
sys.modules["opentelemetry.exporter.otlp.proto.grpc"] = MagicMock()
sys.modules["opentelemetry.exporter.otlp.proto.grpc.trace_exporter"] = MagicMock()
sys.modules["opentelemetry.exporter.prometheus"] = MagicMock()
sys.modules["prometheus_client"] = MagicMock()
sys.modules["opentelemetry.sdk"] = MagicMock()
sys.modules["opentelemetry.sdk.metrics"] = MagicMock()
sys.modules["opentelemetry.sdk.resources"] = MagicMock()
sys.modules["opentelemetry.sdk.trace"] = MagicMock()
sys.modules["opentelemetry.sdk.trace.export"] = MagicMock()
sys.modules["opentelemetry.instrumentation"] = MagicMock()
sys.modules["opentelemetry.instrumentation.system_metrics"] = MagicMock()

from linodemcp.config import (  # noqa: E402 - imports after sys.modules mocking
    HealthConfig,
    LoggingConfig,
    MetricsConfig,
    ObservabilityConfig,
    PrometheusConfig,
    TracingConfig,
)
from linodemcp.observability import (  # noqa: E402 - imports after sys.modules mocking
    Observability,
)


def _make_obs() -> Observability:
    """Construct an Observability with all subsystems off so tests don't open ports."""
    return Observability(
        ObservabilityConfig(
            tracing=TracingConfig(enabled=False),
            metrics=MetricsConfig(enabled=False),
            health=HealthConfig(enabled=False),
            logging=LoggingConfig(level="info", format="json"),
        )
    )


def test_metrics_recording_drives_instruments() -> None:
    """Enabling metrics builds instruments; record_* drive them without error.

    opentelemetry is mocked at module level, so this exercises _init_metrics and
    the record methods against MagicMock instruments rather than a live meter.
    The real end-to-end export is exercised by the runtime smoke and the Go
    scrape test (TestPrometheusEndpointExposesApplicationMetrics); a Python
    in-process scrape test can't coexist with this module's global mock.
    prometheus.enabled=False keeps the metrics HTTP server from binding a port.
    """
    obs = Observability(
        ObservabilityConfig(
            tracing=TracingConfig(enabled=False),
            metrics=MetricsConfig(
                enabled=True,
                host=False,
                runtime=False,
                prometheus=PrometheusConfig(enabled=False),
            ),
            health=HealthConfig(enabled=False),
            logging=LoggingConfig(level="error", format="json"),
        )
    )
    try:
        # The mocked meter yields non-None instruments, so the recording
        # branches run (not the None-guard early return); these must not raise.
        obs.record_tool_call("hello", 0.01, error=False)
        obs.record_tool_call("boom", 0.02, error=True)
        obs.record_api_request("/regions", "GET", 200, 0.03)
    finally:
        obs.shutdown()

    # Metrics disabled: instruments stay None, so record_* take the guarded
    # no-op path and must also not raise. This is the behavior asserted
    # directly here; correct label/value EXPORT is verified by the Go scrape
    # test (same OTel->Prometheus mechanism) and the runtime smoke, since this
    # module's global opentelemetry mock blocks an in-process scrape assertion.
    disabled = Observability(
        ObservabilityConfig(
            tracing=TracingConfig(enabled=False),
            metrics=MetricsConfig(enabled=False),
            health=HealthConfig(enabled=False),
            logging=LoggingConfig(level="error", format="json"),
        )
    )
    try:
        disabled.record_tool_call("hello", 0.01, error=False)
        disabled.record_tool_call("boom", 0.02, error=True)
        disabled.record_api_request("/regions", "GET", 0, 0.03)
    finally:
        disabled.shutdown()


class TestConstruction:
    """Constructor accepts disabled config and None."""

    def test_construct_with_disabled_components(self) -> None:
        obs = _make_obs()
        try:
            assert obs.logger is not None
            assert obs.tracer is not None
        finally:
            obs.shutdown()

    def test_construct_with_none_config(self) -> None:
        obs = Observability(None)
        try:
            assert obs.logger is not None
        finally:
            obs.shutdown()


class TestShutdown:
    """Shutdown is idempotent and safe to call twice."""

    def test_shutdown_runs(self) -> None:
        obs = _make_obs()
        obs.shutdown()
        # Second call must not raise.
        obs.shutdown()


class TestToolExecution:
    """tool_execution context manager."""

    def test_tool_execution_success(self) -> None:
        obs = _make_obs()
        try:
            with obs.tool_execution("test_tool") as span:
                assert span is not None
        finally:
            obs.shutdown()

    def test_tool_execution_failure(self) -> None:
        obs = _make_obs()
        try:
            with pytest.raises(ValueError, match="test error"):  # noqa: SIM117
                with obs.tool_execution("test_tool"):
                    raise ValueError("test error")
        finally:
            obs.shutdown()


class TestAPICall:
    """api_call context manager."""

    def test_api_call_success(self) -> None:
        obs = _make_obs()
        try:
            with obs.api_call("/v4/test", "GET") as span:
                assert span is not None
        finally:
            obs.shutdown()

    def test_api_call_failure(self) -> None:
        obs = _make_obs()
        try:
            with pytest.raises(RuntimeError, match="API error"):  # noqa: SIM117
                with obs.api_call("/v4/test", "POST"):
                    raise RuntimeError("API error")
        finally:
            obs.shutdown()


class TestIndependence:
    """Two Observability instances do not share state."""

    def test_two_instances_independent(self) -> None:
        a = _make_obs()
        b = _make_obs()
        try:
            assert a is not b
            assert a.logger is not None
            assert b.logger is not None
        finally:
            a.shutdown()
            b.shutdown()
