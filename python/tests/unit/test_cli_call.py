"""Unit tests for ``linodemcp call`` and the CLI-to-registry parity gate.

The command is driven with captured stdout/stderr and a temp config so the
tests assert on output text and the exit code without touching the real config
or audit log. Meta tools (version, hello) need no Linode token, so the
dispatch-equivalence test runs against them.
"""

from __future__ import annotations

import io
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli.call import run_call_command
from linodemcp.server import Server, get_tool_registry

if TYPE_CHECKING:
    from pathlib import Path


@pytest.fixture
def cli_config_file(tmp_path: Path) -> Path:
    """Write a minimal full-access config and return its path.

    full-access is used so ``call`` can reach every registered tool (the
    default read-only profile would filter writes out and change the
    parity surface the test asserts on).
    """
    config_file = tmp_path / "config.yml"
    config_file.write_text(
        "server:\n"
        "  name: TestCLI\n"
        "active_profile: full-access\n"
        "profiles_builtin_overrides:\n"
        "  full-access:\n"
        "    disabled: false\n"
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
    """Point the audit log at a temp dir so CLI calls in tests never write to
    the real state directory."""
    monkeypatch.setenv("XDG_STATE_HOME", str(tmp_path / "state"))


async def test_call_version_meta_tool_succeeds(cli_config_file: Path) -> None:
    """`call version` runs the meta tool and exits 0 with the version JSON."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["version"], stdout, stderr, config_path=cli_config_file
    )
    assert code == 0
    assert '"version"' in stdout.getvalue()


async def test_call_hello_coerces_and_succeeds(cli_config_file: Path) -> None:
    """`call hello --arg name=Ada` folds the arg and returns the greeting."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["hello", "--arg", "name=Ada"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 0
    assert "Ada" in stdout.getvalue()


async def test_call_matches_direct_dispatch(cli_config_file: Path) -> None:
    """`call` of a meta tool returns the same payload a direct dispatch does.

    This is the core contract: the CLI is a front-end over the same chokepoint,
    so its output must equal driving ``Server.dispatch`` directly.
    """
    # Reference: drive dispatch directly.
    from linodemcp.config import load_from_file

    cfg = load_from_file(cli_config_file)
    server = Server(cfg)
    direct = await server.dispatch("hello", {"name": "Ada"})
    direct_text = direct[0].text

    # CLI path.
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["hello", "--arg", "name=Ada"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 0
    assert stdout.getvalue().strip() == direct_text


async def test_call_unknown_tool_exits_usage_error(cli_config_file: Path) -> None:
    """An unknown tool name is a usage error (exit 2) with a helpful message."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["linode_not_a_real_tool"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 2
    assert "unknown tool" in stderr.getvalue()


async def test_call_bad_arg_coercion_exits_usage_error(cli_config_file: Path) -> None:
    """A value that does not fit its schema type is a usage error before dispatch.

    ``limit`` on the audit tool is an integer; ``--arg limit=lots`` cannot
    coerce, so the call exits 2 without dispatching.
    """
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["linode_audit_recent", "--arg", "limit=lots"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 2
    assert "limit" in stderr.getvalue()


async def test_call_json_and_arg_mutually_exclusive(cli_config_file: Path) -> None:
    """Passing both --json and --arg is a usage error."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["version", "--json", "{}", "--arg", "x=1"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 2
    assert "mutually exclusive" in stderr.getvalue()


async def test_call_bad_json_exits_usage_error(cli_config_file: Path) -> None:
    """A malformed --json blob is a usage error caught before dispatch."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["version", "--json", "{not valid}"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 2


async def test_call_tool_error_result_exits_one(cli_config_file: Path) -> None:
    """A tool that returns an error result exits 1 with the message on stderr.

    ``linode_instance_get`` with a non-numeric instance_id returns an
    ``Error:`` payload (handler-level validation), which the CLI maps to
    exit 1.
    """
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["linode_instance_get", "--arg", "instance_id=notanumber"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 1
    assert "Error" in stderr.getvalue()


async def test_call_table_output(cli_config_file: Path) -> None:
    """`call version --output table` renders the object as key/value rows."""
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["version", "--output", "table"],
        stdout,
        stderr,
        config_path=cli_config_file,
    )
    assert code == 0
    assert "version" in stdout.getvalue()
    # Table output is not JSON, so no leading brace.
    assert not stdout.getvalue().lstrip().startswith("{")


async def test_call_profile_filtered_tool_exits_usage_error(tmp_path: Path) -> None:
    """A tool the active profile filters out is refused (exit 2) via dispatch.

    Under the read-only default profile, a write tool is not in the allow list;
    the CLI validates the name against the full registry (so it is "known"),
    but dispatch refuses it, and the CLI maps that ValueError to exit 2.
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
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["linode_instance_delete", "--arg", "instance_id=1", "--dry-run"],
        stdout,
        stderr,
        config_path=config_file,
    )
    assert code == 2
    assert "Unknown tool" in stderr.getvalue()


async def test_call_malformed_config_exits_usage_error(tmp_path: Path) -> None:
    """A malformed config (not merely absent) fails construction with exit 2.

    A missing file falls back to the in-memory default, but a malformed one is
    a real error the user must see, so it still propagates and the CLI reports
    it as a usage error.
    """
    bad = tmp_path / "bad_config.yml"
    bad.write_text("server: [this is not a mapping\n")
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(["version"], stdout, stderr, config_path=bad)
    assert code == 2
    assert "failed to start runtime" in stderr.getvalue()


async def test_call_version_offline_no_config(tmp_path: Path) -> None:
    """With NO config file, `call version` (a meta tool) still runs, exit 0.

    The runtime falls back to the in-memory default so meta-tool calls work
    offline, matching the Go CLI. This locks the no-config fallback in.
    """
    missing = tmp_path / "absent_config.yml"
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(["version"], stdout, stderr, config_path=missing)
    assert code == 0
    assert '"version"' in stdout.getvalue()


async def test_call_hello_offline_no_config(tmp_path: Path) -> None:
    """With NO config file, `call hello --arg name=x` runs offline, exit 0."""
    missing = tmp_path / "absent_config.yml"
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["hello", "--arg", "name=x"], stdout, stderr, config_path=missing
    )
    assert code == 0
    assert "x" in stdout.getvalue()


