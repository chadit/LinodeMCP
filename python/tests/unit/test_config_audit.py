"""Audit config block tests.

Mirrors ``go/internal/config/audit_config_test.go``. Covers defaults,
the explicit-zero (never-delete) distinction, validation, the SQLite
sub-block, and the LINODEMCP_AUDIT_* env overrides.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from linodemcp.config import (
    DEFAULT_AUDIT_REDACT_PII,
    DEFAULT_AUDIT_RETENTION_DAYS,
    DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS,
    ConfigInvalidError,
    load_from_file,
)

if TYPE_CHECKING:
    from pathlib import Path

_MINIMAL = """
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


def _write(tmp_path: Path, audit_block: str) -> Path:
    """Write a minimal config with the supplied audit block appended."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL + audit_block, encoding="utf-8")
    return path


def test_audit_defaults(tmp_path: Path) -> None:
    """An omitted audit block defaults retention to 14, PII redaction on,
    SQLite off."""
    cfg = load_from_file(_write(tmp_path, ""))

    assert cfg.audit.retention_days == DEFAULT_AUDIT_RETENTION_DAYS
    assert cfg.audit.redact_pii is DEFAULT_AUDIT_REDACT_PII
    assert cfg.audit.redact_pii is True
    assert cfg.audit.sqlite.enabled is False
    assert cfg.audit.sqlite.busy_timeout_ms == DEFAULT_AUDIT_SQLITE_BUSY_TIMEOUT_MS


def test_audit_redact_pii_explicit_false_preserved(tmp_path: Path) -> None:
    """An explicit redact_pii: false survives parsing rather than being
    clobbered back to the True default. The absent-vs-explicit distinction
    matters: a quiet re-enrollment would defeat an operator's opt-out.
    """
    cfg = load_from_file(_write(tmp_path, "audit:\n  redact_pii: false\n"))
    assert cfg.audit.redact_pii is False


def test_audit_redact_pii_explicit_true_preserved(tmp_path: Path) -> None:
    """An explicit redact_pii: true round-trips. Belt-and-suspenders against
    a future refactor that might fold redact_pii into a multi-value or
    invert the field meaning.
    """
    cfg = load_from_file(_write(tmp_path, "audit:\n  redact_pii: true\n"))
    assert cfg.audit.redact_pii is True


def test_audit_retention_explicit_zero_preserved(tmp_path: Path) -> None:
    """An explicit retention_days: 0 survives as never-delete."""
    cfg = load_from_file(_write(tmp_path, "audit:\n  retention_days: 0\n"))
    assert cfg.audit.retention_days == 0


def test_audit_retention_explicit_value(tmp_path: Path) -> None:
    """A non-default retention passes through unchanged."""
    cfg = load_from_file(_write(tmp_path, "audit:\n  retention_days: 30\n"))
    assert cfg.audit.retention_days == 30


def test_audit_retention_negative_rejected(tmp_path: Path) -> None:
    """A negative retention is a load-time validation error."""
    with pytest.raises(ConfigInvalidError):
        load_from_file(_write(tmp_path, "audit:\n  retention_days: -1\n"))


def test_audit_sqlite_block_parses(tmp_path: Path) -> None:
    """The SQLite sub-block fields load."""
    db_path = str(tmp_path / "audit.db")
    block = (
        "audit:\n"
        "  sqlite:\n"
        "    enabled: true\n"
        f'    path: "{db_path}"\n'
        "    busy_timeout_ms: 1234\n"
    )
    cfg = load_from_file(_write(tmp_path, block))

    assert cfg.audit.sqlite.enabled is True
    assert cfg.audit.sqlite.path == db_path
    assert cfg.audit.sqlite.busy_timeout_ms == 1234


def test_audit_env_overrides(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """LINODEMCP_AUDIT_* env vars override the file values."""
    monkeypatch.setenv("LINODEMCP_AUDIT_RETENTION_DAYS", "7")
    monkeypatch.setenv("LINODEMCP_AUDIT_REDACT_PII", "false")
    monkeypatch.setenv("LINODEMCP_AUDIT_SQLITE_ENABLED", "true")
    monkeypatch.setenv("LINODEMCP_AUDIT_SQLITE_PATH", "/var/audit.db")
    monkeypatch.setenv("LINODEMCP_AUDIT_SQLITE_BUSY_TIMEOUT_MS", "999")

    cfg = load_from_file(
        _write(tmp_path, "audit:\n  retention_days: 30\n  redact_pii: true\n"),
    )

    assert cfg.audit.retention_days == 7
    assert cfg.audit.redact_pii is False
    assert cfg.audit.sqlite.enabled is True
    assert cfg.audit.sqlite.path == "/var/audit.db"
    assert cfg.audit.sqlite.busy_timeout_ms == 999


def test_audit_retention_default_matches_audit_package() -> None:
    """The config default must stay in sync with the audit sweeper default."""
    from linodemcp.audit import DEFAULT_AUDIT_RETENTION_DAYS as AUDIT_PKG_DEFAULT

    assert DEFAULT_AUDIT_RETENTION_DAYS == AUDIT_PKG_DEFAULT == 14
