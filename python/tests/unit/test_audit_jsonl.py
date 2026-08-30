"""Rolling JSONL audit sink tests.

Mirrors ``go/internal/audit/jsonl_test.go``. Tests define the sink's
contract: one JSON line per event, UTC-day rotation to gzip, drop on
write-after-close.
"""

from __future__ import annotations

import gzip
import io
import json
import os
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    ACTIVE_LOG_FILE_NAME,
    Capability,
    JSONLSink,
    JSONLSinkClosedError,
    Status,
    finalize,
    new_event,
)
from linodemcp.genlocal import record_audit_event

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
        event1 = finalize(event1, Status.SUCCESS, 12, "", "5 instances")
        sink.write(event1)

        event2 = _make_event("linode_instance_create", Capability.WRITE)
        event2 = finalize(event2, Status.ERROR, 45, "boom", "")
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


def test_jsonl_sink_writes_the_record_itself(tmp_path: Path) -> None:
    """Pin the line the sink appends to the record writer's own text.

    Writing anything else would carry the members in this module's own order
    rather than the message's, which is the divergence the record direction
    exists to close and which every other case here reads through key lookups
    that cannot see it.
    """
    sink = JSONLSink(str(tmp_path))
    try:
        event = _make_event("linode_instance_list", Capability.READ)
        sink.write(event)
    finally:
        sink.close()

    lines = [
        line
        for line in (tmp_path / "audit.log").read_text(encoding="utf-8").splitlines()
        if line
    ]

    assert lines == [record_audit_event(event, "", "")]


def test_jsonl_sink_rotates_on_day_boundary(tmp_path: Path) -> None:
    """Crossing UTC midnight rotates the prior day to audit-DATE.log.gz."""
    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)
    clock = _fixed_clock([day1, day1, day2, day2])

    sink = JSONLSink(str(tmp_path), clock=clock)
    try:
        day1_event = _make_event("linode_instance_list", Capability.READ)
        day1_event = finalize(day1_event, Status.SUCCESS, 10, "", "day-1-event")
        sink.write(day1_event)

        day2_event = _make_event("linode_instance_get", Capability.READ)
        day2_event = finalize(day2_event, Status.SUCCESS, 11, "", "day-2-event")
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


def _summarized_event(marker: str) -> Event:
    """A finalized read event whose result summary is ``marker``, which the
    rotation tests grep for to tell which file an event landed in."""
    event = _make_event("linode_instance_list", Capability.READ)
    return finalize(event, Status.SUCCESS, 10, "", marker)


@pytest.mark.parametrize(
    ("blocker", "kept_in"),
    [
        ("audit-2026-05-18.log", "audit.log"),
        ("audit-2026-05-18.log.gz", "audit-2026-05-18.log"),
    ],
)
def test_jsonl_sink_rotation_failure_keeps_the_triggering_event(
    tmp_path: Path, blocker: str, kept_in: str
) -> None:
    """A directory planted at the rename or gzip target reports the failed
    rotation and still writes the event whose write triggered it, with the
    closed day's data left readable."""
    (tmp_path / blocker).mkdir()

    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)
    captured: list[Exception] = []

    sink = JSONLSink(
        str(tmp_path),
        clock=_fixed_clock([day1, day1, day2, day2, day2]),
        on_write_error=captured.append,
    )
    try:
        sink.write(_summarized_event("day-1-event"))
        sink.write(_summarized_event("day-2-event"))
    finally:
        sink.close()

    assert any("rotate failed" in str(error) for error in captured)
    assert "day-1-event" in (tmp_path / kept_in).read_text(encoding="utf-8")
    assert "day-2-event" in Path(sink.path).read_text(encoding="utf-8")


def test_jsonl_sink_retries_rotation_after_the_blocker_clears(
    tmp_path: Path,
) -> None:
    """The rotation a blocked rename could not finish runs on the next write,
    carrying the events kept in audit.log meanwhile into the dated file."""
    blocker = tmp_path / "audit-2026-05-18.log"
    blocker.mkdir()

    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)

    sink = JSONLSink(str(tmp_path), clock=_fixed_clock([day1, day1, day2, day2, day2]))
    try:
        sink.write(_summarized_event("day-1-event"))
        sink.write(_summarized_event("day-2-event"))
        blocker.rmdir()
        sink.write(_summarized_event("day-3-event"))
    finally:
        sink.close()

    with gzip.open(tmp_path / "audit-2026-05-18.log.gz", "rt") as handle:
        body = handle.read()

    assert "day-1-event" in body
    assert "day-2-event" in body

    lines = _read_lines(sink.path)
    assert len(lines) == 1
    assert "day-3-event" in lines[0]


