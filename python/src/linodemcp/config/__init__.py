"""Configuration management for LinodeMCP."""

import json
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

import yaml


class ConfigError(Exception):
    """Base configuration error."""


class ConfigFileNotFoundError(ConfigError):
    """Configuration file not found."""


class ConfigInvalidError(ConfigError):
    """Configuration is invalid."""


class ConfigMalformedError(ConfigError):
    """Configuration file is malformed."""


class EnvironmentNotFoundError(ConfigError):
    """Environment not found in configuration."""


class PathValidationError(ConfigError):
    """Path validation failed."""


@dataclass
class ServerConfig:
    """Core server settings."""

    name: str = "LinodeMCP"
    log_level: str = "info"
    transport: str = "stdio"
    host: str = "127.0.0.1"
    port: int = 8080


@dataclass
class MetricsConfig:
    """Prometheus metrics settings."""

    enabled: bool = True
    port: int = 9090
    path: str = "/metrics"


@dataclass
class TracingConfig:
    """OpenTelemetry tracing settings."""

    enabled: bool = False
    exporter: str = "otlp"
    endpoint: str = "localhost:4317"
    sample_rate: float = 1.0


@dataclass
class ResilienceConfig:
    """Retry, rate limit, and circuit breaker settings."""

    rate_limit_per_minute: int = 700
    circuit_breaker_threshold: int = 5
    circuit_breaker_timeout: int = 30
    max_retries: int = 3
    base_retry_delay: int = 1
    max_retry_delay: int = 30


@dataclass
class LinodeConfig:
    """Linode API settings."""

    api_url: str = ""
    token: str = ""


@dataclass
class EnvironmentConfig:
    """Settings for a named environment."""

    label: str = ""
    linode: LinodeConfig = field(default_factory=LinodeConfig)


@dataclass
class Config:
    """Full LinodeMCP configuration."""

    server: ServerConfig = field(default_factory=ServerConfig)
    metrics: MetricsConfig = field(default_factory=MetricsConfig)
    tracing: TracingConfig = field(default_factory=TracingConfig)
    resilience: ResilienceConfig = field(default_factory=ResilienceConfig)
    environments: dict[str, EnvironmentConfig] = field(default_factory=dict)

    def select_environment(self, user_input: str) -> EnvironmentConfig:
        """Select a Linode environment from the config."""
        if not user_input or not user_input.strip():
            msg = "environment name cannot be empty"
            raise ValueError(msg)

        if not self.environments:
            msg = "no provider environments configured"
            raise EnvironmentNotFoundError(msg)

        user_input_lower = user_input.strip().lower()
        for env_name, env in self.environments.items():
            if env_name.lower() == user_input_lower:
                return env

        if "default" in self.environments:
            return self.environments["default"]

        return next(iter(self.environments.values()))

    def get_linode_environment(self, environment_name: str) -> LinodeConfig:
        """Get the LinodeConfig for a named environment."""
        if not self.environments:
            msg = "no provider environments configured"
            raise EnvironmentNotFoundError(msg)

        if environment_name not in self.environments:
            msg = f"environment '{environment_name}' not found"
            raise EnvironmentNotFoundError(msg)

        return self.environments[environment_name].linode


def _validate_path(path: Path) -> None:
    """Validate file path for security."""
    if not path:
        msg = "path cannot be empty"
        raise PathValidationError(msg)

    dangerous_paths = [
        "/etc/",
        "/root/",
        "/proc/",
        "/sys/",
        "/dev/",
        "/bin/",
        "/sbin/",
        "/usr/bin/",
        "/usr/sbin/",
        "/boot/",
        "/var/log/",
        "/var/run/",
    ]

    str_path = str(path)
    resolved_path = str(path.resolve())
    for dangerous in dangerous_paths:
        if str_path.startswith(dangerous) or resolved_path.startswith(dangerous):
            msg = f"path contains dangerous elements: {path}"
            raise PathValidationError(msg)

    try:
        resolved = path.resolve()
        if ".." in resolved.parts:
            msg = f"path contains directory traversal: {path}"
            raise PathValidationError(msg)
    except (OSError, RuntimeError) as e:
        msg = f"failed to resolve path: {path}"
        raise PathValidationError(msg) from e


