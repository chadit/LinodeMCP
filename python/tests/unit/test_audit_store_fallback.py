"""A configured SQLite store that will not answer falls back, and says so.

Go used to answer a failure for an unopenable audit database while Python
silently answered from the JSONL log, so the same deployment reported different
data depending on which binary served it. Neither is right for an audit
surface: it must not change data source without saying so, and it must not go
dark when a secondary index is broken. Both languages now fall back and carry a
warning naming the configured path and the reason.

Mirrors ``go/internal/tools/linode_audit_store_fallback_test.go``.
"""

from __future__ import annotations

import dataclasses
import json
import os
import stat
from datetime import UTC, datetime
from pathlib import Path
from typing import Any, cast

import pytest

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    Status,
    event_timestamp,
    resolve_default_audit_dir,
    resolve_sqlite_path,
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
    record_audit_event,
)
from linodemcp.gentools.audit import (
    handle_linode_audit_export,
    handle_linode_audit_health,
    handle_linode_audit_report,
    handle_linode_audit_summary,
)

# The phrase every fallback warning ends with, whichever tool answered it.
_STORE_WARNING_MARK = "answered from the JSONL log instead"


def _reporting_config(reports: dict[str, ReportConfig]) -> Config:
    """A config carrying the report catalog, which is where the tool reads it."""
    return Config(audit=AuditConfig(reports=reports))


def _event(second: int) -> Event:
    """One read event at a distinct second so write order equals time order."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=event_timestamp(ts),
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool="linode_instance_list",
        tool_capability=Capability.READ.value,
        environment="default",
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
        session_id="session-1",
        credential_generation=1,
    )


@pytest.fixture
def unopenable_store(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Config:
    """Config naming a SQLite file that is not a database, JSONL beside it."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text(
        "".join(record_audit_event(_event(second), "", "") + "\n" for second in (1, 2)),
        encoding="utf-8",
    )

    db_path = audit_dir / "audit.db"
    db_path.write_text("not a database", encoding="utf-8")

    return dataclasses.replace(
        Config(),
        audit=dataclasses.replace(
            AuditConfig(),
            sqlite=AuditSQLiteConfig(enabled=True, path=str(db_path)),
        ),
    )


def _payload(text: str) -> dict[str, Any]:
    """Parse a handler's JSON answer, refusing an error result outright."""
    assert not text.startswith("Error: "), text
    parsed: object = json.loads(text)
    assert isinstance(parsed, dict)
    return cast("dict[str, Any]", parsed)


def _assert_names_the_store(warnings: object, db_path: str) -> None:
    """The one warning names the configured path and the source that answered."""
    assert isinstance(warnings, list)
    lines = cast("list[str]", warnings)
    assert len(lines) == 1, lines
    assert db_path in lines[0]
    assert _STORE_WARNING_MARK in lines[0]


@pytest.mark.asyncio
async def test_summary_falls_back_to_jsonl(unopenable_store: Config) -> None:
    """The summary counts the JSONL events and states the swap."""
    result = await handle_linode_audit_summary({}, unopenable_store)

    payload = _payload(result[0].text)
    assert payload["total_events"] == 2
    _assert_names_the_store(payload["warnings"], unopenable_store.audit.sqlite.path)


@pytest.mark.asyncio
async def test_export_falls_back_to_jsonl(unopenable_store: Config) -> None:
    """The export writes the JSONL events and states the swap."""
    result = await handle_linode_audit_export({"format": "json"}, unopenable_store)

    payload = _payload(result[0].text)
    assert payload["record_count"] == 2
    _assert_names_the_store(payload["warnings"], unopenable_store.audit.sqlite.path)


@pytest.mark.asyncio
async def test_health_falls_back_to_jsonl(unopenable_store: Config) -> None:
    """The health report keeps its JSONL half rather than answering a failure."""
    result = await handle_linode_audit_health({}, unopenable_store)

    payload = _payload(result[0].text)
    _assert_names_the_store(payload["warnings"], unopenable_store.audit.sqlite.path)


@pytest.mark.asyncio
async def test_no_warning_when_the_store_is_off(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """With no SQLite configured there is no swap, so no warning."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text(
        record_audit_event(_event(1), "", "") + "\n", encoding="utf-8"
    )

    result = await handle_linode_audit_summary({}, Config())

    assert _payload(result[0].text)["warnings"] == []


def test_store_path_defaults_beside_the_log(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """An enabled sink with no configured path reads audit.db beside the log.

    That is where the sink writes it, and Go derives the same path, so a
    deployment that names no path still points both languages at one file.
    """
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))

    cfg = dataclasses.replace(
        Config(),
        audit=dataclasses.replace(
            AuditConfig(), sqlite=AuditSQLiteConfig(enabled=True)
        ),
    )

    assert resolve_sqlite_path(cfg) == str(
        Path(resolve_default_audit_dir()) / "audit.db"
    )
    assert resolve_sqlite_path(Config()) == ""


@pytest.mark.asyncio
async def test_report_answers_a_read_failure(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    """A report whose store cannot be read answers the declared read failure.

    The sentence is declared on AuditReportInput, so its twin in the other
    language is the same case: one contract edit turns both red.
    """
    if hasattr(os, "geteuid") and os.geteuid() == 0:
        pytest.skip("root can list a mode-000 directory, so nothing fails")

    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text("", encoding="utf-8")

    cfg = _reporting_config(
        {
            "everything": ReportConfig(
                filter=ReportFilter(), group_by=["tool"], output=REPORT_OUTPUT_SUMMARY
            )
        }
    )
    audit_dir.chmod(0)

    try:
        result = await handle_linode_audit_report({"name": "everything"}, cfg)
    finally:
        audit_dir.chmod(stat.S_IRWXU)

    assert result[0].text.startswith(
        "Error: failed to run report: load report events: "
    )
