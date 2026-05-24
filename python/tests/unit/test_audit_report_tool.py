"""linode_audit_report tool tests.

Mirrors ``go/internal/tools/linode_audit_report_test.go``. Covers the
tool identity, unknown-name rejection, the summary output path (with
capability_in post-filter), and list output capped at the report's
limit.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, Status
from linodemcp.config import (
    REPORT_OUTPUT_LIST,
    REPORT_OUTPUT_SUMMARY,
    ReportConfig,
    ReportFilter,
)
from linodemcp.profiles import Capability as ProfileCapability
from linodemcp.tools.linode_audit_report import (
    create_linode_audit_report_tool,
    handle_linode_audit_report,
    set_audit_reports,
)
from linodemcp.tools.linode_audit_summary import set_audit_sqlite_path

if TYPE_CHECKING:
    from pathlib import Path

    import pytest


def _event(tool: str, capability: Capability, status: Status, second: int) -> Event:
    """Build an event at a distinct second."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=ts,
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id=f"evt_{second}",
        tool=tool,
        tool_capability=capability,
        environment="prod",
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


def test_definition() -> None:
    """The tool advertises its name, CapMeta tag, and required name param."""
    tool, capability = create_linode_audit_report_tool()

    assert tool.name == "linode_audit_report"
    assert capability == ProfileCapability.Meta
    assert "name" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["name"]


async def test_unknown_report_returns_error() -> None:
    """A name with no matching catalog entry returns an error message."""
    set_audit_reports({})

    result = await handle_linode_audit_report({"name": "does-not-exist"})
    assert "does-not-exist" in result[0].text


async def test_summary_counts_with_capability_in(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """Summary output groups destroy events by tool, excluding reads."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    events = [
        _event("linode_instance_delete", Capability.DESTROY, Status.SUCCESS, 1),
        _event("linode_instance_delete", Capability.DESTROY, Status.SUCCESS, 2),
        _event("linode_volume_delete", Capability.DESTROY, Status.SUCCESS, 3),
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 4),
    ]
    body = "".join(json.dumps(event.to_dict()) + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    set_audit_reports(
        {
            "destroys": ReportConfig(
                filter=ReportFilter(capability_in=["destroy"]),
                output=REPORT_OUTPUT_SUMMARY,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "destroys"})
    payload = json.loads(result[0].text)

    assert payload["name"] == "destroys"
    assert payload["output"] == REPORT_OUTPUT_SUMMARY
    assert payload["total_events"] == 3, "three destroys match, read excluded"
    assert len(payload["rows"]) == 2
    assert payload["rows"][0]["count"] == 2


async def test_list_output_capped_at_limit(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """List output returns matching events capped at the report's limit."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    events = [
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 1),
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 2),
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 3),
    ]
    body = "".join(json.dumps(event.to_dict()) + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    set_audit_reports(
        {
            "recent-reads": ReportConfig(
                filter=ReportFilter(capability=Capability.READ.value),
                output=REPORT_OUTPUT_LIST,
                limit=2,
            ),
        }
    )

    result = await handle_linode_audit_report({"name": "recent-reads"})
    payload = json.loads(result[0].text)

    assert payload["output"] == REPORT_OUTPUT_LIST
    assert payload["total_events"] == 2
    assert len(payload["events"]) == 2
