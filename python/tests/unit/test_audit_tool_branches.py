"""Error and filter branch coverage for the audit query tools.

The happy paths live in ``test_audit_{export,report,summary}_tool.py``. This
file drives the branches those skip: malformed timestamps, the max-records
cap, the empty-name and bad-group-by guards, and the report post-filters
(status_in, environment glob) plus the since/until/since_offset time bounds.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import (
    Capability,
    Event,
    Mode,
    Status,
)
from linodemcp.config import (
    REPORT_OUTPUT_LIST,
    REPORT_OUTPUT_SUMMARY,
    ReportConfig,
    ReportFilter,
)
from linodemcp.tools.linode_audit_export import handle_linode_audit_export
from linodemcp.tools.linode_audit_report import (
    handle_linode_audit_report,
    set_audit_reports,
)
from linodemcp.tools.linode_audit_summary import (
    handle_linode_audit_summary,
    set_audit_sqlite_path,
)

if TYPE_CHECKING:
    from pathlib import Path

    import pytest


def _event(
    tool: str,
    status: Status,
    environment: str,
    second: int,
) -> Event:
    """Build an event at a distinct second with a chosen status/environment."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=ts,
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool=tool,
        tool_capability=Capability.READ,
        environment=environment,
        profile="operator",
        mode=Mode.NORMAL,
        plan_id=None,
        args={},
        args_redacted=[],
        status=status,
        latency_ms=0,
        result_summary="",
        error=None,
        linodemcp_version="0.1.0",
        session_id="s1",
        credential_generation=1,
    )


def _write_log(tmp_path: Path, events: list[Event]) -> None:
    """Write events as a JSONL audit.log under the XDG state dir."""
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True, exist_ok=True)
    body = "".join(json.dumps(event.to_dict()) + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")


# --- export tool ---------------------------------------------------------


async def test_export_rejects_malformed_since() -> None:
    """A non-RFC-3339 since surfaces as an error, not a written file."""
    result = await handle_linode_audit_export({"format": "json", "since": "garbage"})

    assert len(result) == 1
    assert "invalid timestamp" in result[0].text
    assert "garbage" in result[0].text


async def test_export_caps_record_count_to_max_records(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """max_records bounds how many events the export writes: six on disk, a
    requested cap of five, five in the exported file. Proves the resolved
    limit is threaded into the query rather than ignored."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod", second)
            for second in range(6)
        ],
    )

    result = await handle_linode_audit_export({"format": "json", "max_records": 5})

    payload = json.loads(result[0].text)
    assert payload["record_count"] == 5


# --- report tool ---------------------------------------------------------


async def test_report_empty_name_errors() -> None:
    """An empty name is rejected before catalog lookup."""
    result = await handle_linode_audit_report({"name": ""})

    assert "report name is required" in result[0].text


async def test_report_bad_group_by_errors(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A summary report grouping on an unknown column surfaces the error."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(tmp_path, [])

    set_audit_reports(
        {
            "broken": ReportConfig(
                filter=ReportFilter(),
                output=REPORT_OUTPUT_SUMMARY,
                group_by=["bogus"],
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "broken"})
    assert "bogus" in result[0].text


async def test_report_status_in_post_filter(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """status_in keeps only matching-status events after load."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod", 1),
            _event("linode_instance_list", Status.ERROR, "prod", 2),
            _event("linode_instance_list", Status.ERROR, "prod", 3),
        ],
    )

    set_audit_reports(
        {
            "errors": ReportConfig(
                filter=ReportFilter(status_in=["error"]),
                output=REPORT_OUTPUT_LIST,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "errors"})
    payload = json.loads(result[0].text)
    assert payload["total_events"] == 2, "two error events, the success dropped"


async def test_report_environment_glob_post_filter(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An environment glob keeps only matching-environment events."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod-us", 1),
            _event("linode_instance_list", Status.SUCCESS, "dev", 2),
        ],
    )

    set_audit_reports(
        {
            "prod-only": ReportConfig(
                filter=ReportFilter(environment="prod*"),
                output=REPORT_OUTPUT_LIST,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "prod-only"})
    payload = json.loads(result[0].text)
    assert payload["total_events"] == 1
    assert payload["events"][0]["environment"] == "prod-us"


async def test_report_absolute_until_bounds_window(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An absolute until drops events after the bound."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod", 1),
            _event("linode_instance_list", Status.SUCCESS, "prod", 2),
            _event("linode_instance_list", Status.SUCCESS, "prod", 3),
        ],
    )

    set_audit_reports(
        {
            "until-2": ReportConfig(
                filter=ReportFilter(until="2026-05-20T00:00:02+00:00"),
                output=REPORT_OUTPUT_LIST,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "until-2"})
    payload = json.loads(result[0].text)
    assert payload["total_events"] == 2, "seconds 1 and 2 kept, second 3 dropped"


async def test_report_absolute_since_bounds_window(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """An absolute since drops events before the bound."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod", 1),
            _event("linode_instance_list", Status.SUCCESS, "prod", 2),
            _event("linode_instance_list", Status.SUCCESS, "prod", 3),
        ],
    )

    set_audit_reports(
        {
            "since-2": ReportConfig(
                filter=ReportFilter(since="2026-05-20T00:00:02+00:00"),
                output=REPORT_OUTPUT_LIST,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "since-2"})
    payload = json.loads(result[0].text)
    assert payload["total_events"] == 2, "seconds 2 and 3 kept, second 1 dropped"


async def test_report_since_offset_excludes_old_events(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """A one-hour since_offset (now - 1h) excludes the fixed 2026-05-20 events.

    Proves since_offset is resolved and applied: were it ignored, all three
    events would return instead of none.
    """
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")
    _write_log(
        tmp_path,
        [
            _event("linode_instance_list", Status.SUCCESS, "prod", 1),
            _event("linode_instance_list", Status.SUCCESS, "prod", 2),
        ],
    )

    set_audit_reports(
        {
            "recent-1h": ReportConfig(
                filter=ReportFilter(since_offset="1h"),
                output=REPORT_OUTPUT_LIST,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "recent-1h"})
    payload = json.loads(result[0].text)
    assert payload["total_events"] == 0, "events older than an hour ago are excluded"


# --- summary tool --------------------------------------------------------


async def test_summary_rejects_malformed_since() -> None:
    """A non-RFC-3339 since surfaces as an error before any load."""
    result = await handle_linode_audit_summary({"since": "garbage"})

    assert len(result) == 1
    assert "invalid 'since' timestamp" in result[0].text
