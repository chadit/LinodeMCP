"""SQLite audit sink tests.

Mirrors ``go/internal/audit/sqlite_test.go``. Covers round-trip
insert/read, INSERT OR IGNORE idempotency, and NULL storage for
absent optional fields.
"""

from __future__ import annotations

import asyncio
import json
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, SQLiteSink, Status

if TYPE_CHECKING:
    from pathlib import Path

_ARG_KEY_LABEL = "label"
_ARG_KEY_TOKEN = "token"


def _event(
    *,
    event_id: str,
    tool: str,
    capability: Capability = Capability.WRITE,
    status: Status = Status.SUCCESS,
    plan_id: str | None = None,
    error: str | None = None,
    args: dict[str, object] | None = None,
    redacted: list[str] | None = None,
    ts: datetime | None = None,
) -> Event:
    """Build an event with the fields the SQLite tests assert on."""
    ts = ts or datetime(2026, 5, 20, 12, 0, 0, tzinfo=UTC)
    return Event(
        ts=ts,
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=event_id,
        tool=tool,
        tool_capability=capability,
        environment="prod",
        profile="operator",
        mode=Mode.NORMAL,
        plan_id=plan_id,
        args=args or {},
        args_redacted=redacted or [],
        status=status,
        latency_ms=42,
        result_summary="",
        error=error,
        linodemcp_version="0.1.0",
        session_id="session-1",
        credential_generation=1,
    )


def _open_sink(tmp_path: Path) -> SQLiteSink:
    """Open a SQLite sink at a temp DB path."""
    return SQLiteSink(str(tmp_path / "audit.db"), 5000)


def test_sqlite_sink_inserts_and_reads_back(tmp_path: Path) -> None:
    """An event written through the sink round-trips with JSON args."""
    sink = _open_sink(tmp_path)
    try:
        evt = _event(
            event_id="evt_one",
            tool="linode_instance_create",
            args={_ARG_KEY_LABEL: "web-1", "region": "us-east"},
            redacted=[_ARG_KEY_TOKEN],
        )
        sink.write(evt)

        row = sink.connection.execute(
            "SELECT tool, tool_capability, status, latency_ms, "
            "args_json, args_redacted_json FROM events WHERE event_id = ?",
            (evt.event_id,),
        ).fetchone()
    finally:
        sink.close()

    assert row is not None
    tool, capability, status, latency_ms, args_json, redacted_json = row
    assert tool == "linode_instance_create"
    assert capability == "write"
    assert status == "success"
    assert latency_ms == 42
    assert json.loads(args_json) == {_ARG_KEY_LABEL: "web-1", "region": "us-east"}
    assert json.loads(redacted_json) == [_ARG_KEY_TOKEN]


def test_sqlite_sink_ignores_duplicate_event_id(tmp_path: Path) -> None:
    """INSERT OR IGNORE keeps a re-delivered event idempotent."""
    sink = _open_sink(tmp_path)
    try:
        evt = _event(event_id="evt_dup", tool="linode_instance_list")
        sink.write(evt)
        sink.write(evt)

        count = sink.connection.execute(
            "SELECT COUNT(*) FROM events WHERE event_id = ?",
            (evt.event_id,),
        ).fetchone()[0]
    finally:
        sink.close()

    assert count == 1


def test_sqlite_sink_stores_nulls_for_absent_optionals(tmp_path: Path) -> None:
    """plan_id and error are NULL when the event's fields are None."""
    sink = _open_sink(tmp_path)
    try:
        evt = _event(
            event_id="evt_nulls",
            tool="linode_instance_list",
            plan_id=None,
            error=None,
        )
        sink.write(evt)

        plan_id, error = sink.connection.execute(
            "SELECT plan_id, error FROM events WHERE event_id = ?",
            (evt.event_id,),
        ).fetchone()
    finally:
        sink.close()

    assert plan_id is None
    assert error is None


