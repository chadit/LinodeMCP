"""linode_audit_recent tool tests.

Mirrors ``go/internal/tools/linode_audit_recent_test.go``. Drives the
handler against a temp audit directory (via XDG_STATE_HOME) and checks
the response envelope, default meta exclusion, and the bad-timestamp
path.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, Status
from linodemcp.profiles import Capability as ProfileCapability
from linodemcp.tools.linode_audit_recent import (
    create_linode_audit_recent_tool,
    handle_linode_audit_recent,
)

if TYPE_CHECKING:
    from pathlib import Path

    import pytest


def _event(tool: str, capability: Capability, second: int) -> Event:
    """Build an event at a distinct second so write order equals time order."""
    ts = datetime(2026, 5, 20, 0, 0, second, tzinfo=UTC)
    return Event(
        ts=ts,
        ts_unix_ns=int(ts.timestamp() * 1_000_000_000),
        event_id="evt_" + tool,
        tool=tool,
        tool_capability=capability,
        environment="default",
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
        session_id="session-1",
        credential_generation=1,
    )


def test_definition() -> None:
    """The tool advertises its name, CapMeta tag, and filter params."""
    tool, capability = create_linode_audit_recent_tool()

    assert tool.name == "linode_audit_recent"
    assert capability == ProfileCapability.Meta

    props = tool.inputSchema["properties"]
    expected_params = (
        "limit",
        "since",
        "until",
        "tool",
        "capability",
        "status",
        "include_meta",
    )
    for param in expected_params:
        assert param in props, f"schema should declare the {param!r} filter"

    assert "confirm" not in props, "a read-only query must not declare confirm"


async def test_returns_events_meta_excluded(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The handler returns the envelope newest-first with meta excluded."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)

    events = [
        _event("linode_instance_list", Capability.READ, 1),
        _event("linode_audit_recent", Capability.META, 2),
        _event("linode_instance_delete", Capability.DESTROY, 3),
    ]
    body = "".join(json.dumps(event.to_dict()) + "\n" for event in events)
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    result = await handle_linode_audit_recent({})
    payload = json.loads(result[0].text)

    assert payload["count"] == 2, "meta event excluded by default leaves two"
    assert payload["events"][0]["tool"] == "linode_instance_delete", (
        "newest event (written last) must come first"
    )
    assert all(event["tool_capability"] != "meta" for event in payload["events"]), (
        "meta events excluded without include_meta"
    )


async def test_invalid_since_returns_error() -> None:
    """A malformed since surfaces an error message naming the parameter."""
    result = await handle_linode_audit_recent({"since": "not-a-timestamp"})

    assert len(result) == 1
    assert "since" in result[0].text, "error should name the bad parameter"
