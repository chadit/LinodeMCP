"""Phase 3f audit export.

Loads a filtered window of full audit events and encodes them as JSON,
CSV, or NDJSON. Unlike :func:`load_window` (summary aggregation, which
reads only the grouped columns), this reconstructs complete events so
the export carries the whole record.

Mirrors ``go/internal/audit/export.go``.
"""

from __future__ import annotations

import csv
import json
import sqlite3
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit.event import Event, event_timestamp
from linodemcp.audit.reader import event_matches, scan_matching, unix_ns_bound
from linodemcp.genlocal import record_audit_event, record_value

if TYPE_CHECKING:
    from _typeshed import SupportsWrite

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

# One level of indentation in the JSON document, which is the two spaces that
# format has always carried.
_EXPORT_INDENT = "  "

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
    since_ns = unix_ns_bound(query.since) if query.since else 0

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
    args/args_redacted JSON columns and rebuilding the timestamp text from
    ts_unix_ns.

    The store keeps only the nanosecond count, so the row's timestamp text is
    written back in the record's own spelling rather than in a second one.
    """
    return Event(
        ts=event_timestamp(datetime.fromtimestamp(ts_unix_ns / 1_000_000_000, UTC)),
        ts_unix_ns=ts_unix_ns,
        event_id=event_id,
        tool=tool,
        tool_capability=tool_capability,
        environment=environment,
        profile=profile,
        mode=mode,
        plan_id=plan_id,
        args=json.loads(args_json),
        args_redacted=json.loads(args_redacted_json),
        status=status,
        latency_ms=latency_ms,
        result_summary=result_summary if result_summary is not None else "",
        error=error,
        linodemcp_version=linodemcp_version,
        session_id=session_id,
        credential_generation=credential_generation,
    )


def encode_events(
    out: SupportsWrite[str], events: list[Event], export_format: str
) -> None:
    """Write events to ``out`` in the named format. JSON is a single indented
    array; NDJSON one compact object per line; CSV a header row plus one
    row per event (args and args_redacted are JSON text in their cells).
    An unknown format raises UnknownExportFormatError.

    Writes into a handle rather than answering a string because an export lands
    in a file: the caller that has the handle is the one that has to report a
    write that failed part way, and a whole export held in memory to be handed
    over is a copy nothing needs.

    Every record goes through the contract's own writer, so an export written
    here and one written by another language carry the same bytes rather than
    the same values in two spellings.
    """
    if export_format == EXPORT_FORMAT_JSON:
        out.write(_json_document(events))
        return

    if export_format == EXPORT_FORMAT_NDJSON:
        for event in events:
            out.write(record_audit_event(event, "", "") + "\n")
        return

    if export_format == EXPORT_FORMAT_CSV:
        _encode_csv(out, events)
        return

    msg = f"unknown export format: {export_format!r}"
    raise UnknownExportFormatError(msg)


def _json_document(events: list[Event]) -> str:
    """The records as one indented array closed by a newline, which is the
    document shape this format has always had.
    """
    written = [
        "\n"
        + _EXPORT_INDENT
        + record_audit_event(event, _EXPORT_INDENT, _EXPORT_INDENT)
        for event in events
    ]

    return "[" + ",".join(written) + ("\n" if events else "") + "]\n"


def _encode_csv(out: SupportsWrite[str], events: list[Event]) -> None:
    """Write a header row then one row per event.

    The writer ends a row with a bare newline, which is the terminator this
    format carries in every language rather than the one a spreadsheet dialect
    would pick.
    """
    writer = csv.writer(out, lineterminator="\n")
    writer.writerow(_CSV_HEADER)

    for event in events:
        writer.writerow(_export_csv_row(event))


def _export_csv_row(event: Event) -> list[str]:
    """Flatten an event into CSV cells in _CSV_HEADER order. Nullable
    plan_id/error render as empty cells; args and args_redacted are
    compact JSON.
    """
    # The record's own value spelling rather than this language's default, so
    # a cell holding a map comes out the same in every language.
    return [
        event.ts,
        event.event_id,
        event.tool,
        event.tool_capability,
        event.status,
        event.environment,
        event.profile,
        event.mode,
        str(event.latency_ms),
        event.result_summary,
        event.error or "",
        event.plan_id or "",
        event.session_id,
        str(event.credential_generation),
        record_value(event.args_redacted or [], "", ""),
        record_value(event.args or {}, "", ""),
    ]


def resolve_max_records(requested: int) -> int:
    """Apply the default and the hard ceiling to a requested cap.

    Zero or negative means the caller asked for the default.
    """
    if requested <= 0:
        return DEFAULT_EXPORT_MAX_RECORDS

    return min(requested, MAX_EXPORT_RECORDS)


def export_to_file(events: list[Event], export_format: str) -> str:
    """Encode the events into a temp file named for the format, answering
    its path. The file is left for the caller to read; the OS reclaims the
    temp directory on its own schedule.

    The format doubles as the extension because the three the contract allows
    are spelled the way their files are. A format the encoder does not know
    reaches here only from a caller the contract's own rule does not stand in
    front of, and the half-written file is removed rather than named.
    """
    with tempfile.NamedTemporaryFile(
        mode="w",
        prefix="linode-audit-export-",
        suffix=f".{export_format}",
        delete=False,
        encoding="utf-8",
    ) as handle:
        try:
            encode_events(handle, events, export_format)
        except (OSError, ValueError):
            Path(handle.name).unlink(missing_ok=True)
            raise

        return handle.name
