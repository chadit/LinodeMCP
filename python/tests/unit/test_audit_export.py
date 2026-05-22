"""Audit export tests.

Mirrors ``go/internal/audit/eventexport_test.go``. Covers the JSONL and
SQLite loaders (full-event reconstruction, tool-glob filtering) and the
three encoders (json/csv/ndjson) plus the unknown-format and
empty-export cases.
"""

from __future__ import annotations

import csv
import io
import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    RecentQuery,
    SQLiteSink,
    Status,
    UnknownExportFormatError,
    encode_events,
    export_events,
)

if TYPE_CHECKING:
    from pathlib import Path

_DEFAULT_MAX = 10000


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


def test_export_events_jsonl(tmp_path: Path) -> None:
    """The JSONL loader applies the tool glob and carries full args."""
    keep = _event("linode_instance_list", 1)
    keep.args = {"region": "us-east"}
    drop = _event("linode_volume_list", 2)

    body = "".join(json.dumps(e.to_dict()) + "\n" for e in (keep, drop))
    (tmp_path / "audit.log").write_text(body, encoding="utf-8")

    query = RecentQuery(limit=_DEFAULT_MAX, tool="linode_instance_*")
    events = export_events("", str(tmp_path), query)

    assert len(events) == 1, "glob excludes the volume event"
    assert events[0].tool == "linode_instance_list"
    assert events[0].args["region"] == "us-east"


def test_export_events_sqlite_full_record(tmp_path: Path) -> None:
    """The SQLite loader reconstructs the full event, including args and
    a nullable error, not just the summary columns.
    """
    db_path = tmp_path / "audit.db"
    sink = SQLiteSink(str(db_path), 5000)

    evt = _event("linode_instance_delete", 1)
    evt.tool_capability = Capability.DESTROY
    evt.args = {"linode_id": 123, "confirm": True}
    evt.args_redacted = ["token"]
    evt.error = "boom"
    sink.write(evt)
    sink.close()

    query = RecentQuery(limit=_DEFAULT_MAX, include_meta=True)
    events = export_events(str(db_path), str(tmp_path / "empty"), query)

    assert len(events) == 1
    got = events[0]
    assert got.tool == "linode_instance_delete"
    assert got.args["linode_id"] == 123
    assert got.args["confirm"] is True
    assert got.args_redacted == ["token"]
    assert got.error == "boom"


def test_encode_json_round_trips() -> None:
    """JSON encodes to an array that decodes back to the events."""
    events = [_event("tool_a", 1)]
    decoded = json.loads(encode_events(events, "json"))

    assert len(decoded) == 1
    assert decoded[0]["tool"] == "tool_a"


def test_encode_ndjson_one_line_per_event() -> None:
    """NDJSON writes one JSON object per line."""
    events = [_event("tool_a", 1), _event("tool_b", 2)]
    text = encode_events(events, "ndjson")

    lines = text.rstrip("\n").split("\n")
    assert len(lines) == 2
    assert json.loads(lines[0])["tool"] == "tool_a"


def test_encode_csv_header_and_args_cell() -> None:
    """CSV writes a header plus a data row; args lands as a JSON cell."""
    evt = _event("tool_a", 1)
    evt.args = {"region": "us-east"}

    records = list(csv.reader(io.StringIO(encode_events([evt], "csv"))))

    assert len(records) == 2, "header plus one data row"
    assert records[0][2] == "tool", "header column order"
    assert records[1][2] == "tool_a"
    assert "us-east" in records[1][-1], "args cell is JSON"


def test_encode_unknown_format_raises() -> None:
    """An unsupported format raises UnknownExportFormatError."""
    with pytest.raises(UnknownExportFormatError):
        encode_events([], "xml")


def test_encode_json_empty_is_array() -> None:
    """An empty export renders as an empty JSON array, not null."""
    assert json.loads(encode_events([], "json")) == []