def get_config_dir() -> Path:
    """Get configuration directory path."""
    custom_path = os.getenv("LINODEMCP_CONFIG_PATH")
    if custom_path:
        try:
            path = Path(custom_path)
            _validate_path(path)
        except PathValidationError:
            pass
        else:
            return path.parent

    home_dir = Path.home()
    return home_dir / ".config" / "linodemcp"


def get_config_path() -> Path:
    """Get configuration file path."""
    custom_path = os.getenv("LINODEMCP_CONFIG_PATH")
    if custom_path:
        try:
            path = Path(custom_path)
            _validate_path(path)
        except PathValidationError:
            pass
        else:
            return path

    config_dir = get_config_dir()
    json_path = config_dir / "config.json"
    if json_path.exists():
        return json_path

    return config_dir / "config.yml"


def _parse_config_data(data: str) -> dict[str, Any]:
    """Parse configuration data from JSON or YAML."""
    data_stripped = data.strip()
    if data_stripped.startswith("{"):
        try:
            return json.loads(data_stripped)  # type: ignore[no-any-return]
        except json.JSONDecodeError:
            pass

    try:
        return yaml.safe_load(data_stripped)  # type: ignore[no-any-return]
    except yaml.YAMLError as e:
        msg = f"failed to parse YAML: {e}"
        raise ConfigMalformedError(msg) from e


def _apply_defaults(data: dict[str, Any]) -> None:
    """Apply default values to configuration."""
    data.setdefault("server", {})
    data["server"].setdefault("name", "LinodeMCP")
    data["server"].setdefault("logLevel", "info")
    data["server"].setdefault("transport", "stdio")
    data["server"].setdefault("host", "127.0.0.1")
    data["server"].setdefault("port", 8080)

    data.setdefault("metrics", {})
    data["metrics"].setdefault("enabled", True)
    data["metrics"].setdefault("port", 9090)
    data["metrics"].setdefault("path", "/metrics")

    data.setdefault("tracing", {})
    data["tracing"].setdefault("enabled", False)
    data["tracing"].setdefault("exporter", "otlp")
    data["tracing"].setdefault("endpoint", "localhost:4317")
    data["tracing"].setdefault("sampleRate", 1.0)

    data.setdefault("resilience", {})
    data["resilience"].setdefault("rateLimitPerMinute", 700)
    data["resilience"].setdefault("circuitBreakerThreshold", 5)
    data["resilience"].setdefault("circuitBreakerTimeout", 30)
    data["resilience"].setdefault("maxRetries", 3)
    data["resilience"].setdefault("baseRetryDelay", 1)
    data["resilience"].setdefault("maxRetryDelay", 30)


def _apply_environment_overrides(data: dict[str, Any]) -> None:
    """Apply environment variable overrides."""
    if server_name := os.getenv("LINODEMCP_SERVER_NAME"):
        data.setdefault("server", {})
        data["server"]["name"] = server_name

    if log_level := os.getenv("LINODEMCP_LOG_LEVEL"):
        data.setdefault("server", {})
        data["server"]["logLevel"] = log_level

    data.setdefault("environments", {})
    data["environments"].setdefault("default", {})

    api_url = os.getenv("LINODEMCP_LINODE_API_URL")
    token = os.getenv("LINODEMCP_LINODE_TOKEN")

    if api_url or token:
        data["environments"]["default"].setdefault("linode", {})
        if api_url:
            data["environments"]["default"]["linode"]["apiUrl"] = api_url
        if token:
            data["environments"]["default"]["linode"]["token"] = token
        if not data["environments"]["default"].get("label"):
            data["environments"]["default"]["label"] = "Default"


