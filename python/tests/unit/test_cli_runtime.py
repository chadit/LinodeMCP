"""Unit tests for the one-shot CLI server runtime.

The runtime builds a ``Server``, attaches the audit sink, and tears it down,
without starting the metrics endpoint or the config watcher. The tests confirm
the server is wired and that a dispatch through the runtime lands in the audit
log on disk (the whole reason the CLI attaches the sink).
"""

from __future__ import annotations

import json
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli.runtime import (
    OneShotRuntime,
    load_config_or_default,
    open_runtime,
)
from linodemcp.config import ConfigError
from linodemcp.profiles import DEFAULT_PROFILE_NAME, resolve_active_profile
from linodemcp.server import Server, get_tool_registry

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture
def runtime_config_file(tmp_path: Path) -> Path:
    """Write a minimal config the runtime can load."""
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestRuntime\n"
        "environments:\n"
        "  default:\n"
        "    label: Default\n"
        "    linode:\n"
        "      apiUrl: https://api.linode.com/v4\n"
        "      token: test-token\n"
    )
    return config_file


@pytest.fixture(autouse=True)
def isolate_audit(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    """Send the audit log to a temp dir so the disk-write assertion is isolated."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


def test_load_config_or_default_missing_falls_back(tmp_path: Path) -> None:
    """A missing config file yields the in-memory default, not an error.

    The default has an empty active profile (so the resolver selects the
    read-only ``default`` built-in) and no environments. This is the core of
    the no-config parity fix.
    """
    from linodemcp.profiles import ToolDescriptor

    cfg = load_config_or_default(tmp_path / "absent.yml")
    assert cfg.active_profile == ""
    assert cfg.environments == {}

    # The empty active profile resolves to the read-only default built-in.
    descriptors = [
        ToolDescriptor(name=e.name, capability=e.capability)
        for e in get_tool_registry()
    ]
    profile = resolve_active_profile(cfg, descriptors)
    assert profile.name == DEFAULT_PROFILE_NAME


def test_load_config_or_default_malformed_raises(tmp_path: Path) -> None:
    """A malformed config still raises; only file-not-found is swallowed."""
    bad = tmp_path / "bad.yml"
    bad.write_text("server: [not a mapping\n")
    with pytest.raises(ConfigError):
        load_config_or_default(bad)


def test_load_config_or_default_present_loads_it(runtime_config_file: Path) -> None:
    """A present, valid config is loaded as written (no fallback)."""
    cfg = load_config_or_default(runtime_config_file)
    assert cfg.server.name == "TestRuntime"


def test_create_builds_server(runtime_config_file: Path) -> None:
    """create loads the config and builds a Server without opening sinks."""
    runtime = OneShotRuntime.create(runtime_config_file)
    assert isinstance(runtime.server, Server)
    assert runtime.config.server.name == "TestRuntime"


async def test_open_runtime_dispatches_and_audits(
    runtime_config_file: Path,
    tmp_path: Path,
) -> None:
    """A dispatch through the runtime lands an event in the on-disk audit log.

    Confirms the CLI's reason for attaching the sink: a one-shot call is
    recorded the same way an MCP call is.
    """
    async with open_runtime(runtime_config_file) as runtime:
        result = await runtime.server.dispatch("version", {})
        assert result[0].text  # the version payload

    log_path = tmp_path / "state" / "linodemcp" / "audit.log"
    assert log_path.exists()
    lines = [line for line in log_path.read_text().splitlines() if line.strip()]
    assert lines, "expected at least one audit event written"
    event = json.loads(lines[-1])
    assert event["tool"] == "version"


async def test_runtime_with_sqlite_enabled(tmp_path: Path) -> None:
    """When audit.sqlite.enabled, the runtime opens the SQLite sink too.

    The dispatch still records and tears down cleanly with both sinks behind
    the MultiSink.
    """
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestRuntime\n"
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
    async with open_runtime(config_file) as runtime:
        result = await runtime.server.dispatch("version", {})
        assert result[0].text

    db_path = tmp_path / "state" / "linodemcp" / "audit.db"
    assert db_path.exists()


async def test_runtime_teardown_closes_cleanly(runtime_config_file: Path) -> None:
    """Entering and exiting the runtime context does not raise."""
    runtime = OneShotRuntime.create(runtime_config_file)
    async with runtime:
        assert runtime.server is not None
    # A second exit-style close path: the context manager already closed the
    # sinks; re-entering builds nothing new, so just assert the server stands.
    assert runtime.server is not None
