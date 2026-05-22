"""Phase 3f audit export.

Loads a filtered window of full audit events and encodes them as JSON,
CSV, or NDJSON. Unlike :func:`load_window` (summary aggregation, which
reads only the grouped columns), this reconstructs complete events so
the export carries the whole record.

Mirrors ``go/internal/audit/export.go``.
"""

from __future__ import annotations

import csv
import io
import json
import sqlite3
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit.event import Capability, Event, Mode, Status
from linodemcp.audit.reader import event_matches, scan_matching

if TYPE_CHECKING:
    from linodemcp.audit.reader import RecentQuery

# Export format names accepted by encode_events.
EXPORT_FORMAT_JSON = "json"
EXPORT_FORMAT_CSV = "csv"
EXPORT_FORMAT_NDJSON = "ndjson"

# DEFAULT_EXPORT_MAX_RECORDS bounds an export when the caller does not
# ask for a cap; MAX_EXPORT_RECORDS is the hard ceiling so one call
# cannot pull an unbounded range into memory.
DEFAULT_EXPORT_MAX_RECORDS = 10000
MAX_EXPORT_RECORDS = 100000

# CSV column order, mirrored by _export_csv_row.
_CSV_HEADER = [
    "ts",
    "event_id",
    "tool",
    "tool_capability",
    "status",
    "environment",
    "profile",
    "mode",
    "latency_ms",
    "result_summary",
    "error",
    "plan_id",
    "session_id",
    "credential_generation",
    "args_redacted",
    "args",
]


class UnknownExportFormatError(ValueError):
    """Raised when an export names a format other than json/csv/ndjson."""


def export_events(sqlite_path: str, jsonl_dir: str, query: RecentQuery) -> list[Event]:
    """Return up to ``query.limit`` matching events, newest first. Reads
    SQLite when ``sqlite_path`` is non-empty (full-row reconstruction),
    else scans the JSONL directory.
    """
    if sqlite_path:
        return _export_from_sqlite(sqlite_path, query)

    return scan_matching(jsonl_dir, query, query.limit)


def _export_from_sqlite(path: str, query: RecentQuery) -> list[Event]:
    """Read full event rows from SQLite via a static parameterized query.

    The lower-bound on ts_unix_ns is parameterized; the remaining
    filters apply in Python via event_matches so the statement stays
    static. Rows come newest-first; the scan stops at the query limit.
    """
    since_ns = (
        int(query.since.astimezone(UTC).timestamp() * 1_000_000_000)
        if query.since
        else 0
    )

    conn = sqlite3.connect(path)
    try:
        cursor = conn.execute(
            "SELECT event_id, ts_unix_ns, tool, tool_capability, environment, "
            "profile, mode, plan_id, status, latency_ms, result_summary, error, "
            "linodemcp_version, session_id, credential_generation, args_json, "
            "args_redacted_json FROM events WHERE ts_unix_ns >= ? "
            "ORDER BY ts_unix_ns DESC",
            (since_ns,),
        )
        rows = cursor.fetchall()
    finally:
        conn.close()

    events: list[Event] = []
    for (
        event_id,
        ts_unix_ns,
        tool,
        tool_capability,
        environment,
        profile,
        mode,
        plan_id,
        status,
        latency_ms,
        result_summary,
        error,
        linodemcp_version,
        session_id,
        credential_generation,
        args_json,
        args_redacted_json,
    ) in rows:
        event = _event_from_export_row(
            event_id,
            ts_unix_ns,
            tool,
            tool_capability,
            environment,
            profile,
            mode,
            plan_id,
            status,
            latency_ms,
            result_summary,
            error,
            linodemcp_version,
            session_id,
            credential_generation,
            args_json,
            args_redacted_json,
        )
        if not event_matches(query, event):
            continue

        events.append(event)
        if len(events) >= query.limit:
            break

    return events


def _event_from_export_row(
    event_id: str,
    ts_unix_ns: int,
    tool: str,
    tool_capability: str,
    environment: str,
    profile: str,
    mode: str,
    plan_id: str | None,
    status: str,
    latency_ms: int,
    result_summary: str | None,
    error: str | None,
    linodemcp_version: str,
    session_id: str,
    credential_generation: int,
    args_json: str,
    args_redacted_json: str,
) -> Event:
    """Reconstruct a full Event from a SQLite export row, decoding the
    args/args_redacted JSON columns and rebuilding ts from ts_unix_ns.
    """
    return Event(
        ts=datetime.fromtimestamp(ts_unix_ns / 1_000_000_000, UTC),
        ts_unix_ns=ts_unix_ns,
        event_id=event_id,
        tool=tool,
        tool_capability=Capability(tool_capability),
        environment=environment,
        profile=profile,
        mode=Mode(mode),
        plan_id=plan_id,
        args=json.loads(args_json),
        args_redacted=json.loads(args_redacted_json),
        status=Status(status),
        latency_ms=latency_ms,
        result_summary=result_summary if result_summary is not None else "",
        error=error,
        linodemcp_version=linodemcp_version,
        session_id=session_id,
        credential_generation=credential_generation,
    )


def encode_events(events: list[Event], export_format: str) -> str:
    """Encode events in the named format. JSON is a single indented
    array; NDJSON one compact object per line; CSV a header row plus one
    row per event (args and args_redacted are JSON text in their cells).
    An unknown format raises UnknownExportFormatError.
    """
    if export_format == EXPORT_FORMAT_JSON:
        return json.dumps([event.to_dict() for event in events], indent=2)

    if export_format == EXPORT_FORMAT_NDJSON:
        return "".join(json.dumps(event.to_dict()) + "\n" for event in events)

    if export_format == EXPORT_FORMAT_CSV:
        return _encode_csv(events)

    msg = f"unknown export format: {export_format!r}"
    raise UnknownExportFormatError(msg)


def _encode_csv(events: list[Event]) -> str:
    """Write a header row then one row per event to a CSV string."""
    buffer = io.StringIO()
    writer = csv.writer(buffer)
    writer.writerow(_CSV_HEADER)

    for event in events:
        writer.writerow(_export_csv_row(event))

    return buffer.getvalue()


def _export_csv_row(event: Event) -> list[str]:
    """Flatten an event into CSV cells in _CSV_HEADER order. Nullable
    plan_id/error render as empty cells; args and args_redacted are
    compact JSON.
    """
    return [
        event.ts.isoformat().replace("+00:00", "Z"),
        event.event_id,
        event.tool,
        event.tool_capability.value,
        event.status.value,
        event.environment,
        event.profile,
        event.mode.value,
        str(event.latency_ms),
        event.result_summary,
        event.error or "",
        event.plan_id or "",
        event.session_id,
        str(event.credential_generation),
        json.dumps(event.args_redacted or []),
        json.dumps(event.args or {}),
    ]