async def test_call_api_tool_offline_no_config_exits_one(tmp_path: Path) -> None:
    """With NO config, a Linode-API tool fails at CALL time (exit 1), not load.

    The fallback default has no environments, so the tool reaches its handler
    and returns a clear no-environment error result, which the CLI maps to
    exit 1. The command does not die at construction with exit 2.
    """
    missing = tmp_path / "absent_config.yml"
    stdout, stderr = io.StringIO(), io.StringIO()
    code = await run_call_command(
        ["linode_instance_list"], stdout, stderr, config_path=missing
    )
    assert code == 1
    # A clear no-environment message, not a construction failure.
    combined = stdout.getvalue() + stderr.getvalue()
    assert "environment" in combined.lower()
    assert "failed to start runtime" not in stderr.getvalue()


def test_cli_call_surface_equals_registry() -> None:
    """CLI-to-registry parity: the callable surface equals the registry.

    Mirrors the tool-manifest gate. ``call`` keeps no allow list of its own, so
    the names it can invoke (``callable_tool_names``) must equal the registry.
    This locks the CLI surface to the tool surface so a future change cannot let
    them drift; a non-registry name is correctly excluded.
    """
    from linodemcp.cli.call import callable_tool_names

    registry_names = {entry.name for entry in get_tool_registry()}
    assert registry_names, "registry must not be empty"
    assert callable_tool_names() == registry_names
    assert "definitely_not_a_tool" not in callable_tool_names()
