"""The audit export operation, at the surface the generated arm calls.

The handler cases beside these read the sentence a caller sees, which the tool
declares. What they cannot see is the condition the operation raises, and that
is the whole of what the emitted except ladder catches: a method raising the
wrong one answers the wrong sentence with nothing failing. These read it
directly, and they reach the cap, the filters and the store fallback, which no
behavior fixture does.

Mirrors ``go/internal/tools/auditops_test.go``.
"""

from __future__ import annotations

import stat
import tempfile
from dataclasses import replace
from datetime import UTC, datetime, timedelta
from pathlib import Path

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    SQLiteSink,
    Status,
    event_timestamp,
    resolve_default_audit_dir,
)
from linodemcp.config import (
    REPORT_OUTPUT_SUMMARY,
    AuditConfig,
    AuditSQLiteConfig,
    Config,
    ReportConfig,
    ReportFilter,
)
from linodemcp.genlocal import (
    LocalInputRejectedError,
    LocalNotFoundError,
    LocalReadFailedError,
    LocalWriteFailedError,
    record_audit_event,
)
from linodemcp.tools.operations import (
    audit_export,
    audit_health,
    audit_recent,
    audit_report,
    audit_summary,
)

_LIST_TOOL = "linode_instance_list"
_VOLUME_TOOL = "linode_volume_list"
_BOOT_TOOL = "linode_instance_boot"
_NDJSON = "ndjson"


def _event(tool: str, second: int) -> Event:
    """One event at a distinct second so write order equals time order."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{tool}_{second}",
        tool=tool,
        tool_capability=Capability.READ.value,
        environment="default",
        profile="full-access",
        mode=Mode.NORMAL.value,
        plan_id=None,
        args={},
        args_redacted=[],
        status=Status.SUCCESS.value,
        latency_ms=1,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="sess-audit-operations",
        credential_generation=0,
    )


def _destroy_event(tool: str, second: int) -> Event:
    """One event a capability and a status filter can single out."""
    return replace(
        _event(tool, second),
        tool_capability=Capability.DESTROY.value,
        status=Status.ERROR.value,
    )


@pytest.fixture
def audit_dir(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """The seeded audit directory both the readers and the export resolve to.

    The export's own directory is set through tempfile.tempdir rather than the
    environment, since gettempdir caches its answer on first use and a TMPDIR
    set here would leave every export in the real temp directory.
    """
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))
    (tmp_path / "temp").mkdir()
    monkeypatch.setattr(tempfile, "tempdir", str(tmp_path / "temp"))

    directory = Path(resolve_default_audit_dir())
    directory.mkdir(parents=True, exist_ok=True)

    return directory


def _seed(directory: Path, events: list[Event]) -> None:
    """Write the events as one JSON line each, in slice order."""
    lines = [record_audit_event(event, "", "") for event in events]
    (directory / "audit.log").write_text("\n".join(lines) + "\n", encoding="utf-8")


def _remove(path: str) -> None:
    """Take the written export back out of the temp directory."""
    Path(path).unlink(missing_ok=True)


def test_audit_export_answers_the_window_it_wrote(audit_dir: Path) -> None:
    """Every seeded event reaches the file and the answer names where."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1), _event(_VOLUME_TOOL, 2)])

    answer = audit_export(Config(), _NDJSON, None, None, "", 0, include_meta=False)

    try:
        assert answer.record_count == 2
        assert answer.format == _NDJSON
        assert answer.warnings == []
        assert Path(answer.path).suffix == f".{_NDJSON}"
    finally:
        _remove(answer.path)


def test_audit_export_caps_the_records_it_reads(audit_dir: Path) -> None:
    """The bound that keeps an unbounded range out of memory.

    Nothing else exercises it: no fixture and no capture sends a cap.
    """
    _seed(
        audit_dir,
        [_event(_LIST_TOOL, 1), _event(_VOLUME_TOOL, 2), _event(_BOOT_TOOL, 3)],
    )

    answer = audit_export(Config(), _NDJSON, None, None, "", 1, include_meta=False)

    try:
        assert answer.record_count == 1
    finally:
        _remove(answer.path)


def test_audit_export_filters_by_the_bounds_it_is_handed(audit_dir: Path) -> None:
    """The filters reach the query rather than being dropped on the way."""
    _seed(
        audit_dir,
        [_event(_LIST_TOOL, 1), _event(_VOLUME_TOOL, 2), _event(_BOOT_TOOL, 3)],
    )

    window = datetime(2026, 5, 20, 0, 0, 2, tzinfo=UTC)

    answer = audit_export(
        Config(),
        _NDJSON,
        window,
        window + timedelta(minutes=1),
        "linode_instance_*",
        0,
        include_meta=False,
    )

    try:
        assert answer.record_count == 1
    finally:
        _remove(answer.path)


