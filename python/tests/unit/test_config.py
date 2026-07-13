"""Unit tests for config module."""

import json
import os
from pathlib import Path
from typing import Any

import pytest
import yaml

from linodemcp.config import (
    Config,
    ConfigFileNotFoundError,
    ConfigInvalidError,
    ConfigMalformedError,
    EnvironmentConfig,
    EnvironmentNotFoundError,
    LinodeConfig,
    PathValidationError,
    get_config_dir,
    get_config_path,
    load_from_file,
    validate_config,
    validate_path,
)


def test_config_defaults() -> None:
    """Test default configuration values."""
    cfg = Config()
    assert cfg.server.name == "LinodeMCP"
    assert cfg.server.log_level == "info"
    assert cfg.server.transport == "stdio"
    assert cfg.server.port == 8080
    assert cfg.observability.metrics.prometheus.port == 8888
    assert cfg.resilience.max_retries == 3


def test_bind_host_defaults() -> None:
    """Metrics and health servers default to loopback.

    Parity with Go's TestBindHostDefaults / config.DefaultBindHost: loopback
    is the safe default because those endpoints leak operational signal, so
    remote exposure must be an explicit choice.
    """
    cfg = Config()
    assert cfg.observability.metrics.prometheus.host == "127.0.0.1"
    assert cfg.observability.health.host == "127.0.0.1"


def test_bind_host_override(tmp_path: Path) -> None:
    """An explicit host is honored so operators can expose the endpoints."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        yaml.dump(
            {
                "environments": {
                    "default": {
                        "label": "d",
                        "linode": {
                            "apiUrl": "https://api.linode.com/v4",
                            "token": "t",
                        },
                    }
                },
                "observability": {
                    "metrics": {"enabled": True, "prometheus": {"host": "192.0.2.10"}},
                    "health": {"enabled": True, "host": "192.0.2.10"},
                },
            }
        )
    )

    cfg = load_from_file(config_file)
    assert cfg.observability.metrics.prometheus.host == "192.0.2.10"
    assert cfg.observability.health.host == "192.0.2.10"


def test_tracing_defaults_match_go() -> None:
    """Tracing defaults mirror Go's TracingConfig.

    protocol grpc and insecure False keep both binaries secure by default and
    byte-identical on the shared config schema; the old Python-only exporter
    key is gone because nothing ever read it.
    """
    cfg = Config()
    tracing = cfg.observability.tracing
    assert tracing.protocol == "grpc"
    assert tracing.insecure is False
    assert tracing.sample_rate == 1.0
    assert tracing.headers == {}
    assert not hasattr(tracing, "exporter")


def test_tracing_override(tmp_path: Path) -> None:
    """protocol, insecure, and headers are wired from the config file.

    These three keys were previously honored by Go and silently dropped by
    Python; this pins the loader so the shared config file means the same
    thing to both binaries.
    """
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        yaml.dump(
            {
                "environments": {
                    "default": {
                        "label": "d",
                        "linode": {
                            "apiUrl": "https://api.linode.com/v4",
                            "token": "t",
                        },
                    }
                },
                "observability": {
                    "tracing": {
                        "enabled": True,
                        "endpoint": "collector.internal:4318",
                        "protocol": "http",
                        "insecure": True,
                        "headers": {"x-team": "infra"},
                    }
                },
            }
        )
    )

    cfg = load_from_file(config_file)
    tracing = cfg.observability.tracing
    assert tracing.protocol == "http"
    assert tracing.insecure is True
    assert tracing.headers == {"x-team": "infra"}


def _config_data_with_resilience(resilience: dict[str, Any]) -> dict[str, Any]:
    return {
        "environments": {
            "default": {
                "label": "d",
                "linode": {"apiUrl": "https://api.linode.com/v4", "token": "t"},
            }
        },
        "resilience": resilience,
    }


def test_resilience_duration_strings_load(tmp_path: Path) -> None:
    """Go duration strings are the canonical form for the shared config file
    (Go's yaml decoder rejects bare numbers for time.Duration), so Python must
    read them into seconds."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        yaml.dump(
            _config_data_with_resilience(
                {
                    "circuitBreakerTimeout": "45s",
                    "baseRetryDelay": "250ms",
                    "maxRetryDelay": "1m30s",
                }
            )
        )
    )

    cfg = load_from_file(config_file)
    assert cfg.resilience.circuit_breaker_timeout == 45.0
    assert cfg.resilience.base_retry_delay == 0.25
    assert cfg.resilience.max_retry_delay == 90.0


