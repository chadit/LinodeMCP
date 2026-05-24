"""Configuration management for LinodeMCP."""

import contextlib
import json
import logging
import os
import re
import tempfile
from dataclasses import dataclass, field
from datetime import datetime
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


# Default rotated-log retention window in days. Keep in sync with
# linodemcp.audit.DEFAULT_AUDIT_RETENTION_DAYS, which is the sweeper's
# intrinsic default when no config is supplied (config stays a leaf and
# does not import the audit package).
DEFAULT_AUDIT_RETENTION_DAYS = 14

# Default SQLite busy_timeout in milliseconds, applied when the SQLite
# sink is enabled but no explicit timeout is configured. Consumed by
# the Phase 3b sink.
DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS = 5000

# Default for audit.redact_pii: PII fields (tax_id, phone, address_1/2,
# city, state, zip) are redacted alongside the always-on credential
# list. Operators who need raw PII in audit (e.g. for accountability
# investigations) can opt out by setting audit.redact_pii: false.
DEFAULT_AUDIT_REDACT_PII = True


@dataclass
class AuditSQLiteConfig:
    """Optional SQLite audit sink settings.

    Disabled by default; when enabled, audit events dual-write to both
    JSONL and SQLite. An empty ``path`` resolves to audit.db alongside
    the JSONL log. Consumed by the Phase 3b sink.
    """

    enabled: bool = False
    path: str = ""
    busy_timeout_ms: int = DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS


# Report output modes. Summary aggregates into per-bucket counts (like
# linode_audit_summary); list returns the matching events (like
# linode_audit_recent), capped by the report's limit.
REPORT_OUTPUT_SUMMARY = "summary"
REPORT_OUTPUT_LIST = "list"


@dataclass
class ReportFilter:
    """Typed report filter grammar. Each field maps to an event field.

    ``tool`` and ``environment`` are globs; ``capability`` and
    ``status`` accept either a scalar or the ``*_in`` list form (not
    both). ``since_offset`` is a duration relative to now (e.g. "24h");
    ``since`` and ``until`` are absolute RFC 3339 timestamps. Compiled to
    a predicate by the Phase 4b tool.
    """

    tool: str = ""
    capability: str = ""
    capability_in: list[str] = field(default_factory=list[str])
    status: str = ""
    status_in: list[str] = field(default_factory=list[str])
    environment: str = ""
    profile: str = ""
    since_offset: str = ""
    since: str = ""
    until: str = ""


@dataclass
class ReportConfig:
    """One named custom audit report under AuditConfig.reports.

    The linode_audit_report tool (Phase 4b) resolves and runs it at call
    time, so editing the report file takes effect on the next call. An
    empty ``output`` defaults to "summary".
    """

    description: str = ""
    filter: ReportFilter = field(default_factory=ReportFilter)
    group_by: list[str] = field(default_factory=list[str])
    output: str = REPORT_OUTPUT_SUMMARY
    limit: int = 0


@dataclass
class AuditConfig:
    """Audit-log settings.

    The JSONL sink is always on (Phase 2); these fields tune retention,
    the optional SQLite sink (Phase 3b), the optional PII redaction tier
    (Phase 4c), and named custom reports (Phase 4a/b).
    ``retention_days`` of 0 means "never delete"; an absent key defaults
    to DEFAULT_AUDIT_RETENTION_DAYS. ``redact_pii`` defaults to True so
    PII fields are redacted alongside credentials; operators opt out by
    setting it to False. Both absent-vs-explicit distinctions are
    handled at parse time via ``dict.get`` returning None.
    """

    retention_days: int = DEFAULT_AUDIT_RETENTION_DAYS
    redact_pii: bool = DEFAULT_AUDIT_REDACT_PII
    sqlite: AuditSQLiteConfig = field(default_factory=AuditSQLiteConfig)
    reports: dict[str, ReportConfig] = field(default_factory=dict[str, ReportConfig])


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
    audit: AuditConfig = field(default_factory=AuditConfig)

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

    _apply_audit_overrides(data)


