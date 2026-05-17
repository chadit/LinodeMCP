"""Configuration management for LinodeMCP."""

import json
import logging
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, cast

import yaml

logger = logging.getLogger(__name__)


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
    runtime: bool = True
    host: bool = True
    prometheus_port: int = 8888
    prometheus_path: str = "/metrics"


@dataclass
class TracingConfig:
    """OpenTelemetry tracing settings."""

    enabled: bool = False
    exporter: str = "otlp"
    endpoint: str = "localhost:4317"
    sample_rate: float = 1.0
    insecure: bool = True
    headers: dict[str, str] = field(default_factory=dict[str, str])


@dataclass
class ResilienceConfig:
    """Retry, rate limit, circuit breaker, and HTTP pool settings."""

    rate_limit_per_minute: int = 700
    circuit_breaker_threshold: int = 5
    circuit_breaker_timeout: int = 30
    max_retries: int = 3
    base_retry_delay: int = 1
    max_retry_delay: int = 30
    pool_max_connections: int = 10
    pool_max_keepalive_connections: int = 10
    pool_keepalive_expiry: float = 30.0


@dataclass
class LoggingConfig:
    """Logging configuration."""

    level: str = "info"
    format: str = "json"


@dataclass
class HealthConfig:
    """Health check configuration."""

    enabled: bool = True
    port: int = 8889
    path: str = "/healthz"


@dataclass
class ObservabilityConfig:
    """Observability settings combining tracing, metrics, logging, and health."""

    tracing: TracingConfig = field(default_factory=TracingConfig)
    metrics: MetricsConfig = field(default_factory=MetricsConfig)
    logging: LoggingConfig = field(default_factory=LoggingConfig)
    health: HealthConfig = field(default_factory=HealthConfig)


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


@dataclass(frozen=True)
class UserProfileConfig:
    """User-defined profile entry loaded from config.

    Tuples (not lists) keep the dataclass hashable so callers can compare or
    cache profile entries without copy semantics tripping pyright-strict.
    Wildcard expansion against the live tool registry happens later in the
    profile resolver; the values stored here are the raw spec inputs.
    """

    description: str = ""
    allowed_tools: tuple[str, ...] = ()
    denied_tools: tuple[str, ...] = ()
    allowed_environments: tuple[str, ...] = ()
    required_token_scopes: tuple[str, ...] = ()
    allow_yolo: bool = False


@dataclass(frozen=True)
class BuiltinOverride:
    """Per-built-in toggle. ``disabled`` is the only knob users may flip."""

    disabled: bool = False


@dataclass
class Config:
    """Full LinodeMCP configuration."""

    server: ServerConfig = field(default_factory=ServerConfig)
    observability: ObservabilityConfig = field(default_factory=ObservabilityConfig)
    resilience: ResilienceConfig = field(default_factory=ResilienceConfig)
    environments: dict[str, EnvironmentConfig] = field(
        default_factory=dict[str, EnvironmentConfig]
    )
    active_profile: str = ""
    profiles: dict[str, UserProfileConfig] = field(
        default_factory=dict[str, UserProfileConfig]
    )
    profiles_builtin_overrides: dict[str, BuiltinOverride] = field(
        default_factory=dict[str, BuiltinOverride]
    )

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


def validate_path(path: Path) -> None:
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
            validate_path(path)
        except PathValidationError as e:
            logger.warning(
                "Config path %s failed validation, using default: %s",
                custom_path,
                e,
            )
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
            validate_path(path)
        except PathValidationError as e:
            logger.warning(
                "Config path %s failed validation, using default: %s",
                custom_path,
                e,
            )
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
            parsed: object = json.loads(data_stripped)
            if isinstance(parsed, dict):
                return cast("dict[str, Any]", parsed)
        except json.JSONDecodeError:
            pass

    try:
        parsed = yaml.safe_load(data_stripped)
        if isinstance(parsed, dict):
            return cast("dict[str, Any]", parsed)
        msg = "config must be a YAML mapping, not a scalar or list"
        raise ConfigMalformedError(msg)
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
    data["resilience"].setdefault("poolMaxConnections", 10)
    data["resilience"].setdefault("poolMaxKeepaliveConnections", 10)
    data["resilience"].setdefault("poolKeepaliveExpiry", 30.0)


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


def validate_config(cfg: Config) -> None:
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


def _parse_string_tuple(raw: Any) -> tuple[str, ...]:
    """Coerce a YAML/JSON list-of-strings into a tuple, dropping non-strings.

    Returns an empty tuple if the input is missing, null, or not a list.
    Non-string entries are skipped silently; the resolver later warns on
    unmatched tool names, which catches typos better than parsing errors do.
    """
    if not isinstance(raw, list):
        return ()
    raw_list = cast("list[object]", raw)
    return tuple(item for item in raw_list if isinstance(item, str))


