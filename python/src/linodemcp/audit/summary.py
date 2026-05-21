"""Audit summary aggregation.

Mirrors ``go/internal/audit/summary.go``. Counts events bucketed by a
set of group-by columns over a time window, reading from SQLite when a
path is given and falling back to the JSONL scan otherwise.
"""

from __future__ import annotations

import sqlite3
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit.event import Capability, Event, Mode, Status
from linodemcp.audit.reader import scan_events

if TYPE_CHECKING:
    from collections.abc import Callable

# Default group-by columns when the request omits them.
_DEFAULT_GROUP_BY = ("tool", "status")


class UnknownGroupByColumnError(ValueError):
    """Raised when a summary query requests a column not in the allowlist."""


@dataclass
class SummaryQuery:
    """Filters and grouping for a summary aggregation."""

    since: datetime | None = None
    group_by: list[str] = field(default_factory=lambda: list(_DEFAULT_GROUP_BY))
    include_meta: bool = False


@dataclass
class SummaryRow:
    """One aggregated bucket: grouped column values and the count."""

    groups: dict[str, str]
    count: int


def _column_accessor(name: str) -> Callable[[Event], str] | None:
    """Return the field extractor for a groupable column, or None if the
    name is not in the allowlist.
    """
    match name:
        case "tool":
            return lambda e: e.tool
        case "status":
            return lambda e: e.status.value
        case "capability":
            return lambda e: e.tool_capability.value
        case "profile":
            return lambda e: e.profile
        case "environment":
            return lambda e: e.environment
        case _:
            return None


def validate_group_by(group_by: list[str] | None) -> list[str]:
    """Validate group-by columns against the allowlist.

    An empty/None request defaults to ``[tool, status]``. An unknown
    column raises :class:`UnknownGroupByColumnError` so a typo surfaces
    rather than producing an empty grouping.
    """
    if not group_by:
        return list(_DEFAULT_GROUP_BY)

    for name in group_by:
        if _column_accessor(name) is None:
            msg = f"unknown group_by column: {name!r}"
            raise UnknownGroupByColumnError(msg)

    return group_by


def summarize(events: list[Event], group_by: list[str]) -> list[SummaryRow]:
    """Aggregate events into per-bucket counts grouped by the given
    columns. ``group_by`` must already be validated. Rows are sorted by
    count descending, then by grouped values, for deterministic output.
    """
    accessors = [(name, _column_accessor(name)) for name in group_by]
    counts: dict[tuple[str, ...], int] = {}
    groups_by_key: dict[tuple[str, ...], dict[str, str]] = {}

    for event in events:
        values: list[str] = []
        groups: dict[str, str] = {}
        for name, accessor in accessors:
            value = accessor(event) if accessor else ""
            values.append(value)
            groups[name] = value

        key = tuple(values)
        counts[key] = counts.get(key, 0) + 1
        groups_by_key.setdefault(key, groups)

    rows = [
        SummaryRow(groups=groups_by_key[key], count=count)
        for key, count in counts.items()
    ]
    rows.sort(key=lambda row: tuple(row.groups[name] for name in group_by))
    rows.sort(key=lambda row: row.count, reverse=True)
    return rows


def load_window(
    sqlite_path: str,
    jsonl_dir: str,
    since: datetime | None,
    include_meta: bool,
) -> list[Event]:
    """Return events at or after ``since`` (None = all), honoring
    ``include_meta``. Reads SQLite when ``sqlite_path`` is non-empty,
    else scans the JSONL directory.
    """
    if sqlite_path:
        return _load_window_sqlite(sqlite_path, since, include_meta)

    return scan_events(jsonl_dir, since, include_meta)


def _load_window_sqlite(
    path: str, since: datetime | None, include_meta: bool
) -> list[Event]:
    """Read windowed events from SQLite via a static parameterized query."""
    since_ns = int(since.astimezone(UTC).timestamp() * 1_000_000_000) if since else 0

    conn = sqlite3.connect(path)
    try:
        cursor = conn.execute(
            "SELECT tool, tool_capability, status, profile, environment, ts_unix_ns "
            "FROM events WHERE ts_unix_ns >= ? ORDER BY ts_unix_ns DESC",
            (since_ns,),
        )
        rows = cursor.fetchall()
    finally:
        conn.close()

    events: list[Event] = []
    for tool, capability, status, profile, environment, ts_unix_ns in rows:
        cap = Capability(capability)
        if not include_meta and cap == Capability.META:
            continue
        events.append(
            _event_from_row(tool, cap, status, profile, environment, ts_unix_ns)
        )

    return events


def _event_from_row(
    tool: str,
    capability: Capability,
    status: str,
    profile: str,
    environment: str,
    ts_unix_ns: int,
) -> Event:
    """Build a minimal Event from a summary-query row. Only the grouped
    columns are meaningful; the rest get zero values since aggregation
    ignores them.
    """
    return Event(
        ts=datetime.fromtimestamp(ts_unix_ns / 1_000_000_000, UTC),
        ts_unix_ns=ts_unix_ns,
        event_id="",
        tool=tool,
        tool_capability=capability,
        environment=environment,
        profile=profile,
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status(status),
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="",
        session_id="",
        credential_generation=0,
    )
