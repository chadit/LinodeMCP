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
    assert cfg.metrics.port == 9090
    assert cfg.resilience.max_retries == 3


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
