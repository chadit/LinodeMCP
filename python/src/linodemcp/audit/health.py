"""Audit subsystem health report.

Mirrors ``go/internal/audit/health.go``. Reports the JSONL log
footprint and, when a SQLite path is given, the SQLite row count,
oldest event, and database size.
"""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass
from pathlib import Path

from linodemcp.audit.jsonl import ACTIVE_LOG_FILE_NAME
from linodemcp.audit.retention import parse_rotated_file_day


@dataclass
class SQLiteHealth:
    """SQLite-sink portion of the health report."""

    path: str
    event_count: int
    oldest_event_unix_ns: int
    db_bytes: int


@dataclass
class HealthReport:
    """Audit subsystem status.

    ``dropped_events`` is always 0: the sinks write synchronously, so
    there is no bounded channel to drop from. The field exists so the
    wire shape stays stable if a future async sink adds drop accounting.
    """

    jsonl_path: str
    active_log_exists: bool = False
    rotated_file_count: int = 0
    oldest_rotated_date: str = ""
    disk_bytes: int = 0
    dropped_events: int = 0
    sqlite: SQLiteHealth | None = None


def collect_health(sqlite_path: str, jsonl_dir: str) -> HealthReport:
    """Gather audit subsystem status. The JSONL directory is always
    inspected; the SQLite database only when ``sqlite_path`` is given.
    A missing JSONL directory reports zero values, not an error.
    """
    report = HealthReport(jsonl_path=str(Path(jsonl_dir) / ACTIVE_LOG_FILE_NAME))
    _collect_jsonl_health(jsonl_dir, report)

    if sqlite_path:
        report.sqlite = _collect_sqlite_health(sqlite_path)

    return report


def _collect_jsonl_health(directory: str, report: HealthReport) -> None:
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


def _collect_sqlite_health(path: str) -> SQLiteHealth:
    """Query the row count and oldest timestamp and stat the DB size."""
    conn = sqlite3.connect(path)
    try:
        count, oldest = conn.execute(
            "SELECT COUNT(*), COALESCE(MIN(ts_unix_ns), 0) FROM events"
        ).fetchone()
    finally:
        conn.close()

    db_bytes = Path(path).stat().st_size if Path(path).exists() else 0

    return SQLiteHealth(
        path=path,
        event_count=int(count),
        oldest_event_unix_ns=int(oldest),
        db_bytes=db_bytes,
    )
