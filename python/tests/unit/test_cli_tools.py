"""Unit tests for ``linodemcp tools`` (list and show).

Discovery views over the registry. ``tools`` and ``tools show`` are
synchronous and take their streams as parameters, so the tests assert on
captured output and exit codes the way the profile-subcommand tests do.
"""

from __future__ import annotations

import io
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli.tools import run_tools_command
from linodemcp.server import get_tool_registry

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture
def tools_config_file(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> Path:
    """Write a read-only-default config and point the loader at it.

    ``tools`` (no --all) reads the active profile from the standard config
    path, so the env override is set here rather than passing a path.
    """
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestCLI\n"
        "environments:\n"
        "  default:\n"
        "    label: Default\n"
        "    linode:\n"
        "      apiUrl: https://api.linode.com/v4\n"
        "      token: test-token\n"
    )
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(config_file))
    return config_file


def test_tools_all_lists_full_registry(tools_config_file: Path) -> None:
    """`tools --all` lists every registered tool and reports the full count."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["--all"], stdout, stderr)
    assert code == 0
    out = stdout.getvalue()
    registry_count = len(get_tool_registry())
    assert f"{registry_count} tools (all)" in out
    # A known meta tool shows up with its capability.
    assert "version" in out
    assert "meta" in out


def test_tools_default_filters_by_active_profile(tools_config_file: Path) -> None:
    """`tools` (no flag) lists only the active profile's tools, fewer than all.

    The default profile is read-only, so its surface is a strict subset of the
    full registry; a write tool is absent.
    """
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command([], stdout, stderr)
    assert code == 0
    out = stdout.getvalue()
    assert "active profile" in out
    # A read tool is present; a write tool is filtered out under read-only.
    assert "linode_instance_list" in out
    assert "linode_instance_delete" not in out


def test_tools_default_is_subset_of_all(tools_config_file: Path) -> None:
    """The active-profile list is no larger than the full registry list."""
    all_out, _ = io.StringIO(), io.StringIO()
    run_tools_command(["--all"], all_out, io.StringIO())
    active_out = io.StringIO()
    run_tools_command([], active_out, io.StringIO())

    all_tools = _tool_names(all_out.getvalue())
    active_tools = _tool_names(active_out.getvalue())
    assert active_tools <= all_tools
    assert len(active_tools) < len(all_tools)


def _tool_names(listing: str) -> set[str]:
    """Pull the linode_* / meta tool names out of a `tools` listing body."""
    names: set[str] = set()
    for line in listing.splitlines():
        token = line.split(" ", 1)[0]
        if token and token[0].isalpha() and token not in ("name", "tools"):
            names.add(token)
    return names


def test_tools_show_prints_schema(tools_config_file: Path) -> None:
    """`tools show <tool>` prints capability, description, and the arg schema."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["show", "linode_instance_get"], stdout, stderr)
    assert code == 0
    out = stdout.getvalue()
    assert "Tool: linode_instance_get" in out
    assert "Capability: read" in out
    assert "instance_id" in out
    assert "required" in out


def test_tools_show_unknown_exits_usage_error(tools_config_file: Path) -> None:
    """`tools show` of an unknown tool is a usage error (exit 2)."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["show", "linode_made_up"], stdout, stderr)
    assert code == 2
    assert "unknown tool" in stderr.getvalue()


def test_tools_show_requires_one_arg(tools_config_file: Path) -> None:
    """`tools show` without exactly one tool name is a usage error."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["show"], stdout, stderr)
    assert code == 2
    assert "Usage" in stderr.getvalue()


def test_tools_unknown_arguments_exits_usage_error(tools_config_file: Path) -> None:
    """An unrecognized tools flag is a usage error, not a silent list."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["--bogus"], stdout, stderr)
    assert code == 2
    assert "unknown tools arguments" in stderr.getvalue()


def test_tools_list_config_load_failure_exits_usage_error(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """`tools` (active-profile filtered) reports a config-load failure as exit 2.

    The default list needs the config to resolve the active profile; a missing
    config file fails that load, so the command exits 2 with a message rather
    than listing an empty surface.
    """
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(tmp_path / "missing_config.yml"))
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command([], stdout, stderr)
    assert code == 2
    assert "load config" in stderr.getvalue()


def test_tools_all_does_not_need_config(
    tmp_path: Path,
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    """`tools --all` lists the registry even when no config exists.

    The full surface does not depend on the active profile, so a missing config
    must not block discovery.
    """
    monkeypatch.setenv("LINODEMCP_CONFIG_PATH", str(tmp_path / "missing_config.yml"))
    stdout, stderr = io.StringIO(), io.StringIO()
    code = run_tools_command(["--all"], stdout, stderr)
    assert code == 0
    assert "tools (all)" in stdout.getvalue()