def _validate_config(cfg: Config) -> None:
    """Validate configuration."""
    if not cfg.server.name:
        msg = "server name cannot be empty"
        raise ConfigInvalidError(msg)

    if not cfg.server.log_level:
        msg = "log level cannot be empty"
        raise ConfigInvalidError(msg)

    if not cfg.environments:
        msg = "no environments defined in configuration"
        raise ConfigInvalidError(msg)

    for env_name, env in cfg.environments.items():
        if not env_name:
            msg = "environment name cannot be empty"
            raise ConfigInvalidError(msg)

        if env.linode.api_url or env.linode.token:
            if not env.linode.api_url:
                msg = (
                    f"environment '{env_name}': "
                    "Linode API URL is required when token is provided"
                )
                raise ConfigInvalidError(msg)
            if not env.linode.token:
                msg = (
                    f"environment '{env_name}': "
                    "Linode token is required when API URL is provided"
                )
                raise ConfigInvalidError(msg)


def _data_to_config(data: dict[str, Any]) -> Config:
    """Convert parsed data to Config object."""
    server = ServerConfig(
        name=data.get("server", {}).get("name", "LinodeMCP"),
        log_level=data.get("server", {}).get("logLevel", "info"),
        transport=data.get("server", {}).get("transport", "stdio"),
        host=data.get("server", {}).get("host", "127.0.0.1"),
        port=data.get("server", {}).get("port", 8080),
    )

    metrics = MetricsConfig(
        enabled=data.get("metrics", {}).get("enabled", True),
        port=data.get("metrics", {}).get("port", 9090),
        path=data.get("metrics", {}).get("path", "/metrics"),
    )

    tracing = TracingConfig(
        enabled=data.get("tracing", {}).get("enabled", False),
        exporter=data.get("tracing", {}).get("exporter", "otlp"),
        endpoint=data.get("tracing", {}).get("endpoint", "localhost:4317"),
        sample_rate=data.get("tracing", {}).get("sampleRate", 1.0),
    )

    resilience = ResilienceConfig(
        rate_limit_per_minute=data.get("resilience", {}).get("rateLimitPerMinute", 700),
        circuit_breaker_threshold=data.get("resilience", {}).get(
            "circuitBreakerThreshold", 5
        ),
        circuit_breaker_timeout=data.get("resilience", {}).get(
            "circuitBreakerTimeout", 30
        ),
        max_retries=data.get("resilience", {}).get("maxRetries", 3),
        base_retry_delay=data.get("resilience", {}).get("baseRetryDelay", 1),
        max_retry_delay=data.get("resilience", {}).get("maxRetryDelay", 30),
    )

    environments: dict[str, EnvironmentConfig] = {}
    for env_name, env_data in data.get("environments", {}).items():
        linode_data = env_data.get("linode", {})
        linode_cfg = LinodeConfig(
            api_url=linode_data.get("apiUrl", ""),
            token=linode_data.get("token", ""),
        )
        environments[env_name] = EnvironmentConfig(
            label=env_data.get("label", ""),
            linode=linode_cfg,
        )

    return Config(
        server=server,
        metrics=metrics,
        tracing=tracing,
        resilience=resilience,
        environments=environments,
    )


def load_from_file(path: Path) -> Config:
    """Load configuration from a file."""
    if not path.exists():
        msg = f"configuration file not found: {path}"
        raise ConfigFileNotFoundError(msg)

    try:
        _validate_path(path)
    except PathValidationError as e:
        msg = f"invalid file path: {path}"
        raise ConfigInvalidError(msg) from e

    try:
        content = path.read_text(encoding="utf-8")
    except OSError as e:
        msg = f"failed to read config file: {path}"
        raise ConfigError(msg) from e

    data = _parse_config_data(content)
    _apply_defaults(data)
    _apply_environment_overrides(data)

    cfg = _data_to_config(data)
    _validate_config(cfg)

    return cfg


def load() -> Config:
    """Load configuration from default location."""
    return load_from_file(get_config_path())


def exists() -> bool:
    """Check if configuration file exists."""
    return get_config_path().exists()
