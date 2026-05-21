"""linode_audit_summary tool tests.

Mirrors ``go/internal/tools/linode_audit_summary_test.go``. Drives the
handler against a temp JSONL log (SQLite path cleared) and checks the
default grouping, meta exclusion, and the bad-group_by path.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, Status
from linodemcp.profiles import Capability as ProfileCapability
from linodemcp.tools.linode_audit_summary import (
    create_linode_audit_summary_tool,
    handle_linode_audit_summary,
    set_audit_sqlite_path,
)

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
    """The tool advertises its name, CapMeta tag, and filter params."""
    tool, capability = create_linode_audit_summary_tool()

    assert tool.name == "linode_audit_summary"
    assert capability == ProfileCapability.Meta

    props = tool.inputSchema["properties"]
    for param in ("since", "group_by", "include_meta"):
        assert param in props


async def test_counts_by_tool_status(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The handler groups by tool+status and excludes meta by default."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")  # force the JSONL path

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)

    events = [
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 1),
        _event("linode_instance_list", Capability.READ, Status.SUCCESS, 2),
        _event("linode_instance_delete", Capability.DESTROY, Status.ERROR, 3),
        _event("linode_audit_recent", Capability.META, Status.SUCCESS, 4),
    ]
    body = "".join(json.dumps(event.to_dict()) + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    result = await handle_linode_audit_summary({})
    payload = json.loads(result[0].text)

    assert payload["total_events"] == 3, "meta event excluded by default"
    assert len(payload["rows"]) == 2
    assert payload["rows"][0]["groups"]["tool"] == "linode_instance_list"
    assert payload["rows"][0]["count"] == 2


async def test_invalid_group_by_returns_error() -> None:
    """An unknown group_by column surfaces as an error message."""
    result = await handle_linode_audit_summary({"group_by": ["bogus"]})

    assert len(result) == 1
    assert "bogus" in result[0].text
