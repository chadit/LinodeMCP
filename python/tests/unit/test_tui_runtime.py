"""Tests for the TUI session runtime and runner entry point.

The runtime builds the server once, attaches the audit sink, and tears it down;
the runner opens the runtime and runs the app. These cover the lifecycle and
the audit wiring (a dispatch lands on disk) without a real terminal.
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from linodemcp.server import Server
from linodemcp.tui import runner
from linodemcp.tui.runtime import TuiRuntime, open_tui_runtime

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Point the audit log at a temp dir so the disk-write assertion is isolated."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def test_create_no_config_uses_offline_default(tmp_path: Path) -> None:
    """A missing config file yields the offline default (read-only profile)."""
    runtime = TuiRuntime.create(tmp_path / "absent.yml")
    assert isinstance(runtime.server, Server)
    # The offline default resolves the read-only 'default' profile.
    assert runtime.server.active_profile.name == "default"
    assert runtime.config.environments == {}


async def test_runtime_dispatch_is_audited(tmp_path: Path) -> None:
    """A dispatch through the runtime lands an event in the on-disk audit log.

    Confirms the runtime attaches the sink so a TUI call is recorded like an
    MCP call.
    """
    async with open_tui_runtime(tmp_path / "absent.yml") as runtime:
        result = await runtime.server.dispatch("version", {})
        assert result[0].text

    log_path = tmp_path / "state" / "linodemcp" / "audit.log"
    assert log_path.exists()
    lines = [line for line in log_path.read_text().splitlines() if line.strip()]
    assert lines
    assert json.loads(lines[-1])["tool"] == "version"


async def test_runtime_with_sqlite_enabled(tmp_path: Path) -> None:
    """When audit.sqlite.enabled, the runtime opens the SQLite sink too."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestTUI\n"
        "audit:\n"
        "  sqlite:\n"
        "    enabled: true\n"
        "environments:\n"
        "  default:\n"
        "    label: Default\n"
        "    linode:\n"
        "      apiUrl: https://api.linode.com/v4\n"
        "      token: test-token\n"
    )
    async with open_tui_runtime(config_file) as runtime:
        result = await runtime.server.dispatch("version", {})
        assert result[0].text

    assert (tmp_path / "state" / "linodemcp" / "audit.db").exists()


async def test_runtime_teardown_closes_cleanly(tmp_path: Path) -> None:
    """Entering and exiting the runtime context does not raise."""
    runtime = TuiRuntime.create(tmp_path / "absent.yml")
    async with runtime:
        assert runtime.server is not None
    assert runtime.server is not None


def test_run_tui_opens_runtime_and_runs_app(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """``run_tui`` builds the runtime, runs the app, and returns 0.

    The app's ``run_async`` is replaced with a no-op that records it ran, so the
    full sync entry exercises the runtime open/teardown and the app launch
    without a real terminal. Going through the public entry avoids reaching into
    the private async helper.
    """
    started: list[bool] = []

    async def _fake_run_async(self: object) -> None:
        started.append(True)

    monkeypatch.setattr(
        "linodemcp.tui.app.LinodeTUI.run_async", _fake_run_async, raising=True
    )
    code = runner.run_tui(tmp_path / "absent.yml")
    assert code == 0
    assert started == [True]
