"""linode_audit_export tool tests.

Mirrors ``go/internal/tools/linode_audit_export_test.go``. Pins the
tool identity (format required) and drives the handler against a temp
JSONL log to confirm the response points at a file with one NDJSON
line per event, plus the unknown-format error path.
"""

from __future__ import annotations

import json
from datetime import UTC, datetime
from pathlib import Path
from typing import TYPE_CHECKING

from linodemcp.audit import Capability, Event, Mode, Status
from linodemcp.profiles import Capability as ProfileCapability
from linodemcp.tools.linode_audit_export import (
    create_linode_audit_export_tool,
    handle_linode_audit_export,
)
from linodemcp.tools.linode_audit_summary import set_audit_sqlite_path

if TYPE_CHECKING:
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
    """The tool advertises its name, CapMeta tag, and required format."""
    tool, capability = create_linode_audit_export_tool()

    assert tool.name == "linode_audit_export"
    assert capability == ProfileCapability.Meta
    assert "format" in tool.inputSchema["properties"]
    assert tool.inputSchema["required"] == ["format"]


async def test_writes_ndjson(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """The handler writes one NDJSON line per event to the temp file."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path))
    set_audit_sqlite_path("")  # force the JSONL path

    audit_dir = tmp_path / "linodemcp"
    audit_dir.mkdir(parents=True)
    body = "".join(
        json.dumps(e.to_dict()) + "\n"
        for e in (_event("linode_instance_list", 1), _event("linode_volume_list", 2))
    )
    (audit_dir / "audit.log").write_text(body, encoding="utf-8")

    result = await handle_linode_audit_export({"format": "ndjson"})
    payload = json.loads(result[0].text)

    assert payload["format"] == "ndjson"
    assert payload["record_count"] == 2

    exported = Path(payload["path"])
    try:
        lines = exported.read_text(encoding="utf-8").rstrip("\n").split("\n")
        assert len(lines) == 2, "one NDJSON line per event"
    finally:
        exported.unlink(missing_ok=True)


async def test_unknown_format_returns_error() -> None:
    """An unsupported format surfaces as an error message, no file."""
    result = await handle_linode_audit_export({"format": "xml"})

    assert len(result) == 1
    assert "format must be one of: json, csv, ndjson" in result[0].text
