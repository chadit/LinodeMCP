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
import tempfile
from dataclasses import replace
from datetime import UTC, datetime
from pathlib import Path

import pytest

from linodemcp.audit import (
    DEFAULT_EXPORT_MAX_RECORDS,
    MAX_EXPORT_RECORDS,
    Capability,
    Event,
    Mode,
    RecentQuery,
    SQLiteSink,
    Status,
    UnknownExportFormatError,
    encode_events,
    event_timestamp,
    export_events,
    export_to_file,
    resolve_max_records,
)
from linodemcp.genlocal import (
    record_audit_event,
)

_DEFAULT_MAX = 10000


def _event(tool: str, second: int) -> Event:
    """Build an event at a distinct second in 2026-05-20."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool=tool,
        tool_capability=Capability.READ.value,
        environment="prod",
        profile="operator",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS.value,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="s1",
        credential_generation=1,
    )


def test_export_events_jsonl(tmp_path: Path) -> None:
    """The JSONL loader applies the tool glob and carries full args."""
    keep = replace(_event("linode_instance_list", 1), args={"region": "us-east"})
    drop = _event("linode_volume_list", 2)

    body = "".join(record_audit_event(e, "", "") + "\n" for e in (keep, drop))
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

    evt = replace(
        _event("linode_instance_delete", 1),
        tool_capability=Capability.DESTROY.value,
        args={"linode_id": 123, "confirm": True},
        args_redacted=["token"],
        error="boom",
    )
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


def test_export_events_sqlite_bound_past_the_nanosecond_range(
    tmp_path: Path,
) -> None:
    """A lower bound past the years a nanosecond count fits in an int64
    narrows the query instead of overflowing the SQLite binding, so both
    languages answer the same empty export.
    """
    db_path = tmp_path / "audit.db"
    sink = SQLiteSink(str(db_path), 5000)
    sink.write(_event("linode_instance_list", 1))
    sink.close()

    query = RecentQuery(
        limit=_DEFAULT_MAX,
        since=datetime(2999, 1, 1, tzinfo=UTC),
        include_meta=True,
    )

    assert export_events(str(db_path), str(tmp_path / "empty"), query) == []


def _encoded(events: list[Event], export_format: str) -> str:
    """One export as text, written through the handle the encoder takes."""
    sink = io.StringIO()
    encode_events(sink, events, export_format)

    return sink.getvalue()


def test_encode_json_round_trips() -> None:
    """JSON encodes to an array that decodes back to the events."""
    events = [_event("tool_a", 1)]
    decoded = json.loads(_encoded(events, "json"))

    assert len(decoded) == 1
    assert decoded[0]["tool"] == "tool_a"


def test_encode_ndjson_one_line_per_event() -> None:
    """NDJSON writes one JSON object per line."""
    events = [_event("tool_a", 1), _event("tool_b", 2)]
    text = _encoded(events, "ndjson")

    lines = text.rstrip("\n").split("\n")
    assert len(lines) == 2
    assert json.loads(lines[0])["tool"] == "tool_a"


def test_encode_csv_header_and_args_cell() -> None:
    """CSV writes a header plus a data row; args lands as a JSON cell."""
    evt = replace(_event("tool_a", 1), args={"region": "us-east"})

    records = list(csv.reader(io.StringIO(_encoded([evt], "csv"))))

    assert len(records) == 2, "header plus one data row"
    assert records[0][2] == "tool", "header column order"
    assert records[1][2] == "tool_a"
    assert "us-east" in records[1][-1], "args cell is JSON"


def test_encode_unknown_format_raises() -> None:
    """An unsupported format raises UnknownExportFormatError."""
    with pytest.raises(UnknownExportFormatError):
        encode_events(io.StringIO(), [], "xml")


def test_encode_json_empty_is_array() -> None:
    """An empty export renders as an empty JSON array, not null."""
    assert json.loads(_encoded([], "json")) == []


def test_export_to_file_lands_under_the_format_extension() -> None:
    """The written file is named for the format and carries the events."""
    path = Path(export_to_file([_event("tool_a", 1)], "json"))

    try:
        assert path.suffix == ".json"
        assert path.name.startswith("linode-audit-export-")
        assert json.loads(path.read_text(encoding="utf-8"))[0]["tool"] == "tool_a"
    finally:
        path.unlink(missing_ok=True)


def test_export_to_file_leaves_nothing_behind_when_the_encoder_refuses(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A format the encoder refuses removes the half-written file.

    Go's writer does the same, and a stray temp file under a name a caller was
    never told about is the one thing a failed export must not leave.

    tempfile.tempdir rather than the environment: gettempdir caches its answer
    on first use, so a TMPDIR set here would leave the export in the real temp
    directory and this case would pass while looking at an empty one.
    """
    monkeypatch.setattr(tempfile, "tempdir", str(tmp_path))

    with pytest.raises(UnknownExportFormatError):
        export_to_file([_event("tool_a", 1)], "xml")

    assert list(tmp_path.glob("linode-audit-export-*")) == []


@pytest.mark.parametrize(
    ("requested", "want"),
    [
        (0, DEFAULT_EXPORT_MAX_RECORDS),
        (-1, DEFAULT_EXPORT_MAX_RECORDS),
        (5, 5),
        (MAX_EXPORT_RECORDS + 1, MAX_EXPORT_RECORDS),
    ],
)
def test_resolve_max_records_applies_the_default_and_the_ceiling(
    requested: int, want: int
) -> None:
    """Zero or negative asks for the default; anything over the ceiling caps."""
    assert resolve_max_records(requested) == want
