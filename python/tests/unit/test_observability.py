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
