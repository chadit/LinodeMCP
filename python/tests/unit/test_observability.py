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
sys.modules["opentelemetry.instrumentation.runtime"] = MagicMock()
sys.modules["opentelemetry.instrumentation.system_metrics"] = MagicMock()

from linodemcp.config import (  # noqa: E402 - imports after sys.modules mocking
    HealthConfig,
    LoggingConfig,
    MetricsConfig,
    ObservabilityConfig,
    TracingConfig,
)
from linodemcp.observability import (  # noqa: E402 - imports after sys.modules mocking
    api_call,
    get_logger,
    get_tracer,
    init,
    shutdown,
    tool_execution,
)


class TestInit:
    """Tests for init() function."""

    def test_init_with_disabled_components(self) -> None:
        """Test init with all components disabled."""
        cfg = ObservabilityConfig(
            tracing=TracingConfig(enabled=False),
            metrics=MetricsConfig(enabled=False),
            health=HealthConfig(enabled=False),
            logging=LoggingConfig(level="info", format="json"),
        )

        # Should not raise
        init(cfg)

    def test_init_with_nil_config(self) -> None:
        """Test init with None config uses defaults."""
        # Should not raise
        init(None)


class TestShutdown:
    """Tests for shutdown() function."""

    def test_shutdown(self) -> None:
        """Test shutdown completes without error."""
        # Should not raise
        shutdown()


class TestGetLogger:
    """Tests for get_logger() function."""

    def test_get_logger_returns_logger(self) -> None:
        """Test get_logger returns a valid logger."""
        logger = get_logger()
        assert logger is not None


class TestGetTracer:
    """Tests for get_tracer() function."""

    def test_get_tracer_returns_tracer(self) -> None:
        """Test get_tracer returns a valid tracer."""
        tracer = get_tracer()
        assert tracer is not None


class TestToolExecution:
    """Tests for tool_execution() context manager."""

    def test_tool_execution_success(self) -> None:
        """Test tool_execution with successful function."""
        with tool_execution("test_tool") as span:
            assert span is not None

    def test_tool_execution_failure(self) -> None:
        """Test tool_execution with failing function."""
        # Note: nested with required - pytest.raises must wrap context manager
        with pytest.raises(ValueError, match="test error"):  # noqa: SIM117
            with tool_execution("test_tool"):
                raise ValueError("test error")


class TestAPICall:
    """Tests for api_call() context manager."""

    def test_api_call_success(self) -> None:
        """Test api_call with successful function."""
        with api_call("/v4/test", "GET") as span:
            assert span is not None

    def test_api_call_failure(self) -> None:
        """Test api_call with failing function."""
        # Note: nested with required - pytest.raises must wrap context manager
        with pytest.raises(RuntimeError, match="API error"):  # noqa: SIM117
            with api_call("/v4/test", "POST"):
                raise RuntimeError("API error")
