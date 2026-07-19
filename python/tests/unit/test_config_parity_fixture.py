"""Shared-config parity fixture test.

Loads ``testdata/config/parity.yml``, the same file the Go suite loads
(go/internal/config/parity_fixture_test.go), and asserts the same parsed value
for every field. The two tests pin identical expectations, so a loader that
reads a shared key differently from the other implementation fails here
instead of surfacing later as a config-file incompatibility.
"""

from pathlib import Path

import pytest

from linodemcp.config import load_from_file

_PARITY_FIXTURE = (
    Path(__file__).resolve().parents[3] / "testdata" / "config" / "parity.yml"
)

# Env overrides both loaders honor (the docs/contracts/env-vars.txt surface;
# observability has none by design); blanked so a developer shell with
# LINODEMCP_* set cannot change what the fixture parses to. An empty value is
# treated as unset by both loaders.
_OVERRIDE_ENV_VARS = (
    "LINODEMCP_SERVER_NAME",
    "LINODEMCP_LOG_LEVEL",
    "LINODEMCP_LINODE_API_URL",
    "LINODEMCP_LINODE_TOKEN",
)


def test_shared_config_parity_fixture(monkeypatch: pytest.MonkeyPatch) -> None:
    """Every shared config key parses to the value the Go test asserts."""
    for name in _OVERRIDE_ENV_VARS:
        monkeypatch.setenv(name, "")

    cfg = load_from_file(_PARITY_FIXTURE)

    assert cfg.server.name == "ParityCheck"
    assert cfg.server.log_level == "debug"
    assert cfg.server.transport == "stdio"
    assert cfg.server.host == "127.0.0.2"
    assert cfg.server.port == 8180

    metrics = cfg.observability.metrics
    assert metrics.enabled is True
    assert metrics.runtime is False
    assert metrics.host is True
    assert metrics.prometheus.enabled is True
    assert metrics.prometheus.host == "192.0.2.7"
    assert metrics.prometheus.port == 9101
    assert metrics.prometheus.path == "/parity-metrics"

    tracing = cfg.observability.tracing
    assert tracing.enabled is True
    assert tracing.endpoint == "collector.example.internal:4317"
    assert tracing.protocol == "http"
    assert tracing.insecure is True
    assert tracing.sample_rate == 0.25
    assert tracing.headers == {"x-parity": "check"}

    logging_cfg = cfg.observability.logging
    assert logging_cfg.level == "warn"
    assert logging_cfg.format == "text"

    health = cfg.observability.health
    assert health.enabled is True
    assert health.host == "192.0.2.8"
    assert health.port == 9102
    assert health.path == "/parity-health"

    res = cfg.resilience
    assert res.rate_limit_per_minute == 500
    assert res.circuit_breaker_threshold == 7
    assert res.circuit_breaker_timeout == 45.0
    assert res.max_retries == 2
    assert res.base_retry_delay == 0.25
    assert res.max_retry_delay == 90.0

    env = cfg.environments["default"]
    assert env.label == "Parity"
    assert env.linode.api_url == "https://api.linode.com/v4"
    assert env.linode.token == "parity-test-token"