def _apply_audit_overrides(data: dict[str, Any]) -> None:
    """Apply LINODEMCP_AUDIT_* environment overrides onto the raw dict."""
    retention = os.getenv("LINODEMCP_AUDIT_RETENTION_DAYS")
    redact_pii = os.getenv("LINODEMCP_AUDIT_REDACT_PII")
    sqlite_enabled = os.getenv("LINODEMCP_AUDIT_SQLITE_ENABLED")
    sqlite_path = os.getenv("LINODEMCP_AUDIT_SQLITE_PATH")
    sqlite_timeout = os.getenv("LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS")

    if not any((retention, redact_pii, sqlite_enabled, sqlite_path, sqlite_timeout)):
        return

    data.setdefault("audit", {})
    audit = data["audit"]

    if retention is not None:
        with contextlib.suppress(ValueError):
            audit["retention_days"] = int(retention)

    if redact_pii is not None:
        audit["redact_pii"] = redact_pii.lower() in ("true", "1")

    if sqlite_enabled is None and sqlite_path is None and sqlite_timeout is None:
        return

    audit.setdefault("sqlite", {})
    sqlite = audit["sqlite"]
    if sqlite_enabled is not None:
        sqlite["enabled"] = sqlite_enabled.lower() in ("true", "1")
    if sqlite_path:
        sqlite["path"] = sqlite_path
    if sqlite_timeout is not None:
        with contextlib.suppress(ValueError):
            sqlite["busy_timeout_ms"] = int(sqlite_timeout)


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

    if cfg.audit.retention_days < 0:
        msg = "audit.retention_days cannot be negative"
        raise ConfigInvalidError(msg)

    _validate_reports(cfg.audit.reports)


def _validate_reports(reports: dict[str, ReportConfig]) -> None:
    """Validate each custom report's structural grammar: a known output
    mode, a parseable since_offset, parseable since/until timestamps, and
    capability/status using either the scalar or list form but not both.
    """
    for name, report in reports.items():
        if report.output not in (REPORT_OUTPUT_SUMMARY, REPORT_OUTPUT_LIST):
            msg = f"report {name!r} output must be 'summary' or 'list'"
            raise ConfigInvalidError(msg)

        flt = report.filter
        if flt.capability and flt.capability_in:
            msg = f"report {name!r} sets both capability and capability_in"
            raise ConfigInvalidError(msg)

        if flt.status and flt.status_in:
            msg = f"report {name!r} sets both status and status_in"
            raise ConfigInvalidError(msg)

        if flt.since_offset:
            try:
                parse_duration_seconds(flt.since_offset)
            except ValueError as exc:
                msg = f"report {name!r} since_offset is not a valid duration"
                raise ConfigInvalidError(msg) from exc

        for label, value in (("since", flt.since), ("until", flt.until)):
            if not value:
                continue
            try:
                datetime.fromisoformat(value)
            except ValueError as exc:
                msg = f"report {name!r} {label} is not a valid RFC 3339 timestamp"
                raise ConfigInvalidError(msg) from exc


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
        audit=_parse_audit(data.get("audit")),
    )


def _parse_audit(raw: Any) -> AuditConfig:
    """Build an AuditConfig from the raw ``audit`` block.

    An absent ``retention_days`` key defaults to
    DEFAULT_AUDIT_RETENTION_DAYS; an explicit 0 is preserved as
    "never delete" because ``dict.get`` returns None only when the key
    is absent. Same absent-vs-explicit handling for ``redact_pii``.
    """
    audit_data = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
    sqlite_raw = audit_data.get("sqlite")
    sqlite_data = (
        cast("dict[str, Any]", sqlite_raw) if isinstance(sqlite_raw, dict) else {}
    )

    retention_raw = audit_data.get("retention_days")
    retention_days = (
        DEFAULT_AUDIT_RETENTION_DAYS if retention_raw is None else int(retention_raw)
    )

    redact_pii_raw = audit_data.get("redact_pii")
    redact_pii = (
        DEFAULT_AUDIT_REDACT_PII if redact_pii_raw is None else bool(redact_pii_raw)
    )

    return AuditConfig(
        retention_days=retention_days,
        redact_pii=redact_pii,
        sqlite=AuditSQLiteConfig(
            enabled=bool(sqlite_data.get("enabled", False)),
            path=str(sqlite_data.get("path", "")),
            busy_timeout_ms=int(
                sqlite_data.get("busy_timeout_ms", DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS)
            ),
        ),
        reports=_parse_reports(audit_data.get("reports")),
    )


