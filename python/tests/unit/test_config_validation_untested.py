"""Behavioral tests for the untested error/normalization branches in
``linodemcp.config``.

These target parse and validation paths that the existing config tests
skip: the fall-through arms of environment lookup, path-security guards,
env-override edge cases, the report-grammar conflict checks, the profile
coercion helpers, duration parsing, and the file-load / atomic-write error
handling. Each test drives the real function with a representative bad or
edge input and pins the specific error type/message or the normalized
result, so a regression in that branch fails the test.
"""

import json
import logging
from pathlib import Path
from typing import cast

import pytest

from linodemcp.config import (
    Config,
    ConfigError,
    ConfigInvalidError,
    ConfigMalformedError,
    EnvironmentConfig,
    EnvironmentNotFoundError,
    LinodeConfig,
    PathValidationError,
    ReportConfig,
    ReportFilter,
    ServerConfig,
    exists,
    get_config_dir,
    get_config_path,
    load,
    load_from_file,
    parse_duration_seconds,
    validate_config,
    validate_path,
    write_atomic,
)

# A valid default environment appended to config files so ``load_from_file``
# clears ``validate_config`` and the test can focus on the parse/override
# branch it drives. Kept as top-level YAML so it concatenates after another
# top-level block.
_VALID_ENV_YAML = """
environments:
  default:
    label: Default
    linode:
      apiUrl: https://api.linode.com/v4
      token: tok
"""


def _clear_linode_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Drop the Linode env overrides so the config file is the only source."""
    monkeypatch.delenv("LINODEMCP_LINODE_TOKEN", raising=False)
    monkeypatch.delenv("LINODEMCP_LINODE_API_URL", raising=False)


def _valid_config() -> Config:
    """A config that passes ``validate_config`` so tests can mutate one
    field and prove the targeted guard is what rejects it."""
    return Config(
        environments={
            "default": EnvironmentConfig(
                label="Default",
                linode=LinodeConfig(
                    api_url="https://api.linode.com/v4",
                    token="tok",
                ),
            ),
        },
    )


# --- Config.select_environment / get_linode_environment ------------------


def test_select_environment_falls_back_to_first_when_no_default() -> None:
    """With no matching name and no ``default`` key, selection returns the
    first configured environment rather than raising."""
    only_env = EnvironmentConfig(
        label="Prod",
        linode=LinodeConfig(api_url="https://api.linode.com/v4", token="t"),
    )
    cfg = Config(environments={"prod": only_env})

    assert cfg.select_environment("does-not-exist") is only_env


def test_get_linode_environment_without_environments_raises() -> None:
    """No environments at all is a distinct error from a missing name."""
    cfg = Config()

    with pytest.raises(
        EnvironmentNotFoundError,
        match="no provider environments configured",
    ):
        cfg.get_linode_environment("default")


# --- validate_path -------------------------------------------------------


def test_validate_path_empty_raises() -> None:
    """The falsy-path guard rejects an empty path before any resolution."""
    with pytest.raises(PathValidationError, match="path cannot be empty"):
        # The guard only cares that the value is falsy; cast keeps mypy happy
        # about the declared Path parameter.
        validate_path(cast("Path", ""))


def test_validate_path_traversal_in_resolved_raises(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A resolved path that still carries ``..`` is rejected as traversal.

    ``Path.resolve`` normally collapses ``..``; force a resolved value that
    keeps it so the guard's traversal arm actually runs.
    """

    def fake_resolve(self: Path) -> Path:
        return Path("/srv/data/../secret")

    monkeypatch.setattr(Path, "resolve", fake_resolve)

    with pytest.raises(PathValidationError, match="directory traversal"):
        validate_path(Path("/srv/app/config.yml"))


def test_validate_path_resolve_failure_raises(
    monkeypatch: pytest.MonkeyPatch,
    tmp_path: Path,
) -> None:
    """When resolution itself blows up, the OSError is wrapped as a
    PathValidationError. The first resolve (dangerous-prefix check) must
    succeed and the second (traversal check) raise, so a call counter makes
    only the second one fail."""
    state = {"calls": 0}
    original = Path.resolve

    def flaky_resolve(self: Path) -> Path:
        state["calls"] += 1
        if state["calls"] >= 2:
            raise OSError("resolve failed")
        return original(self)

    monkeypatch.setattr(Path, "resolve", flaky_resolve)

    with pytest.raises(PathValidationError, match="failed to resolve path"):
        validate_path(tmp_path / "config.yml")


