"""Audit summary aggregation tests.

Mirrors ``go/internal/audit/summary_test.go``. Covers group-by
validation, in-memory aggregation/sorting, and that the SQLite and
JSONL window sources agree.
"""

from __future__ import annotations

from datetime import UTC, datetime
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    SQLiteSink,
    Status,
    UnknownGroupByColumnError,
    event_timestamp,
    load_window,
    summarize,
    validate_group_by,
)
from linodemcp.genlocal import (
    record_audit_event,
)

if TYPE_CHECKING:
    from pathlib import Path


def _event(tool: str, capability: Capability, status: Status, hour: int) -> Event:
    """Build an event with the fields summary aggregation reads."""
    ts = datetime(2026, 5, 20, hour, 0, 0, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{tool}_{hour}",
        tool=tool,
        tool_capability=capability,
        environment="prod",
        profile="operator",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=status,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="s1",
        credential_generation=1,
    )


def test_validate_group_by_defaults() -> None:
    """An empty request defaults to [tool, status]."""
    assert validate_group_by(None) == ["tool", "status"]


def test_validate_group_by_accepts_allowed() -> None:
    """Allowlisted columns pass through in order."""
    assert validate_group_by(["capability", "profile"]) == ["capability", "profile"]


def test_validate_group_by_rejects_unknown() -> None:
    """An unknown column raises rather than producing an empty grouping."""
    with pytest.raises(UnknownGroupByColumnError):
        validate_group_by(["tool", "bogus"])


def test_summarize_counts_by_group() -> None:
    """Bucketing and count-descending ordering."""
    events = [
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 8),
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 9),
        _event("linode_instance_delete", Capability.DESTROY, Status.ERROR, 10),
    ]

    rows = summarize(events, ["tool", "status"])

    assert len(rows) == 2
    assert rows[0].groups["tool"] == "linode_instance_list"
    assert rows[0].count == 2
    assert rows[1].count == 1


def _write_jsonl(path: Path, events: list[Event]) -> None:
    body = "".join(record_audit_event(event, "", "") + "\n" for event in events)
    path.write_text(body, encoding="utf-8")


def test_load_window_jsonl_and_sqlite_agree(tmp_path: Path) -> None:
    """Both sources return the same windowed events and summary."""
    events = [
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 8),
        _event("linode_audit_recent", Capability.META, Status.SUCCESS, 9),
        _event("linode_instance_delete", Capability.DESTROY, Status.ERROR, 10),
    ]

    jsonl_dir = tmp_path / "jsonl"
    jsonl_dir.mkdir()
    _write_jsonl(jsonl_dir / "audit.log", events)

    jsonl_events = load_window("", str(jsonl_dir), None, include_meta=True)
    assert len(jsonl_events) == 3

    db_path = str(tmp_path / "audit.db")
    sink = SQLiteSink(db_path, 5000)
    for event in events:
        sink.write(event)
    sink.close()

    sqlite_events = load_window(db_path, "", None, include_meta=True)
    assert len(sqlite_events) == 3

    assert summarize(jsonl_events, ["tool"]) == summarize(sqlite_events, ["tool"])


def test_load_window_bound_past_the_nanosecond_range(tmp_path: Path) -> None:
    """A window starting past the years a nanosecond count fits in an int64
    reaches SQLite as a bound the column can hold, so it answers an empty
    window the way the Go twin does instead of failing the binding.
    """
    db_path = str(tmp_path / "audit.db")
    sink = SQLiteSink(db_path, 5000)
    sink.write(_event("linode_instance_list", Capability.READ, Status.SUCCESS, 8))
    sink.close()

    far_future = datetime(2999, 1, 1, tzinfo=UTC)

    assert load_window(db_path, "", far_future, include_meta=True) == []


def test_load_window_excludes_meta_by_default(tmp_path: Path) -> None:
    """include_meta=False drops meta events."""
    events = [
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 8),
        _event("linode_audit_recent", Capability.META, Status.SUCCESS, 9),
    ]
    _write_jsonl(tmp_path / "audit.log", events)

    got = load_window("", str(tmp_path), None, include_meta=False)
    assert len(got) == 1
    assert got[0].tool == "linode_instance_list"


def test_load_window_missing_dir_returns_empty(tmp_path: Path) -> None:
    """Querying before any audit exists is empty, not an error."""
    assert load_window("", str(tmp_path / "nope"), None, include_meta=True) == []
