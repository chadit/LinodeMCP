"""Unit tests for the TUI run module (dispatch + classify).

``run.execute`` drives the live server's dispatch and classifies the result.
These tests use a real ``Server`` built from a temp config (meta tools need no
token) so the classification covers success, tool-error, and refused without a
terminal.
"""

from __future__ import annotations

from typing import TYPE_CHECKING

import pytest

from linodemcp.config import Config
from linodemcp.server import Server
from linodemcp.tui.run import RunStatus, execute

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Send the audit log to a temp dir so dispatching in tests is isolated."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def _server() -> Server:
    """Build a server on the offline default config (read-only profile)."""
    return Server(Config())


async def test_execute_success_meta_tool() -> None:
    """A meta tool returns SUCCESS with its payload as the text."""
    result = await execute(_server(), "hello", {"name": "Ada"})
    assert result.status is RunStatus.SUCCESS
    assert result.is_success is True
    assert "Ada" in result.text


async def test_execute_version_renders_json() -> None:
    """The version tool's JSON payload survives into the rendered output."""
    result = await execute(_server(), "version", {})
    assert result.status is RunStatus.SUCCESS
    assert '"version"' in result.rendered


async def test_execute_api_tool_offline_is_tool_error() -> None:
    """An API tool with no environment returns TOOL_ERROR, not a crash.

    Mirrors the CLI: the offline default has no environments, so the handler
    returns a clear no-environment error payload.
    """
    result = await execute(_server(), "linode_instance_list", {})
    assert result.status is RunStatus.TOOL_ERROR
    assert result.is_success is False
    assert "environment" in result.text.lower()


async def test_execute_refused_unknown_tool() -> None:
    """An unknown tool name is REFUSED (dispatch raises ValueError)."""
    result = await execute(_server(), "linode_not_a_real_tool", {})
    assert result.status is RunStatus.REFUSED
    assert "Unknown tool" in result.text


async def test_execute_refused_profile_filtered_tool() -> None:
    """A tool the read-only default filters out is REFUSED via dispatch.

    ``linode_instance_delete`` is a Destroy tool absent from the default
    profile's allow list, so dispatch refuses it rather than running it.
    """
    result = await execute(
        _server(), "linode_instance_delete", {"instance_id": 1, "dry_run": True}
    )
    assert result.status is RunStatus.REFUSED


async def test_execute_table_output() -> None:
    """A table output mode renders the object payload as a text table."""
    result = await execute(_server(), "version", {}, output="table")
    assert result.status is RunStatus.SUCCESS
    # Table output is not raw JSON, so it should not start with a brace.
    assert not result.rendered.lstrip().startswith("{")
    assert "version" in result.rendered
