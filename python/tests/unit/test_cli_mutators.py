"""Phase 7b mutator subcommand tests (use, enable, disable).

Mirrors ``go/internal/cli/mutate_test.go``. Each test owns a tempdir
config file so the round-trip exercises ``write_atomic`` end-to-end.
"""

from __future__ import annotations

import io
from typing import TYPE_CHECKING

import pytest

from linodemcp.cli import (
    EXIT_USAGE_ERROR,
    run_profile_disable,
    run_profile_enable,
    run_profile_use,
)
from linodemcp.config import load_from_file

if TYPE_CHECKING:
    from pathlib import Path


_MINIMAL_YAML = """\
server:
  name: "Test"
  logLevel: "info"
  transport: "stdio"
  host: "127.0.0.1"
  port: 8080
environments:
  default:
    label: "Default"
    linode:
      apiUrl: "https://api.linode.com/v4"
      token: "tok"
"""


@pytest.fixture
def config_path(tmp_path: Path) -> Path:
    """Stage a minimal config file and return its path."""
    path = tmp_path / "config.yml"
    path.write_text(_MINIMAL_YAML)
    return path


def test_run_profile_use_switches_active_profile(config_path: Path) -> None:
    """Happy path: switching to a known built-in persists in the file."""
    stdout = io.StringIO()
    stderr = io.StringIO()

    rc = run_profile_use(["readonly-full"], config_path, stdout, stderr)

    assert rc == 0, f"use must succeed: {stderr.getvalue()}"
    assert "active profile switched to readonly-full" in stdout.getvalue()

    reloaded = load_from_file(config_path)
    assert reloaded.active_profile == "readonly-full"


def test_run_profile_use_unknown_profile_exits_one(config_path: Path) -> None:
    """Unknown profile name exits 1 without writing the bad value."""
    stderr = io.StringIO()

    rc = run_profile_use(
        ["definitely-not-a-real-profile"],
        config_path,
        io.StringIO(),
        stderr,
    )

    assert rc == 1
    assert "not found" in stderr.getvalue()

    reloaded = load_from_file(config_path)
    assert reloaded.active_profile == "", (
        "failed use must not persist the bad name on disk"
    )


def test_run_profile_use_zero_args_returns_usage() -> None:
    """Arity check: use requires exactly one positional argument."""
    stderr = io.StringIO()

    rc = run_profile_use([], None, io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()


def test_run_profile_disable_sets_override(config_path: Path) -> None:
    """Disable sets the override flag and persists across a reload."""
    stdout = io.StringIO()
    stderr = io.StringIO()

    rc = run_profile_disable(["compute-admin"], config_path, stdout, stderr)

    assert rc == 0, f"disable must succeed: {stderr.getvalue()}"
    assert "profile compute-admin disabled" in stdout.getvalue()

    reloaded = load_from_file(config_path)
    assert reloaded.profiles_builtin_overrides["compute-admin"].disabled is True


def test_run_profile_enable_clears_override(config_path: Path) -> None:
    """Enable clears a previously-set disabled override."""
    rc = run_profile_disable(
        ["compute-admin"], config_path, io.StringIO(), io.StringIO()
    )
    assert rc == 0

    stdout = io.StringIO()
    stderr = io.StringIO()

    rc = run_profile_enable(["compute-admin"], config_path, stdout, stderr)

    assert rc == 0, f"enable must succeed: {stderr.getvalue()}"
    assert "profile compute-admin enabled" in stdout.getvalue()

    reloaded = load_from_file(config_path)
    assert reloaded.profiles_builtin_overrides["compute-admin"].disabled is False


def test_run_profile_disable_refuses_active_profile(config_path: Path) -> None:
    """Disabling the active profile would leave the server unable to
    start, so the subcommand must reject it and not write.
    """
    # Switch to compute-admin first.
    assert (
        run_profile_use(["compute-admin"], config_path, io.StringIO(), io.StringIO())
        == 0
    )

    stderr = io.StringIO()
    rc = run_profile_disable(["compute-admin"], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "active profile" in stderr.getvalue()

    reloaded = load_from_file(config_path)
    override = reloaded.profiles_builtin_overrides.get("compute-admin")
    assert override is None or override.disabled is False, (
        "refused disable must not flip the bit on disk"
    )


def test_run_profile_enable_refuses_user_defined(config_path: Path) -> None:
    """enable/disable only apply to built-ins; user-defined names exit 1."""
    stderr = io.StringIO()

    rc = run_profile_enable(["my-custom-profile"], config_path, io.StringIO(), stderr)

    assert rc == 1
    assert "not a built-in" in stderr.getvalue()


def test_run_profile_enable_zero_args_returns_usage() -> None:
    """enable requires exactly one positional argument."""
    stderr = io.StringIO()

    rc = run_profile_enable([], None, io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()


def test_run_profile_disable_zero_args_returns_usage() -> None:
    """disable requires exactly one positional argument."""
    stderr = io.StringIO()

    rc = run_profile_disable([], None, io.StringIO(), stderr)

    assert rc == EXIT_USAGE_ERROR
    assert "Usage:" in stderr.getvalue()