def _parse_user_profiles(raw: Any) -> dict[str, UserProfileConfig]:
    """Convert the ``profiles:`` block into typed ``UserProfileConfig`` values."""
    if not isinstance(raw, dict):
        return {}
    raw_dict = cast("dict[object, object]", raw)
    result: dict[str, UserProfileConfig] = {}
    for name, body in raw_dict.items():
        if not isinstance(name, str) or not isinstance(body, dict):
            continue
        body_dict = cast("dict[str, Any]", body)
        description = body_dict.get("description", "")
        if not isinstance(description, str):
            description = ""
        allow_yolo = bool(body_dict.get("allow_yolo", False))
        result[name] = UserProfileConfig(
            description=description,
            allowed_tools=_parse_string_tuple(body_dict.get("allowed_tools")),
            denied_tools=_parse_string_tuple(body_dict.get("denied_tools")),
            allowed_environments=_parse_string_tuple(
                body_dict.get("allowed_environments")
            ),
            required_token_scopes=_parse_string_tuple(
                body_dict.get("required_token_scopes")
            ),
            allow_yolo=allow_yolo,
        )
    return result


def _parse_builtin_overrides(raw: Any) -> dict[str, BuiltinOverride]:
    """Convert ``profiles_builtin_overrides:`` into typed override values."""
    if not isinstance(raw, dict):
        return {}
    raw_dict = cast("dict[object, object]", raw)
    result: dict[str, BuiltinOverride] = {}
    for name, body in raw_dict.items():
        if not isinstance(name, str) or not isinstance(body, dict):
            continue
        body_dict = cast("dict[str, Any]", body)
        result[name] = BuiltinOverride(disabled=bool(body_dict.get("disabled", False)))
    return result


def _data_to_config(data: dict[str, Any]) -> Config:
    """Convert parsed data to Config object."""
    server = ServerConfig(
        name=data.get("server", {}).get("name", "LinodeMCP"),
        log_level=data.get("server", {}).get("logLevel", "info"),
        transport=data.get("server", {}).get("transport", "stdio"),
        host=data.get("server", {}).get("host", "127.0.0.1"),
        port=data.get("server", {}).get("port", 8080),
    )

    tracing_data = data.get("observability", {}).get("tracing", {})
    metrics_data = data.get("observability", {}).get("metrics", {})
    logging_data = data.get("observability", {}).get("logging", {})
    health_data = data.get("observability", {}).get("health", {})

    observability = ObservabilityConfig(
        tracing=TracingConfig(
            enabled=tracing_data.get("enabled", False),
            endpoint=tracing_data.get("endpoint", "localhost:4317"),
            sample_rate=tracing_data.get("sampleRate", 1.0),
        ),
        metrics=MetricsConfig(
            enabled=metrics_data.get("enabled", True),
            runtime=metrics_data.get("runtime", True),
            host=metrics_data.get("host", True),
            prometheus_port=metrics_data.get("prometheusPort", 8888),
            prometheus_path=metrics_data.get("prometheusPath", "/metrics"),
        ),
        logging=LoggingConfig(
            level=logging_data.get("level", "info"),
            format=logging_data.get("format", "json"),
        ),
        health=HealthConfig(
            enabled=health_data.get("enabled", True),
            port=health_data.get("port", 8889),
            path=health_data.get("path", "/healthz"),
        ),
    )

    resilience_data = data.get("resilience", {})
    resilience = ResilienceConfig(
        rate_limit_per_minute=resilience_data.get("rateLimitPerMinute", 700),
        circuit_breaker_threshold=resilience_data.get("circuitBreakerThreshold", 5),
        circuit_breaker_timeout=resilience_data.get("circuitBreakerTimeout", 30),
        max_retries=resilience_data.get("maxRetries", 3),
        base_retry_delay=resilience_data.get("baseRetryDelay", 1),
        max_retry_delay=resilience_data.get("maxRetryDelay", 30),
        pool_max_connections=resilience_data.get("poolMaxConnections", 10),
        pool_max_keepalive_connections=resilience_data.get(
            "poolMaxKeepaliveConnections", 10
        ),
        pool_keepalive_expiry=float(resilience_data.get("poolKeepaliveExpiry", 30.0)),
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

    active_profile_raw = data.get("active_profile", "")
    active_profile = active_profile_raw if isinstance(active_profile_raw, str) else ""

    return Config(
        server=server,
        observability=observability,
        resilience=resilience,
        environments=environments,
        active_profile=active_profile,
        profiles=_parse_user_profiles(data.get("profiles")),
        profiles_builtin_overrides=_parse_builtin_overrides(
            data.get("profiles_builtin_overrides")
        ),
    )


def load_from_file(path: Path) -> Config:
    """Load configuration from a file."""
    if not path.exists():
        msg = f"configuration file not found: {path}"
        raise ConfigFileNotFoundError(msg)

    try:
        validate_path(path)
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
    validate_config(cfg)

    return cfg


def load() -> Config:
    """Load configuration from default location."""
    return load_from_file(get_config_path())


def exists() -> bool:
    """Check if configuration file exists."""
    return get_config_path().exists()
