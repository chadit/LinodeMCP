"""Recent-events reader tests.

Mirrors ``go/internal/audit/reader_test.go``. Tests define the
newest-first ordering, limit clamp, filter dimensions, missing-dir,
and corrupt-line contracts.
"""

from __future__ import annotations

import gzip
from datetime import UTC, datetime, timedelta
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    DEFAULT_RECENT_LIMIT,
    Capability,
    Event,
    Mode,
    RecentQuery,
    Status,
    event_timestamp,
    read_recent,
)
from linodemcp.genlocal import (
    record_audit_event,
)

if TYPE_CHECKING:
    from pathlib import Path


def _event_at(
    tool: str,
    capability: Capability,
    status: Status,
    ts: datetime,
) -> Event:
    """Build an event at an explicit timestamp."""
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id="evt_" + tool,
        tool=tool,
        tool_capability=capability,
        environment="default",
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
        session_id="session-1",
        credential_generation=1,
    )


def _event(tool: str, capability: Capability, status: Status, hour: int) -> Event:
    """Build an event at a fixed May 2026 date and the given hour."""
    ts = datetime(2026, 5, 19, hour, 0, 0, tzinfo=UTC)
    return _event_at(tool, capability, status, ts)


def _write_jsonl(path: Path, gzipped: bool, events: list[Event]) -> None:
    """Write events as one JSON line each, oldest-first."""
    body = "".join(record_audit_event(event, "", "") + "\n" for event in events)
    if gzipped:
        with gzip.open(path, "wt", encoding="utf-8") as handle:
            handle.write(body)
    else:
        path.write_text(body, encoding="utf-8")


def test_read_recent_newest_first_across_files(tmp_path: Path) -> None:
    """Events come back newest-first across active and rotated files."""
    _write_jsonl(
        tmp_path / "audit-2026-05-18.log.gz",
        gzipped=True,
        events=[
            _event("tool_a", Capability.READ, Status.SUCCESS, 8),
            _event("tool_b", Capability.READ, Status.SUCCESS, 9),
        ],
    )
    _write_jsonl(
        tmp_path / "audit.log",
        gzipped=False,
        events=[
            _event("tool_c", Capability.READ, Status.SUCCESS, 10),
            _event("tool_d", Capability.READ, Status.SUCCESS, 11),
        ],
    )

    events = read_recent(str(tmp_path), RecentQuery())
    tools = [event.tool for event in events]

    assert tools == ["tool_d", "tool_c", "tool_b", "tool_a"]


def test_read_recent_limit_clamp(tmp_path: Path) -> None:
    """Explicit limit caps the result; limit 0 falls back to the default."""
    base = datetime(2026, 5, 19, 0, 0, 0, tzinfo=UTC)
    events = [
        _event_at(
            f"tool_{i}",
            Capability.READ,
            Status.SUCCESS,
            base + timedelta(minutes=i),
        )
        for i in range(50)
    ]
    _write_jsonl(tmp_path / "audit.log", gzipped=False, events=events)

    got = read_recent(str(tmp_path), RecentQuery(limit=10))
    assert len(got) == 10

    defaulted = read_recent(str(tmp_path), RecentQuery(limit=0))
    assert len(defaulted) == DEFAULT_RECENT_LIMIT


def test_read_recent_filters(tmp_path: Path) -> None:
    """Every filter dimension narrows the result correctly."""
    _write_jsonl(
        tmp_path / "audit.log",
        gzipped=False,
        events=[
            _event("linode_instance_list", Capability.READ, Status.SUCCESS, 8),
            _event("linode_instance_delete", Capability.DESTROY, Status.ERROR, 9),
            _event("linode_audit_recent", Capability.META, Status.SUCCESS, 10),
            _event("linode_volume_create", Capability.WRITE, Status.SUCCESS, 11),
        ],
    )

    default = read_recent(str(tmp_path), RecentQuery())
    assert len(default) == 3
    assert all(event.tool_capability != Capability.META for event in default)

    with_meta = read_recent(str(tmp_path), RecentQuery(include_meta=True))
    assert len(with_meta) == 4

    glob = read_recent(str(tmp_path), RecentQuery(tool="linode_instance_*"))
    assert len(glob) == 2

    destroy = read_recent(str(tmp_path), RecentQuery(capability=Capability.DESTROY))
    assert len(destroy) == 1
    assert destroy[0].tool == "linode_instance_delete"

    errored = read_recent(str(tmp_path), RecentQuery(status=Status.ERROR))
    assert len(errored) == 1

    window = read_recent(
        str(tmp_path),
        RecentQuery(
            since=datetime(2026, 5, 19, 9, 0, 0, tzinfo=UTC),
            until=datetime(2026, 5, 19, 10, 0, 0, tzinfo=UTC),
            include_meta=True,
        ),
    )
    assert len(window) == 2