def _count_rows(sink: SQLiteSink) -> int:
    """Return the total number of audit rows."""
    return int(sink.connection.execute("SELECT COUNT(*) FROM events").fetchone()[0])


def test_sqlite_sweep_retention_removes_expired_keeps_recent(tmp_path: Path) -> None:
    """Rows older than now - retention_days are deleted; recent kept."""
    now = datetime(2026, 5, 20, 12, 0, 0, tzinfo=UTC)
    sink = _open_sink(tmp_path)
    try:
        sink.write(_event(event_id="old_1", tool="t", ts=now - timedelta(days=30)))
        sink.write(_event(event_id="old_2", tool="t", ts=now - timedelta(days=15)))
        sink.write(_event(event_id="recent", tool="t", ts=now - timedelta(days=1)))

        removed = sink.sweep_retention(now, 14)

        assert removed == 2
        assert _count_rows(sink) == 1
    finally:
        sink.close()


def test_sqlite_sweep_retention_disabled_when_zero(tmp_path: Path) -> None:
    """retention_days <= 0 is a no-op even with ancient rows."""
    now = datetime(2026, 5, 20, 12, 0, 0, tzinfo=UTC)
    sink = _open_sink(tmp_path)
    try:
        sink.write(_event(event_id="ancient", tool="t", ts=now - timedelta(days=2000)))

        removed = sink.sweep_retention(now, 0)

        assert removed == 0
        assert _count_rows(sink) == 1
    finally:
        sink.close()


async def test_sink_drops_unserializable_args(tmp_path: Path) -> None:
    """An event whose args can't be JSON-encoded is logged and dropped."""
    sink = _open_sink(tmp_path)
    try:
        bad_args: dict[str, object] = {"blob": {1, 2, 3}}
        sink.write(_event(event_id="bad", tool="t", args=bad_args))
        count = _count_rows(sink)
    finally:
        sink.close()

    assert count == 0, "a marshal failure must drop the event, not insert it"


async def test_sink_insert_on_closed_db_is_dropped(tmp_path: Path) -> None:
    """A write after close is best-effort: the sqlite error is caught, not raised."""
    db = tmp_path / "audit.db"
    sink = SQLiteSink(str(db), 5000)
    sink.close()

    # Must not raise even though the connection is closed.
    sink.write(_event(event_id="after_close", tool="t"))

    verify = SQLiteSink(str(db), 5000)
    try:
        count = _count_rows(verify)
    finally:
        verify.close()

    assert count == 0, "the write against a closed connection must not persist"


async def test_run_retention_sweeps_in_loop_then_exits_on_cancel(
    tmp_path: Path,
) -> None:
    """run_retention deletes expired rows in the loop, then stops on cancel."""
    now = datetime.now(UTC)
    sink = _open_sink(tmp_path)
    try:
        sink.write(_event(event_id="ancient", tool="t", ts=now - timedelta(days=90)))
        sink.write(_event(event_id="fresh", tool="t", ts=now - timedelta(hours=1)))

        task = asyncio.create_task(sink.run_retention(14, 0.01))
        for _ in range(50):
            await asyncio.sleep(0.01)
            if _count_rows(sink) == 1:
                break

        assert _count_rows(sink) == 1, "the background sweep should drop the old row"

        task.cancel()
        await task
    finally:
        sink.close()


async def test_run_retention_swallows_db_error_and_keeps_looping(
    tmp_path: Path,
) -> None:
    """A sweep failure inside run_retention is logged; the loop stays alive."""
    db = tmp_path / "audit.db"
    sink = SQLiteSink(str(db), 5000)
    sink.write(_event(event_id="row", tool="t"))
    sink.close()

    task = asyncio.create_task(sink.run_retention(14, 0.01))
    await asyncio.sleep(0.05)
    assert not task.done(), "a closed-connection sweep error must not kill the loop"

    task.cancel()
    await task
