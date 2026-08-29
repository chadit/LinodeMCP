"""Audit subsystem health report.

Mirrors ``go/internal/audit/health.go``. Reports the JSONL log
footprint and, when a SQLite path is given, the SQLite row count,
oldest event, and database size.
"""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass, field
from pathlib import Path

from linodemcp.audit.jsonl import ACTIVE_LOG_FILE_NAME
from linodemcp.audit.retention import parse_rotated_file_day
from linodemcp.audit.store import read_with_fallback
from linodemcp.genlocal import AuditHealthResponse, AuditHealthSQLite

# Always 0: the sinks write synchronously, so there is no bounded channel to
# drop from. The member stays declared so the answer's shape survives a future
# async sink that does drop.
_DROPPED_EVENTS = 0


@dataclass
class _JSONLHealth:
    """The JSONL half of the report while it is being gathered.

    The answer it fills is built once with every member, so the halves are
    carried rather than assigned into it.
    """

    oldest_rotated_date: str = ""
    disk_bytes: int = 0
    rotated_file_count: int = 0
    active_log_exists: bool = False


@dataclass
class _HealthParts:
    """One read of both sinks, before the store fallback has said whether it
    degraded.
    """

    jsonl: _JSONLHealth = field(default_factory=_JSONLHealth)
    sqlite: AuditHealthSQLite | None = None


def health(sqlite_path: str, jsonl_dir: str) -> AuditHealthResponse:
    """The audit stores' own status, read through the configured database where
    one answers and through the JSONL log otherwise.

    A configured database that will not open leaves the JSONL half standing
    behind a warning; only a log nothing can read raises. The policy sits here
    rather than beside the tool because it is the store's own degrade rule.
    """
    parts, warnings = read_with_fallback(
        sqlite_path, lambda store: _collect_health(store, jsonl_dir)
    )

    return AuditHealthResponse(
        jsonl_path=str(Path(jsonl_dir) / ACTIVE_LOG_FILE_NAME),
        active_log_exists=parts.jsonl.active_log_exists,
        rotated_file_count=parts.jsonl.rotated_file_count,
        oldest_rotated_date=parts.jsonl.oldest_rotated_date,
        disk_bytes=parts.jsonl.disk_bytes,
        dropped_events=_DROPPED_EVENTS,
        sqlite=parts.sqlite,
        warnings=warnings,
    )


def _collect_health(sqlite_path: str, jsonl_dir: str) -> _HealthParts:
    """Read both sinks once. The JSONL directory is always inspected; the
    SQLite database only when ``sqlite_path`` is given. A missing JSONL
    directory reports zero values, not a failure.
    """
    parts = _HealthParts()
    _collect_jsonl_health(jsonl_dir, parts.jsonl)

    if sqlite_path:
        parts.sqlite = _collect_sqlite_health(sqlite_path)

    return parts


def _collect_jsonl_health(directory: str, report: _JSONLHealth) -> None:
    """Fill the JSONL portion of the report from the directory contents."""
    base = Path(directory)
    if not base.is_dir():
        return

    oldest_date = ""

    for entry in base.iterdir():
        if not entry.is_file():
            continue

        report.disk_bytes += entry.stat().st_size

        if entry.name == ACTIVE_LOG_FILE_NAME:
            report.active_log_exists = True
            continue

        day = parse_rotated_file_day(entry.name)
        if day is None:
            continue

        report.rotated_file_count += 1
        date_str = day.strftime("%Y-%m-%d")
        if not oldest_date or date_str < oldest_date:
            oldest_date = date_str

    report.oldest_rotated_date = oldest_date


def _collect_sqlite_health(path: str) -> AuditHealthSQLite:
    """Query the row count and oldest timestamp and stat the DB size."""
    conn = sqlite3.connect(path)
    try:
        count, oldest = conn.execute(
            "SELECT COUNT(*), COALESCE(MIN(ts_unix_ns), 0) FROM events"
        ).fetchone()
    finally:
        conn.close()

    db_bytes = Path(path).stat().st_size if Path(path).exists() else 0

    return AuditHealthSQLite(
        path=path,
        event_count=int(count),
        oldest_event_unix_ns=int(oldest),
        db_bytes=db_bytes,
    )