def test_audit_export_degrades_to_the_log_and_says_so(audit_dir: Path) -> None:
    """The store fallback through the operation, warning and all."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    unreadable = audit_dir / "not-a-database.db"
    unreadable.write_text("this is not a sqlite file", encoding="utf-8")

    cfg = Config(
        audit=AuditConfig(sqlite=AuditSQLiteConfig(enabled=True, path=str(unreadable)))
    )

    answer = audit_export(cfg, _NDJSON, None, None, "", 0, include_meta=False)

    try:
        assert answer.record_count == 1
        assert len(answer.warnings) == 1
        assert str(unreadable) in answer.warnings[0]
    finally:
        _remove(answer.path)


def test_audit_export_reports_a_store_that_will_not_read(audit_dir: Path) -> None:
    """The read condition, which the arm maps onto the tool's own sentence."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])
    audit_dir.chmod(0o000)

    try:
        with pytest.raises(LocalReadFailedError):
            audit_export(Config(), _NDJSON, None, None, "", 0, include_meta=False)
    finally:
        audit_dir.chmod(stat.S_IRWXU)


def test_audit_export_reports_a_format_the_encoder_refuses(audit_dir: Path) -> None:
    """A format the encoder does not know is the write condition, not a raise.

    The tool's own rule stands in front of this, so the path is reached only
    from a caller that never ran it. Go answers the same condition there, and an
    operation raising something else would leave a tenth language guessing.
    """
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    with pytest.raises(LocalWriteFailedError, match="xml"):
        audit_export(Config(), "xml", None, None, "", 0, include_meta=False)


# The three query operations, at the same surface. Same reason: the handler
# cases read the sentence, these read the condition, and they reach the
# configured store, its fallback and the group-by vocabulary, which no behavior
# fixture does.

_CAPABILITY_COLUMN = "capability"


def _sqlite_config(path: Path) -> Config:
    """A config naming a store at path, which the readers resolve on every
    call.
    """
    return Config(
        audit=AuditConfig(sqlite=AuditSQLiteConfig(enabled=True, path=str(path)))
    )


def _broken_store(audit_dir: Path) -> Path:
    """A store path holding what no driver will open, which is the fallback's
    own trigger.
    """
    unreadable = audit_dir / "not-a-database.db"
    unreadable.write_text("this is not a sqlite file", encoding="utf-8")

    return unreadable


def test_audit_health_answers_the_configured_store(audit_dir: Path) -> None:
    """The nested section no behavior fixture reaches: with the sink on, the
    answer carries the store's own row count beside the log's.
    """
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    store = audit_dir / "audit.db"
    sink = SQLiteSink(str(store), 5000)
    sink.write(_event(_LIST_TOOL, 1))
    sink.write(_event(_VOLUME_TOOL, 2))
    sink.close()

    answer = audit_health(_sqlite_config(store))

    assert answer.sqlite is not None
    assert answer.sqlite.event_count == 2
    assert answer.sqlite.path == str(store)
    assert answer.active_log_exists is True
    assert answer.warnings == []


def test_audit_health_degrades_to_the_log_and_says_so(audit_dir: Path) -> None:
    """The store's own degrade rule: the log still answers and the warning names
    the store.
    """
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    unreadable = _broken_store(audit_dir)

    answer = audit_health(_sqlite_config(unreadable))

    assert answer.sqlite is None
    assert len(answer.warnings) == 1
    assert str(unreadable) in answer.warnings[0]


def test_audit_health_reports_a_directory_it_cannot_read(audit_dir: Path) -> None:
    """The read condition, which the arm maps onto the tool's own sentence."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])
    audit_dir.chmod(0o000)

    try:
        with pytest.raises(LocalReadFailedError):
            audit_health(Config())
    finally:
        audit_dir.chmod(stat.S_IRWXU)


@pytest.mark.parametrize(
    ("tool", "capability", "status", "limit", "want"),
    [
        pytest.param("", "", "", 0, 3, id="every event"),
        pytest.param("linode_instance_*", "", "", 0, 2, id="one tool glob"),
        pytest.param("", "destroy", "", 0, 1, id="one capability"),
        pytest.param("", "", "error", 0, 1, id="one status"),
        pytest.param("", "", "", 2, 2, id="a limit below the count"),
    ],
)
def test_audit_recent_answers_the_filters_it_was_handed(
    audit_dir: Path, tool: str, capability: str, status: str, limit: int, want: int
) -> None:
    """The filters the shipped capture never sends reach the query."""
    _seed(
        audit_dir,
        [
            _event(_LIST_TOOL, 1),
            _event(_BOOT_TOOL, 2),
            _destroy_event(_VOLUME_TOOL, 3),
        ],
    )

    answer = audit_recent(limit, None, None, tool, capability, status, False)

    assert answer.count == want


def test_audit_recent_reports_a_directory_it_cannot_read(audit_dir: Path) -> None:
    """The one condition this operation declares."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])
    audit_dir.chmod(0o000)

    try:
        with pytest.raises(LocalReadFailedError):
            audit_recent(0, None, None, "", "", "", False)
    finally:
        audit_dir.chmod(stat.S_IRWXU)


