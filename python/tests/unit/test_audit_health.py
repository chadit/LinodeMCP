"""Audit health collector tests.

Mirrors ``go/internal/audit/health_test.go``. Covers the JSONL portion
(active-log detection, rotated count and oldest date, disk usage), the
SQLite portion (row count, oldest event, DB size), and the missing-dir
case reporting zeros rather than erroring.
"""

from __future__ import annotations

import gzip
import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    SQLiteSink,
    Status,
    collect_health,
)

if TYPE_CHECKING:
    from pathlib import Path


def _event(tool: str, second: int) -> Event:
    """Build an event at a distinct second in 2026-05-20."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=ts,
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool=tool,
        tool_capability=Capability.READ,
        environment="prod",
        profile="operator",
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="s1",
        credential_generation=1,
    )


def test_collect_health_jsonl(tmp_path: Path) -> None:
    """JSONL footprint: active log, one rotated file, oldest date, size."""
    active = tmp_path / "audit.log"
    active.write_text(
        json.dumps(_event("tool_a", 1).to_dict()) + "\n",
        encoding="utf-8",
    )

    rotated = tmp_path / "audit-2026-05-18.log.gz"
    with gzip.open(rotated, "wt", encoding="utf-8") as handle:
        handle.write(json.dumps(_event("tool_b", 2).to_dict()) + "\n")

    report = collect_health("", str(tmp_path))

    assert report.jsonl_path == str(active)
    assert report.active_log_exists is True
    assert report.rotated_file_count == 1
    assert report.oldest_rotated_date == "2026-05-18"
    assert report.disk_bytes > 0
    assert report.dropped_events == 0
    assert report.sqlite is None


def test_collect_health_sqlite(tmp_path: Path) -> None:
    """SQLite portion: row count, oldest timestamp, non-zero DB size."""
    db_path = tmp_path / "audit.db"
    sink = SQLiteSink(str(db_path), 5000)

    oldest = _event("tool_x", 1)
    newer = _event("tool_x", 5)
    newer.event_id = "evt_newer"
    sink.write(oldest)
    sink.write(newer)
    sink.close()

    report = collect_health(str(db_path), str(tmp_path / "empty"))

    assert report.sqlite is not None
    assert report.sqlite.event_count == 2
    assert report.sqlite.oldest_event_unix_ns == oldest.ts_unix_ns
    assert report.sqlite.db_bytes > 0
    assert report.sqlite.path == str(db_path)


def test_collect_health_missing_dir_is_empty(tmp_path: Path) -> None:
    """An absent JSONL directory reports zeros, not an error."""
    report = collect_health("", str(tmp_path / "no-audit-yet"))

    assert report.active_log_exists is False
    assert report.rotated_file_count == 0
    assert report.oldest_rotated_date == ""
    assert report.disk_bytes == 0