def test_resilience_bare_numbers_still_load(tmp_path: Path) -> None:
    """Configs written by older Python builds carried bare seconds; they keep
    loading here even though Go rejects them and the serializer no longer
    writes them."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        yaml.dump(
            _config_data_with_resilience(
                {"circuitBreakerTimeout": 45, "baseRetryDelay": 2.5}
            )
        )
    )

    cfg = load_from_file(config_file)
    assert cfg.resilience.circuit_breaker_timeout == 45.0
    assert cfg.resilience.base_retry_delay == 2.5


def test_resilience_malformed_duration_rejected(tmp_path: Path) -> None:
    """A unitless or garbage duration string fails loudly with the key name
    instead of landing as a string in a numeric field."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        yaml.dump(_config_data_with_resilience({"circuitBreakerTimeout": "banana"}))
    )

    with pytest.raises(ConfigInvalidError, match="circuitBreakerTimeout"):
        load_from_file(config_file)


def test_resilience_serializes_go_duration_strings(tmp_path: Path) -> None:
    """write_atomic emits duration strings, never bare numbers, so a config
    written by Python loads in the Go binary; the file must also round-trip
    back through the Python loader."""
    from linodemcp.config import ResilienceConfig, write_atomic

    config_file = tmp_path / "config.yml"
    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="d",
                linode=LinodeConfig(api_url="https://api.linode.com/v4", token="t"),
            )
        },
        resilience=ResilienceConfig(
            circuit_breaker_timeout=45.0,
            base_retry_delay=1.5,
            max_retry_delay=90.0,
        ),
    )
    write_atomic(config_file, cfg)

    data = yaml.safe_load(config_file.read_text())
    assert data["resilience"]["circuitBreakerTimeout"] == "45s"
    assert data["resilience"]["baseRetryDelay"] == "1.5s"
    assert data["resilience"]["maxRetryDelay"] == "90s"

    reloaded = load_from_file(config_file)
    assert reloaded.resilience.circuit_breaker_timeout == 45.0
    assert reloaded.resilience.base_retry_delay == 1.5
    assert reloaded.resilience.max_retry_delay == 90.0


def test_tracing_serializes_full_key_set(tmp_path: Path) -> None:
    """write_atomic emits the same six tracing keys Go's template writes, so
    a config file created by either binary reads identically in the other."""
    from linodemcp.config import write_atomic

    config_file = tmp_path / "config.yml"
    cfg = Config(
        environments={
            "default": EnvironmentConfig(
                label="d",
                linode=LinodeConfig(api_url="https://api.linode.com/v4", token="t"),
            )
        }
    )
    write_atomic(config_file, cfg)
    data = yaml.safe_load(config_file.read_text())
    assert set(data["observability"]["tracing"]) == {
        "enabled",
        "endpoint",
        "protocol",
        "insecure",
        "sampleRate",
        "headers",
    }


def test_load_from_file(temp_config_file: Path) -> None:
    """Test loading configuration from file."""
    cfg = load_from_file(temp_config_file)
    assert cfg.server.name == "TestLinodeMCP"
    assert cfg.server.log_level == "debug"
    assert "default" in cfg.environments
    assert cfg.environments["default"].linode.token == "test-token-123"


def test_load_from_nonexistent_file() -> None:
    """Test loading from nonexistent file raises error."""
    with pytest.raises(ConfigFileNotFoundError):
        load_from_file(Path("/nonexistent/config.yml"))


def test_load_malformed_yaml(tmp_path: Path) -> None:
    """Test loading malformed YAML raises error."""
    config_file = tmp_path / "bad.yml"
    config_file.write_text("invalid: yaml: content: [")

    with pytest.raises(ConfigMalformedError):
        load_from_file(config_file)


