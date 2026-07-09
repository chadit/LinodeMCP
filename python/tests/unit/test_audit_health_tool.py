"""linode_audit_health tool tests.

Mirrors ``go/internal/tools/linode_audit_health_test.go``. Pins the
tool identity and drives the handler against a temp JSONL log (SQLite
path cleared) to confirm the report reflects the active log.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, Status
from linodemcp.profiles import Capability as ProfileCapability
from linodemcp.tools.linode_audit_health import (
    create_linode_audit_health_tool,
    handle_linode_audit_health,
)
from linodemcp.tools.linode_audit_summary import set_audit_sqlite_path

if TYPE_CHECKING:
    from pathlib import Path

    import pytest


def _event(tool: str, second: int) -> Event:
    """Build an event at a distinct second."""
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


def test_definition() -> None:
    """The tool advertises its name, CapMeta tag, and no input params."""
    tool, capability = create_linode_audit_health_tool()

    assert tool.name == "linode_audit_health"
    assert capability == ProfileCapability.Meta
    assert tool.inputSchema["properties"] == {}


async def test_reports_jsonl(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The handler reports the active JSONL log when SQLite is disabled."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")  # force the JSONL path

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    (audit_dir / "audit.log").write_text(
        json.dumps(_event("linode_instance_list", 1).to_dict()) + "\n",
        encoding="utf-8",
    )

    result = await handle_linode_audit_health({})
    report = json.loads(result[0].text)

    assert report["jsonl_path"] == str(audit_dir / "audit.log")
    assert report["active_log_exists"] is True
    assert report["rotated_file_count"] == 0
    assert report["dropped_events"] == 0
    assert "sqlite" not in report