# --- get_config_dir / get_config_path ------------------------------------


def test_get_config_dir_custom_valid_returns_parent(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A safe LINODEMCP_CONFIG_PATH yields that file's parent directory."""
    custom = tmp_path / "sub" / "config.yml"
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(custom))

    assert get_config_dir() == tmp_path / "sub"


def test_get_config_dir_custom_invalid_falls_back(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A dangerous LINODEMCP_CONFIG_PATH is rejected and the loader falls
    back to the home-directory default."""
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", "/etc/linodemcp/config.yml")

    assert get_config_dir() == Path.home() / ".config" / "linodemcp"


def test_get_config_path_custom_invalid_falls_back(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An invalid custom path is ignored; with no config.json present the
    path resolves to config.yml under the resolved config dir."""
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", "/root/config.yml")
    monkeypatch.setattr("linodemcp.config.get_config_dir", lambda: tmp_path)

    assert get_config_path() == tmp_path / "config.yml"


def test_get_config_path_prefers_existing_json(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An existing config.json wins over the config.yml fallback."""
    monkeypatch.delenv("LINODEMCP_CONFIG_PATH", raising=False)
    (tmp_path / "config.json").write_text("{}", encoding="utf-8")
    monkeypatch.setattr("linodemcp.config.get_config_dir", lambda: tmp_path)

    assert get_config_path() == tmp_path / "config.json"


# --- _parse_config_data --------------------------------------------------


def test_load_json_prefixed_yaml_flow_falls_back_to_yaml(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A ``{``-prefixed config that is not valid JSON (single-quoted YAML
    flow syntax) is not a hard error; the parser falls through to YAML, which
    accepts it, and the file loads."""
    _clear_linode_env(monkeypatch)
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        "{'environments': {'default': {'label': 'D', 'linode': "
        "{'apiUrl': 'https://api.linode.com/v4', 'token': 'tok'}}}}",
        encoding="utf-8",
    )

    cfg = load_from_file(cfg_file)

    assert cfg.environments["default"].linode.token == "tok"


def test_load_yaml_list_is_malformed(tmp_path: Path) -> None:
    """A config file whose top-level YAML parses to a list, not a mapping, is
    rejected as malformed."""
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text("- one\n- two\n", encoding="utf-8")

    with pytest.raises(ConfigMalformedError, match="must be a YAML mapping"):
        load_from_file(cfg_file)


# --- _apply_environment_overrides / _apply_audit_overrides ---------------


def test_env_token_override_sets_default_label(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Injecting a token via env for a default environment that has no label
    stamps the "Default" label so the entry is well-formed."""
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        """
environments:
  default:
    linode:
      apiUrl: https://api.linode.com/v4
""",
        encoding="utf-8",
    )
    monkeypatch.setenv("LINODEMCP_LINODE_TOKEN", "env-token")
    monkeypatch.delenv("LINODEMCP_LINODE_API_URL", raising=False)

    cfg = load_from_file(cfg_file)

    assert cfg.environments["default"].label == "Default"
    assert cfg.environments["default"].linode.token == "env-token"


def test_load_audit_retention_env_override_skips_sqlite(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A retention-only LINODEMCP_AUDIT_* override enters the audit block and
    sets retention_days, but leaves the SQLite sub-block on its defaults when
    no sqlite env vars are set."""
    _clear_linode_env(monkeypatch)
    for var in (
        "LINODEMCP_AUDIT_REDACT_PII",
        "LINODEMCP_AUDIT_SQLITE_ENABLED",
        "LINODEMCP_AUDIT_SQLITE_PATH",
        "LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS",
    ):
        monkeypatch.delenv(var, raising=False)
    monkeypatch.setenv("LINODEMCP_AUDIT_RETENTION_DAYS", "5")
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(_VALID_ENV_YAML, encoding="utf-8")

    cfg = load_from_file(cfg_file)

    assert cfg.audit.retention_days == 5
    assert cfg.audit.sqlite.enabled is False


# --- validate_config -----------------------------------------------------


def test_validate_config_empty_server_name_raises() -> None:
    """An empty server name is rejected before the environments check."""
    cfg = Config(server=ServerConfig(name=""))

    with pytest.raises(ConfigInvalidError, match="server name cannot be empty"):
        validate_config(cfg)


def test_validate_config_empty_log_level_raises() -> None:
    """An empty log level is rejected once the name check passes."""
    cfg = Config(server=ServerConfig(name="srv", log_level=""))

    with pytest.raises(ConfigInvalidError, match="log level cannot be empty"):
        validate_config(cfg)


def test_validate_config_empty_environment_name_raises() -> None:
    """An environment keyed by the empty string is rejected in the loop."""
    cfg = _valid_config()
    cfg.environments = {
        "": EnvironmentConfig(
            label="x",
            linode=LinodeConfig(api_url="u", token="t"),
        ),
    }

    with pytest.raises(
        ConfigInvalidError,
        match="environment name cannot be empty",
    ):
        validate_config(cfg)


def test_validate_config_token_without_api_url_raises() -> None:
    """A token with no API URL is the mirror of the covered URL-without-token
    case and gets its own required-field error."""
    cfg = _valid_config()
    cfg.environments = {
        "prod": EnvironmentConfig(
            label="p",
            linode=LinodeConfig(api_url="", token="tok"),
        ),
    }

    with pytest.raises(
        ConfigInvalidError,
        match="Linode API URL is required when token is provided",
    ):
        validate_config(cfg)


def test_validate_reports_status_and_status_in_conflict() -> None:
    """A report that sets both scalar ``status`` and list ``status_in`` is a
    grammar conflict."""
    cfg = _valid_config()
    cfg.audit.reports = {
        "r": ReportConfig(
            filter=ReportFilter(status="ok", status_in=["failure"]),
        ),
    }

    with pytest.raises(
        ConfigInvalidError,
        match="sets both status and status_in",
    ):
        validate_config(cfg)


# --- profile/override coercion helpers -----------------------------------


def test_load_profile_string_tuples_filter_and_default(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The string-tuple coercion behind a profile's tool lists drops
    non-strings from a list, treats a scalar (non-list) value as empty, and
    treats an absent key as empty."""
    _clear_linode_env(monkeypatch)
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        "profiles:\n"
        "  operator:\n"
        "    description: mixed\n"
        "    allowed_tools: [a, 1, b, null]\n"
        "    denied_tools: not-a-list\n" + _VALID_ENV_YAML,
        encoding="utf-8",
    )

    cfg = load_from_file(cfg_file)

    profile = cfg.profiles["operator"]
    assert profile.allowed_tools == ("a", "b")
    assert profile.denied_tools == ()
    assert profile.allowed_environments == ()


def test_load_user_profiles_skip_invalid_and_coerce_description(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Non-string profile keys and non-dict bodies are dropped; a non-string
    description is coerced to the empty string."""
    _clear_linode_env(monkeypatch)
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        "profiles:\n"
        "  123:\n"
        "    description: numeric-key-dropped\n"
        "  bad-body: not-a-dict\n"
        "  num-desc:\n"
        "    description: 42\n"
        "    allowed_tools: [t1, 7]\n"
        "  good:\n"
        "    description: real\n"
        "    allow_yolo: true\n" + _VALID_ENV_YAML,
        encoding="utf-8",
    )

    cfg = load_from_file(cfg_file)

    assert set(cfg.profiles) == {"num-desc", "good"}
    assert cfg.profiles["num-desc"].description == ""
    assert cfg.profiles["num-desc"].allowed_tools == ("t1",)
    assert cfg.profiles["good"].description == "real"
    assert cfg.profiles["good"].allow_yolo is True


def test_load_builtin_overrides_skip_invalid_entries(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Non-string keys and non-dict bodies are dropped; valid entries parse,
    with ``disabled`` defaulting to False when absent."""
    _clear_linode_env(monkeypatch)
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        "profiles_builtin_overrides:\n"
        "  9:\n"
        "    disabled: true\n"
        "  str-body: nope\n"
        "  real:\n"
        "    disabled: true\n"
        "  default-off: {}\n" + _VALID_ENV_YAML,
        encoding="utf-8",
    )

    cfg = load_from_file(cfg_file)

    overrides = cfg.profiles_builtin_overrides
    assert set(overrides) == {"real", "default-off"}
    assert overrides["real"].disabled is True
    assert overrides["default-off"].disabled is False


# --- _data_to_config deprecation warning ---------------------------------


def test_load_warns_on_deprecated_prometheus_keys(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
    caplog: pytest.LogCaptureFixture,
) -> None:
    """The old flat prometheusPort/prometheusPath keys are ignored with a
    warning; the nested prometheus block keeps its defaults."""
    _clear_linode_env(monkeypatch)
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        "observability:\n"
        "  metrics:\n"
        "    prometheusPort: 1234\n"
        "    prometheusPath: /old\n" + _VALID_ENV_YAML,
        encoding="utf-8",
    )

    with caplog.at_level(logging.WARNING, logger="linodemcp.config"):
        cfg = load_from_file(cfg_file)

    assert "deprecated" in caplog.text
    assert cfg.observability.metrics.prometheus.port == 8888
    assert cfg.observability.metrics.prometheus.path == "/metrics"


# --- parse_duration_seconds ----------------------------------------------


def test_parse_duration_empty_raises() -> None:
    """A blank duration (after stripping) is rejected."""
    with pytest.raises(ValueError, match="empty duration"):
        parse_duration_seconds("   ")


def test_parse_duration_signed() -> None:
    """A leading sign flips (or keeps) the sign of the total."""
    assert parse_duration_seconds("-1h") == -3600.0
    assert parse_duration_seconds("+30m") == 1800.0


def test_parse_duration_multi_unit() -> None:
    """Adjacent unit tokens accumulate."""
    assert parse_duration_seconds("2h45m") == 9900.0


def test_parse_duration_gap_raises() -> None:
    """A gap between tokens leaves an unparsed tail and is rejected."""
    with pytest.raises(ValueError, match="invalid duration"):
        parse_duration_seconds("1h 2m")


# --- load_from_file / load -----------------------------------------------


def test_load_from_file_dangerous_path_raises() -> None:
    """An existing file under a blocked prefix fails path validation and is
    surfaced as an invalid-path config error."""
    with pytest.raises(ConfigInvalidError, match="invalid file path"):
        load_from_file(Path("/etc/hosts"))


def test_load_from_file_read_error_raises(tmp_path: Path) -> None:
    """A path that exists and validates but cannot be read as text (a
    directory) surfaces the OSError as a ConfigError."""
    with pytest.raises(ConfigError, match="failed to read config file"):
        load_from_file(tmp_path)


def test_load_reads_from_config_path(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """``load`` resolves the default path and parses it."""
    cfg_file = tmp_path / "config.yml"
    cfg_file.write_text(
        """
environments:
  default:
    label: Default
    linode:
      apiUrl: https://api.linode.com/v4
      token: tok
""",
        encoding="utf-8",
    )
    monkeypatch.delenv("LINODEMCP_LINODE_TOKEN", raising=False)
    monkeypatch.delenv("LINODEMCP_LINODE_API_URL", raising=False)
    monkeypatch.setattr("linodemcp.config.get_config_path", lambda: cfg_file)

    cfg = load()

    assert cfg.environments["default"].linode.token == "tok"


# --- write_atomic / exists -----------------------------------------------


def test_write_atomic_json_serializes_json(
    tmp_path: Path,
    sample_config: Config,
) -> None:
    """A ``.json`` target is written as JSON that round-trips through
    ``json.loads``."""
    target = tmp_path / "out.json"

    write_atomic(target, sample_config)

    parsed = json.loads(target.read_text(encoding="utf-8"))
    assert parsed["server"]["name"] == sample_config.server.name
    assert parsed["environments"]["default"]["linode"]["token"] == "test-token-123"


def test_write_atomic_rename_failure_cleans_temp(
    tmp_path: Path,
    sample_config: Config,
) -> None:
    """When the final rename fails (target is a directory), the OSError
    propagates and the temp file is cleaned up rather than left behind."""
    target = tmp_path / "target"
    target.mkdir()

    with pytest.raises(OSError, match="Is a directory"):
        write_atomic(target, sample_config)

    assert list(tmp_path.glob(".target.tmp.*")) == []


def test_exists_reflects_config_path(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """``exists`` mirrors whether the resolved config path is present."""
    present = tmp_path / "config.yml"
    present.write_text("x: 1\n", encoding="utf-8")
    monkeypatch.setattr("linodemcp.config.get_config_path", lambda: present)
    assert exists() is True

    monkeypatch.setattr(
        "linodemcp.config.get_config_path",
        lambda: tmp_path / "absent.yml",
    )
    assert exists() is False