def test_load_json_config(tmp_path: Path, sample_config_data: dict[str, Any]) -> None:
    """Test loading JSON configuration."""
    config_file = tmp_path / "config.json"
    config_file.write_text(json.dumps(sample_config_data))

    cfg = load_from_file(config_file)
    assert cfg.server.name == "TestLinodeMCP"


def test_environment_overrides(
    tmp_path: Path, sample_config_data: dict[str, Any]
) -> None:
    """Test environment variable overrides."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(yaml.dump(sample_config_data))

    os.environ["LINODEMCP_SERVER_NAME"] = "OverriddenName"
    os.environ["LINODEMCP_LOG_LEVEL"] = "error"

    try:
        cfg = load_from_file(config_file)
        assert cfg.server.name == "OverriddenName"
        assert cfg.server.log_level == "error"
    finally:
        del os.environ["LINODEMCP_SERVER_NAME"]
        del os.environ["LINODEMCP_LOG_LEVEL"]


def test_linode_token_override(
    tmp_path: Path, sample_config_data: dict[str, Any]
) -> None:
    """Test Linode token environment override."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(yaml.dump(sample_config_data))

    os.environ["LINODEMCP_LINODE_TOKEN"] = "env-token-456"
    os.environ["LINODEMCP_LINODE_API_URL"] = "https://api.custom.com"

    try:
        cfg = load_from_file(config_file)
        assert cfg.environments["default"].linode.token == "env-token-456"
        assert cfg.environments["default"].linode.api_url == "https://api.custom.com"
    finally:
        del os.environ["LINODEMCP_LINODE_TOKEN"]
        del os.environ["LINODEMCP_LINODE_API_URL"]


def test_select_environment(sample_config: Config) -> None:
    """Test environment selection."""
    env = sample_config.select_environment("default")
    assert env.label == "Default"
    assert env.linode.token == "test-token-123"


def test_select_nonexistent_environment_falls_back(sample_config: Config) -> None:
    """Test selecting nonexistent environment falls back to default."""
    env = sample_config.select_environment("nonexistent")
    assert env.label == "Default"


def test_select_environment_empty_name_raises(sample_config: Config) -> None:
    """Test selecting environment with empty name raises error."""
    cfg = sample_config

    with pytest.raises(ValueError, match="environment name cannot be empty"):
        cfg.select_environment("")


def test_get_linode_environment(sample_config: Config) -> None:
    """Test getting Linode environment."""
    linode_cfg = sample_config.get_linode_environment("default")
    assert linode_cfg.api_url == "https://api.linode.com/v4"
    assert linode_cfg.token == "test-token-123"


def test_get_nonexistent_linode_environment(sample_config: Config) -> None:
    """Test getting nonexistent Linode environment raises error."""
    with pytest.raises(EnvironmentNotFoundError):
        sample_config.get_linode_environment("nonexistent")


def test_config_validation_missing_environments() -> None:
    """Test config validation with missing environments."""
    cfg = Config()
    cfg.environments = {}

    with pytest.raises(ConfigInvalidError, match="no environments defined"):
        validate_config(cfg)


def test_config_validation_incomplete_linode_config() -> None:
    """Test config validation with incomplete Linode config."""
    cfg = Config()
    cfg.environments = {
        "test": EnvironmentConfig(
            label="Test",
            linode=LinodeConfig(api_url="https://api.linode.com/v4", token=""),
        ),
    }

    with pytest.raises(ConfigInvalidError, match="Linode token is required"):
        validate_config(cfg)


def test_get_config_dir_default() -> None:
    """Test getting default config directory."""
    config_dir = get_config_dir()
    assert config_dir.name == "linodemcp"
    assert ".config" in config_dir.parts


def test_get_config_path_default() -> None:
    """Test getting default config path."""
    config_path = get_config_path()
    assert config_path.name in ("config.yml", "config.json")


def test_path_validation_dangerous_paths() -> None:
    """Test path validation rejects dangerous paths."""
    with pytest.raises(PathValidationError):
        validate_path(Path("/etc/passwd"))

    with pytest.raises(PathValidationError):
        validate_path(Path("/root/config.yml"))