@pytest.mark.parametrize(
    ("query", "want"),
    [
        (RecentQuery(since=datetime(2999, 1, 1, tzinfo=UTC)), 0),
        (RecentQuery(until=datetime(2999, 1, 1, tzinfo=UTC)), 1),
        (RecentQuery(since=datetime(1000, 1, 1, tzinfo=UTC)), 1),
        (RecentQuery(until=datetime(1000, 1, 1, tzinfo=UTC)), 0),
    ],
    ids=[
        "since past the range excludes the seeded event",
        "until past the range keeps the seeded event",
        "since before the range keeps the seeded event",
        "until before the range excludes the seeded event",
    ],
)
def test_read_recent_bounds_outside_the_nanosecond_range(
    tmp_path: Path, query: RecentQuery, want: int
) -> None:
    """A bound outside the years a Unix-nanosecond count fits in an int64
    keeps its meaning: far-future since matches nothing, far-future until
    matches everything, and the far past mirrors both.

    This path answers the same with or without the saturating bound, because a
    Python int does not wrap. The rows mirror the Go table so both languages
    pin one answer per bound; the bound itself is load-bearing for the SQLite
    readers, where test_audit_export and test_audit_summary pin it.
    """
    _write_jsonl(
        tmp_path / "audit.log",
        gzipped=False,
        events=[_event("tool_ok", Capability.READ, Status.SUCCESS, 8)],
    )

    assert len(read_recent(str(tmp_path), query)) == want


def test_read_recent_missing_dir_returns_empty(tmp_path: Path) -> None:
    """Querying before any audit exists is an empty result, not an error."""
    missing = tmp_path / "no-audit-yet"
    assert read_recent(str(missing), RecentQuery()) == []


def test_read_recent_skips_corrupt_lines(tmp_path: Path) -> None:
    """A malformed JSON line is skipped, not fatal."""
    good = _event("tool_ok", Capability.READ, Status.SUCCESS, 8)
    content = "{ this is not json\n" + record_audit_event(good, "", "") + "\n"
    (tmp_path / "audit.log").write_text(content, encoding="utf-8")

    got = read_recent(str(tmp_path), RecentQuery())
    assert len(got) == 1
    assert got[0].tool == "tool_ok"


def test_read_recent_skips_corrupt_gzip_header(tmp_path: Path) -> None:
    """A rotated .gz that is not gzip at all is skipped like an open
    failure, matching the Go reader, while other files still contribute
    events."""
    good = _event("tool_ok", Capability.READ, Status.SUCCESS, 8)
    (tmp_path / "audit.log").write_text(
        record_audit_event(good, "", "") + "\n", encoding="utf-8"
    )
    (tmp_path / "audit-2026-05-18.log.gz").write_bytes(b"not gzip data")

    got = read_recent(str(tmp_path), RecentQuery())
    assert [event.tool for event in got] == ["tool_ok"]


def test_read_recent_surfaces_truncated_rotated_file(tmp_path: Path) -> None:
    """A valid-header .gz cut mid-stream raises instead of dropping its
    events silently.

    Mirrors the Go reader: open and header failures skip the file, but a
    failure while reading an opened file surfaces.
    """
    lines = "".join(
        record_audit_event(
            _event(f"tool_{i}", Capability.READ, Status.SUCCESS, 8), "", ""
        )
        + "\n"
        for i in range(500)
    )
    payload = gzip.compress(lines.encode("utf-8"))
    truncated = payload[: len(payload) >> 1]  # keep half, cutting the stream
    (tmp_path / "audit-2026-05-18.log.gz").write_bytes(truncated)

    with pytest.raises(EOFError):
        read_recent(str(tmp_path), RecentQuery())
