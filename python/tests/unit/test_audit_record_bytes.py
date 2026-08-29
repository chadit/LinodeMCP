"""The two durable formats, held to the bytes the shared fixture carries.

Mirrors ``go/internal/audit/recordbytes_test.go``. A log line and an export are
files an operator's own tooling reads, and until now each language wrote them in
its own spelling: one member order for the log, a different line terminator for
the CSV, a trailing newline on one JSON document and not the other. The fixture
under ``testdata/audit/record-bytes`` is the one answer both languages are held
to, so a tenth language has bytes to match rather than two spellings to choose
between.

What the fixture cannot hold is written down rather than hidden: the free-form
args member is each language's own JSON for values the contract declares
free-form, so a value carrying a backspace or a form feed comes out differently
in the two languages and no declaration says which is right. Every value the
fixture carries is inside what both spell alike.
"""

from __future__ import annotations

import io
import json
from pathlib import Path

import pytest

from linodemcp.audit import (
    EXPORT_FORMAT_CSV,
    EXPORT_FORMAT_JSON,
    EXPORT_FORMAT_NDJSON,
    Event,
    encode_events,
)
from linodemcp.genlocal import audit_event_from_record, record_audit_event

_FIXTURE_DIR = (
    Path(__file__).resolve().parents[3] / "testdata" / "audit" / "record-bytes"
)


def _fixture(name: str) -> str:
    """One fixture file's text."""
    return (_FIXTURE_DIR / name).read_text(encoding="utf-8")


def _fixture_events() -> list[Event]:
    """The shared source read through this language's own record reader.

    Reading rather than constructing is what keeps a number's written spelling
    rather than the one a decoder would widen it to.
    """
    source = json.loads(_fixture("events.json"))

    return [
        audit_event_from_record(json.dumps(raw, ensure_ascii=False))
        for raw in source["events"]
    ]


def test_record_writes_the_shared_log_bytes() -> None:
    """The JSONL sink's own line matches the fixture, member for member.

    The order is the contract's, so a language writing its dataclass order
    instead fails here rather than at whatever reads the log later.
    """
    written = "".join(
        record_audit_event(event, "", "") + "\n" for event in _fixture_events()
    )

    assert written == _fixture("log.jsonl")


@pytest.mark.parametrize(
    ("export_format", "name"),
    [
        (EXPORT_FORMAT_JSON, "export.json"),
        (EXPORT_FORMAT_CSV, "export.csv"),
        (EXPORT_FORMAT_NDJSON, "export.ndjson"),
    ],
)
def test_export_writes_the_shared_document_bytes(export_format: str, name: str) -> None:
    """All three export documents match the fixture: the CSV terminator, the
    JSON document's trailing newline and the member order inside every record.
    """
    out = io.StringIO()
    encode_events(out, _fixture_events(), export_format)

    assert out.getvalue() == _fixture(name)


def test_record_reads_back_what_it_wrote() -> None:
    """A line the writer produced hydrates into the record it came from."""
    for event in _fixture_events():
        line = record_audit_event(event, "", "")

        assert record_audit_event(audit_event_from_record(line), "", "") == line