def test_jsonl_sink_close_failure_still_rotates_and_writes(tmp_path: Path) -> None:
    """A close that fails during rotation still rotates and still writes.

    Python closes the descriptor either way, so the rotation carries on. The
    handle has to be cleared regardless: leaving it in place sent the write
    that follows into a closed file, which raises a ValueError the caller has
    no way to catch.
    """
    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)
    captured: list[Exception] = []

    sink = _RefusesFirstClose(
        str(tmp_path),
        clock=_fixed_clock([day1, day1, day2, day2, day2]),
        on_write_error=captured.append,
    )
    try:
        sink.write(_summarized_event("day-1-event"))
        sink.write(_summarized_event("day-2-event"))
    finally:
        sink.close()

    assert captured == []

    with gzip.open(tmp_path / "audit-2026-05-18.log.gz", "rt") as handle:
        assert "day-1-event" in handle.read()

    assert "day-2-event" in Path(sink.path).read_text(encoding="utf-8")


class _ClosesLoudly(io.TextIOWrapper):
    """A text handle that closes the real file, then reports a failure.

    A close that fails needs a filesystem this suite cannot arrange, so the
    handle refuses instead. It closes underneath first, which is what the
    interpreter does when the flush inside ``close`` fails.
    """

    def close(self) -> None:
        """Close the wrapped file, then report the arranged failure."""
        super().close()
        msg = "close refused"
        raise OSError(msg)


class _RefusesFirstClose(JSONLSink):
    """A sink whose first active log handle reports a failure from ``close``.

    Only the first one, so the reopen the rotation performs and the sink's own
    ``close`` behave normally and the test measures the rotation alone.
    """

    _refused = False

    def _open_active_file(self) -> None:
        if self._refused:
            super()._open_active_file()
            return

        self._refused = True
        self._file = _ClosesLoudly(
            (self._dir / ACTIVE_LOG_FILE_NAME).open("ab"), encoding="utf-8"
        )


def test_jsonl_sink_reports_the_event_it_cannot_place(tmp_path: Path) -> None:
    """When a failed rotation cannot reopen the log either, the event is
    reported through the handler rather than dropped in silence."""
    if hasattr(os, "geteuid") and os.geteuid() == 0:
        pytest.skip("root writes into a mode-500 directory, so nothing fails")

    day1 = datetime(2026, 5, 18, 23, 59, 0, tzinfo=UTC)
    day2 = datetime(2026, 5, 19, 0, 0, 1, tzinfo=UTC)
    captured: list[Exception] = []

    sink = JSONLSink(
        str(tmp_path),
        clock=_fixed_clock([day1, day2, day2]),
        on_write_error=captured.append,
    )
    # With the active log gone the rename has nothing to move, and with the
    # directory read-only the sink cannot create a replacement either.
    Path(sink.path).unlink()
    tmp_path.chmod(0o500)
    try:
        sink.write(_summarized_event("day-2-event"))
    finally:
        tmp_path.chmod(0o700)

    assert any("rotate failed" in str(error) for error in captured)
    assert any("no active file" in str(error) for error in captured)

    # Once the directory takes writes again the next write opens a fresh log.
    sink.write(_summarized_event("day-3-event"))
    sink.close()

    assert "day-3-event" in Path(sink.path).read_text(encoding="utf-8")


def test_jsonl_sink_write_after_close_drops_event(tmp_path: Path) -> None:
    """Write after close routes the sentinel to the handler and drops."""
    captured: list[Exception] = []

    sink = JSONLSink(str(tmp_path), on_write_error=captured.append)
    sink.close()
    sink.close()  # idempotent

    event = _make_event("linode_instance_list", Capability.READ)
    event = finalize(event, Status.SUCCESS, 1, "", "")
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
