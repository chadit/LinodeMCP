"""Rolling JSONL audit sink tests.

Mirrors ``go/internal/audit/jsonl_test.go``. Tests define the sink's
contract: one JSON line per event, UTC-day rotation to gzip, drop on
write-after-close.
"""

from __future__ import annotations

import gzip
import json
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit import (
    Capability,
    JSONLSink,
    JSONLSinkClosedError,
    Status,
    new_event,
)

if TYPE_CHECKING:
    from collections.abc import Callable

    from linodemcp.audit import Event

_TEST_PROFILE = "operator"


def _make_event(tool: str, capability: Capability) -> Event:
    """Build an event with the fields tests don't care about filled in."""
    return new_event(
        tool,
        capability,
        {},
        "default",
        _TEST_PROFILE,
        "session-1",
        1,
        "0.1.0",
    )


def _read_lines(path: str) -> list[str]:
    """Return non-empty lines from the file at ``path``."""
    with Path(path).open(encoding="utf-8") as handle:
        return [line.strip() for line in handle if line.strip()]


def _fixed_clock(times: list[datetime]) -> Callable[[], datetime]:
    """Return a clock callable that walks ``times`` then sticks on the last."""
    state = {"index": 0}

    def clock() -> datetime:
        idx = min(state["index"], len(times) - 1)
        state["index"] += 1
        return times[idx]

    return clock


def test_jsonl_sink_appends_one_line_per_event(tmp_path: Path) -> None:
    """Every write must produce exactly one newline-terminated JSON line."""
    sink = JSONLSink(str(tmp_path))
    try:
        event1 = _make_event("linode_instance_list", Capability.READ)
        event1.finalize(Status.SUCCESS, 12, "", "5 instances")
        sink.write(event1)

        event2 = _make_event("linode_instance_create", Capability.WRITE)
        event2.finalize(Status.ERROR, 45, "boom", "")
        sink.write(event2)
    finally:
        sink.close()

    lines = _read_lines(sink.path)
    assert len(lines) == 2

    got1 = json.loads(lines[0])
    got2 = json.loads(lines[1])

    assert got1["tool"] == "linode_instance_list"
    assert got1["status"] == "success"
    assert got1["latency_ms"] == 12

    assert got2["tool"] == "linode_instance_create"
    assert got2["status"] == "error"
    assert got2["error"] == "boom"


def test_jsonl_sink_rotates_on_day_boundary(tmp_path: Path) -> None:
    """Crossing UTC midnight rotates the prior day to audit-DATE.log.gz."""
    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)
    clock = _fixed_clock([day1, day1, day2, day2])

    sink = JSONLSink(str(tmp_path), clock=clock)
    try:
        day1_event = _make_event("linode_instance_list", Capability.READ)
        day1_event.finalize(Status.SUCCESS, 10, "", "day-1-event")
        sink.write(day1_event)

        day2_event = _make_event("linode_instance_get", Capability.READ)
        day2_event.finalize(Status.SUCCESS, 11, "", "day-2-event")
        sink.write(day2_event)
    finally:
        sink.close()

    rotated = tmp_path / "audit-2026-05-18.log.gz"
    assert rotated.exists(), "rotated gzip must exist for the prior day"

    with gzip.open(rotated, "rt", encoding="utf-8") as handle:
        body = handle.read()
    assert "day-1-event" in body
    assert "day-2-event" not in body

    assert not (tmp_path / "audit-2026-05-18.log").exists(), (
        "uncompressed rotated file must be removed after gzip"
    )

    lines = _read_lines(sink.path)
    assert len(lines) == 1
    assert "day-2-event" in lines[0]


def test_jsonl_sink_write_after_close_drops_event(tmp_path: Path) -> None:
    """Write after close routes the sentinel to the handler and drops."""
    captured: list[Exception] = []

    sink = JSONLSink(str(tmp_path), on_write_error=captured.append)
    sink.close()
    sink.close()  # idempotent

    event = _make_event("linode_instance_list", Capability.READ)
    event.finalize(Status.SUCCESS, 1, "", "")
    sink.write(event)

    assert len(captured) == 1
    assert isinstance(captured[0], JSONLSinkClosedError)


def test_jsonl_sink_path_points_at_active_log(tmp_path: Path) -> None:
    """The path property must report the active audit.log location."""
    sink = JSONLSink(str(tmp_path))
    try:
        assert sink.path == str(tmp_path / "audit.log")
    finally:
        sink.close()