def _parse_reports(raw: Any) -> dict[str, ReportConfig]:
    """Build the named-report map from the raw ``audit.reports`` block.

    An empty or absent ``output`` defaults to summary; the rest of the
    grammar is validated later by validate_config.
    """
    if not isinstance(raw, dict):
        return {}

    reports: dict[str, ReportConfig] = {}
    for name, value in cast("dict[str, Any]", raw).items():
        report_data = cast("dict[str, Any]", value) if isinstance(value, dict) else {}
        reports[str(name)] = ReportConfig(
            description=str(report_data.get("description", "")),
            filter=_parse_report_filter(report_data.get("filter")),
            group_by=_parse_string_list(report_data.get("group_by")),
            output=str(report_data.get("output") or REPORT_OUTPUT_SUMMARY),
            limit=int(report_data.get("limit", 0)),
        )

    return reports


def _parse_report_filter(raw: Any) -> ReportFilter:
    """Build a ReportFilter from the raw ``filter`` block."""
    data = cast("dict[str, Any]", raw) if isinstance(raw, dict) else {}
    return ReportFilter(
        tool=str(data.get("tool", "")),
        capability=str(data.get("capability", "")),
        capability_in=_parse_string_list(data.get("capability_in")),
        status=str(data.get("status", "")),
        status_in=_parse_string_list(data.get("status_in")),
        environment=str(data.get("environment", "")),
        profile=str(data.get("profile", "")),
        since_offset=str(data.get("since_offset", "")),
        since=str(data.get("since", "")),
        until=str(data.get("until", "")),
    )


def _parse_string_list(raw: Any) -> list[str]:
    """Coerce a YAML/JSON list into a list of strings, dropping non-strings."""
    if not isinstance(raw, list):
        return []

    raw_list = cast("list[object]", raw)
    return [item for item in raw_list if isinstance(item, str)]


_DURATION_TOKEN = re.compile(r"(\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)")
_DURATION_UNIT_SECONDS = {
    "ns": 1e-9,
    "us": 1e-6,
    "µs": 1e-6,
    "ms": 1e-3,
    "s": 1.0,
    "m": 60.0,
    "h": 3600.0,
}


def parse_duration_seconds(value: str) -> float:
    """Parse a Go-style duration ("24h", "2h45m", "300ms") into seconds.

    Mirrors Go's time.ParseDuration for the units the report grammar
    uses. Raises ValueError on an empty or malformed value.
    """
    text = value.strip()
    if not text:
        msg = "empty duration"
        raise ValueError(msg)

    sign = 1.0
    if text[0] in "+-":
        sign = -1.0 if text[0] == "-" else 1.0
        text = text[1:]

    total = 0.0
    pos = 0
    for match in _DURATION_TOKEN.finditer(text):
        if match.start() != pos:
            break

        total += float(match.group(1)) * _DURATION_UNIT_SECONDS[match.group(2)]
        pos = match.end()

    if pos != len(text) or pos == 0:
        msg = f"invalid duration: {value!r}"
        raise ValueError(msg)

    return sign * total


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