def test_audit_summary_counts_the_column_it_was_handed(audit_dir: Path) -> None:
    """A group-by the shipped capture never sends, read off the bucket."""
    _seed(
        audit_dir,
        [
            _event(_LIST_TOOL, 1),
            _event(_BOOT_TOOL, 2),
            _destroy_event(_VOLUME_TOOL, 3),
        ],
    )

    answer = audit_summary(Config(), None, [_CAPABILITY_COLUMN], include_meta=False)

    assert answer.total_events == 3
    assert len(answer.rows) == 2
    assert answer.rows[0].groups[_CAPABILITY_COLUMN] == Capability.READ.value
    assert answer.rows[0].count == 2


def test_audit_summary_refuses_a_column_the_vocabulary_lacks(audit_dir: Path) -> None:
    """The condition answered before any store is opened, which is why it is a
    different one from a read.
    """
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    with pytest.raises(LocalInputRejectedError):
        audit_summary(Config(), None, ["nope"], include_meta=False)


def test_audit_summary_degrades_to_the_log_and_says_so(audit_dir: Path) -> None:
    """The store's degrade rule through the window reader rather than health."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    unreadable = _broken_store(audit_dir)

    answer = audit_summary(_sqlite_config(unreadable), None, [], include_meta=False)

    assert answer.total_events == 1
    assert len(answer.warnings) == 1


def test_audit_summary_reports_a_directory_it_cannot_read(audit_dir: Path) -> None:
    """The read condition, which only a directory nothing can open reaches."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])
    audit_dir.chmod(0o000)

    try:
        with pytest.raises(LocalReadFailedError):
            audit_summary(Config(), None, [], include_meta=False)
    finally:
        audit_dir.chmod(stat.S_IRWXU)


def _report_config(definition: ReportConfig | None = None) -> Config:
    """A config carrying one report named window, or none at all."""
    reports = {} if definition is None else {"window": definition}
    return Config(audit=AuditConfig(reports=reports))


def test_audit_report_refuses_a_name_the_configuration_does_not_declare(
    audit_dir: Path,
) -> None:
    """The lookup condition, which is answered before any store is opened."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    with pytest.raises(LocalNotFoundError):
        audit_report(_report_config(), datetime.now(UTC), "window")


def test_audit_report_reports_a_definition_apart_from_a_failed_read(
    audit_dir: Path,
) -> None:
    """The condition mapping the generated except ladder walks.

    The engine answers a definition it cannot take and a store it cannot read
    through one call, so this operation is where the two are told apart. A
    mapping that reported either as the other would answer the wrong sentence
    with nothing failing.
    """
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    unusable = ReportConfig(output=REPORT_OUTPUT_SUMMARY, group_by=["nope"])

    with pytest.raises(LocalInputRejectedError):
        audit_report(_report_config(unusable), datetime.now(UTC), "window")


def test_audit_report_reports_a_directory_it_cannot_read(audit_dir: Path) -> None:
    """The read condition, which only a directory nothing can open reaches."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])
    audit_dir.chmod(0o000)

    usable = ReportConfig(output=REPORT_OUTPUT_SUMMARY, filter=ReportFilter())

    try:
        with pytest.raises(LocalReadFailedError):
            audit_report(_report_config(usable), datetime.now(UTC), "window")
    finally:
        audit_dir.chmod(stat.S_IRWXU)


def test_audit_report_degrades_to_the_log_and_says_so(audit_dir: Path) -> None:
    """The store's degrade rule through the report, warning and all."""
    _seed(audit_dir, [_event(_LIST_TOOL, 1)])

    unreadable = _broken_store(audit_dir)
    definition = ReportConfig(output=REPORT_OUTPUT_SUMMARY, filter=ReportFilter())

    cfg = _sqlite_config(unreadable)
    cfg.audit.reports = {"window": definition}

    answer = audit_report(cfg, datetime.now(UTC), "window")

    assert answer.total_events == 1
    assert len(answer.warnings) == 1