def _config_to_data(cfg: Config) -> dict[str, Any]:
    """Convert a Config back into the parsed-dict shape ``_data_to_config``
    consumes. The keys match the on-disk schema (camelCase for server
    fields, snake_case for the profile maps) so the round-trip is
    byte-for-byte symmetric with ``load_from_file``.
    """
    environments: dict[str, Any] = {}
    for name, env in cfg.environments.items():
        environments[name] = {
            "label": env.label,
            "linode": {
                "apiUrl": env.linode.api_url,
                "token": env.linode.token,
            },
        }

    profiles: dict[str, Any] = {}
    for name, prof in (cfg.profiles or {}).items():
        profiles[name] = {
            "description": prof.description,
            "allowed_tools": list(prof.allowed_tools),
            "denied_tools": list(prof.denied_tools),
            "allowed_environments": list(prof.allowed_environments),
            "required_token_scopes": list(prof.required_token_scopes),
            "allow_yolo": prof.allow_yolo,
        }

    overrides: dict[str, Any] = {}
    for name, override in (cfg.profiles_builtin_overrides or {}).items():
        overrides[name] = {"disabled": override.disabled}

    return {
        "server": {
            "name": cfg.server.name,
            "logLevel": cfg.server.log_level,
            "transport": cfg.server.transport,
            "host": cfg.server.host,
            "port": cfg.server.port,
        },
        "observability": {
            "tracing": {
                "enabled": cfg.observability.tracing.enabled,
                "endpoint": cfg.observability.tracing.endpoint,
                "sampleRate": cfg.observability.tracing.sample_rate,
            },
            "metrics": {
                "enabled": cfg.observability.metrics.enabled,
                "runtime": cfg.observability.metrics.runtime,
                "host": cfg.observability.metrics.host,
                "prometheusPort": cfg.observability.metrics.prometheus_port,
                "prometheusPath": cfg.observability.metrics.prometheus_path,
            },
            "logging": {
                "format": cfg.observability.logging.format,
                "level": cfg.observability.logging.level,
            },
            "health": {
                "enabled": cfg.observability.health.enabled,
                "port": cfg.observability.health.port,
                "path": cfg.observability.health.path,
            },
        },
        "resilience": {
            "rateLimitPerMinute": cfg.resilience.rate_limit_per_minute,
            "circuitBreakerThreshold": cfg.resilience.circuit_breaker_threshold,
            "circuitBreakerTimeout": cfg.resilience.circuit_breaker_timeout,
            "maxRetries": cfg.resilience.max_retries,
            "baseRetryDelay": cfg.resilience.base_retry_delay,
            "maxRetryDelay": cfg.resilience.max_retry_delay,
        },
        "environments": environments,
        "active_profile": cfg.active_profile,
        "profiles": profiles,
        "profiles_builtin_overrides": overrides,
        "audit": {
            "retention_days": cfg.audit.retention_days,
            "redact_pii": cfg.audit.redact_pii,
            "sqlite": {
                "enabled": cfg.audit.sqlite.enabled,
                "path": cfg.audit.sqlite.path,
                "busy_timeout_ms": cfg.audit.sqlite.busy_timeout_ms,
            },
        },
    }


def write_atomic(path: Path, cfg: Config) -> None:
    """Rewrite ``path`` with ``cfg`` using a temp-file-and-rename pattern.

    Format is detected from the path's suffix: ``.json`` produces JSON,
    anything else writes YAML. The fresh data is validated by round-
    tripping through ``_data_to_config`` and ``validate_config`` before
    the rename so a malformed write never replaces a good config.

    Comments and key ordering in the original file are NOT preserved
    (PyYAML, like Go's yaml.v3 in non-Node mode, drops them). This
    trade-off is documented in the CLI usage block.
    """
    data = _config_to_data(cfg)
    _apply_defaults(data)

    candidate = _data_to_config(data)
    validate_config(candidate)

    suffix = path.suffix.lower()
    if suffix == ".json":
        serialized = json.dumps(data, indent=2) + "\n"
    else:
        serialized = yaml.safe_dump(data, sort_keys=False)

    parent = path.parent
    parent.mkdir(parents=True, exist_ok=True)

    # Preserve existing mode bits when possible; new files default to 0600.
    mode = 0o600
    if path.exists():
        mode = path.stat().st_mode & 0o777

    # NamedTemporaryFile with delete=False so we can rename it. Same
    # directory as the target so the rename is atomic on POSIX.
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        dir=parent,
        prefix=f".{path.name}.tmp.",
        delete=False,
    ) as tmp:
        tmp.write(serialized)
        tmp.flush()
        os.fsync(tmp.fileno())
        tmp_path = Path(tmp.name)

    try:
        tmp_path.chmod(mode)
        tmp_path.replace(path)
    except OSError:
        if tmp_path.exists():
            tmp_path.unlink()
        raise


def exists() -> bool:
    """Check if configuration file exists."""
    return get_config_path().exists()
